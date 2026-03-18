package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"syscall"
	"time"

	"github.com/benfiola/game-server-images/internal/archive"
	"github.com/benfiola/game-server-images/internal/cache"
	"github.com/benfiola/game-server-images/internal/cliutil"
	"github.com/benfiola/game-server-images/internal/cmd"
	"github.com/benfiola/game-server-images/internal/healthcheck"
	httputil "github.com/benfiola/game-server-images/internal/http"
	"github.com/benfiola/game-server-images/internal/jsonpatch"
	"github.com/benfiola/game-server-images/internal/logging"
	"github.com/urfave/cli/v3"
)

const (
	ServerPort = 6969
)

type Opts struct {
	CachePath     string
	DataPath      string
	GamePath      string
	Version       string
	ModUrls       []string
	ConfigPatches map[string][]jsonpatch.Patch
}

func (o *Opts) Validate() error {
	if o.CachePath == "" {
		return fmt.Errorf("cache path is required")
	}
	if o.DataPath == "" {
		return fmt.Errorf("data path is required")
	}
	if o.GamePath == "" {
		return fmt.Errorf("game path is required")
	}
	return nil
}

var semverRegex = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

func GetMajorVersion(version string) (major int, err error) {
	_, err = fmt.Sscanf(version, "%d", &major)
	return
}

func GetLatestVersion(ctx context.Context) (string, error) {
	logger := logging.FromContext(ctx)
	logger.Debug("fetching releases from GitHub")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/benfiola/game-server-images-single-player-tarkov/releases?per_page=100")
	if err != nil {
		return "", fmt.Errorf("failed to fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	type Release struct {
		TagName string `json:"tag_name"`
	}

	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return "", fmt.Errorf("failed to parse releases: %w", err)
	}

	for _, release := range releases {
		if semverRegex.MatchString(release.TagName) {
			return release.TagName, nil
		}
	}

	return "", fmt.Errorf("failed to find latest version")
}

func DownloadGame(ctx context.Context, c *cache.Cache, version string, gamePath string) error {
	logger := logging.FromContext(ctx)

	cacheKey := fmt.Sprintf("spt-%s", version)

	if !c.Exists(ctx, cacheKey) {
		arch := runtime.GOARCH
		switch arch {
		case "amd64":
			arch = "amd64"
		case "arm64":
			arch = "arm64"
		default:
			return fmt.Errorf("unsupported architecture: %s", arch)
		}
		downloadURL := fmt.Sprintf("https://github.com/benfiola/game-server-images-single-player-tarkov/releases/download/%s/spt-%s-%s.tar.gz", version, version, arch)

		logger.Info("downloading SPT release", "version", version, "url", downloadURL)

		tempFile := filepath.Join(gamePath, ".temp-spt.tar.gz")
		if err := os.MkdirAll(filepath.Dir(tempFile), 0755); err != nil {
			return err
		}

		if err := httputil.Download(ctx, downloadURL, tempFile); err != nil {
			os.Remove(tempFile)
			return fmt.Errorf("failed to download SPT: %w", err)
		}

		logger.Debug("extracting SPT archive")
		if err := archive.Extract(ctx, tempFile, gamePath); err != nil {
			os.Remove(tempFile)
			return fmt.Errorf("failed to extract SPT: %w", err)
		}

		logger.Info("caching downloaded SPT", "key", cacheKey)
		if err := c.Put(ctx, cacheKey, gamePath); err != nil {
			return err
		}

		os.Remove(tempFile)
	} else {
		logger.Info("using cached SPT download", "version", version)
		if err := c.Get(ctx, cacheKey, gamePath); err != nil {
			return err
		}
	}

	return nil
}

func InstallMods(ctx context.Context, modUrls []string, gamePath string, c *cache.Cache) error {
	logger := logging.FromContext(ctx)

	if len(modUrls) == 0 {
		logger.Debug("no mods to install")
		return nil
	}

	modsDir := filepath.Join(gamePath, "user", "mods")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		return err
	}

	for _, modUrl := range modUrls {
		logger.Info("installing mod", "url", modUrl)

		if c.Exists(ctx, modUrl) {
			logger.Debug("mod already cached", "url", modUrl)
			if err := c.Get(ctx, modUrl, modsDir); err != nil {
				return err
			}
			continue
		}

		tempFile := filepath.Join(modsDir, ".temp-mod.zip")
		logger.Debug("downloading mod", "url", modUrl)
		if err := httputil.Download(ctx, modUrl, tempFile); err != nil {
			return fmt.Errorf("failed to download mod: %w", err)
		}

		logger.Debug("extracting mod archive")
		if err := archive.Extract(ctx, tempFile, modsDir); err != nil {
			os.Remove(tempFile)
			return fmt.Errorf("failed to extract mod: %w", err)
		}

		logger.Debug("caching mod", "url", modUrl)
		if err := c.Put(ctx, modUrl, modsDir); err != nil {
			logger.Error("failed to cache mod (continuing anyway)", "url", modUrl, "error", err)
		}

		os.Remove(tempFile)
	}

	return nil
}

func ApplyConfigPatches(ctx context.Context, gamePath string, patches map[string][]jsonpatch.Patch) error {
	logger := logging.FromContext(ctx)

	if len(patches) == 0 {
		logger.Debug("no config patches to apply")
		return nil
	}

	logger.Info("applying config patches", "count", len(patches))

	for filePath, filePatch := range patches {
		fullPath := filepath.Join(gamePath, filePath)
		logger.Debug("applying patch to config", "path", filePath)

		data, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("failed to read config %s: %w", filePath, err)
		}

		var original map[string]interface{}
		if err := json.Unmarshal(data, &original); err != nil {
			return fmt.Errorf("failed to parse config %s: %w", filePath, err)
		}

		var patched map[string]interface{}
		if err := jsonpatch.ApplyPatches(original, filePatch, &patched); err != nil {
			return fmt.Errorf("failed to apply patches to %s: %w", filePath, err)
		}

		patchedData, err := json.MarshalIndent(patched, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal config %s: %w", filePath, err)
		}

		if err := os.WriteFile(fullPath, patchedData, 0644); err != nil {
			return fmt.Errorf("failed to write config %s: %w", filePath, err)
		}

		logger.Debug("config patch applied", "path", filePath)
	}

	return nil
}

func WaitForServerReady(ctx context.Context) error {
	serverUrl := fmt.Sprintf("https://localhost:%d", ServerPort)
	logger := logging.FromContext(ctx)
	logger.Info("waiting for server to be ready", "url", serverUrl)

	maxRetries := 30
	delay := 1 * time.Second
	maxDelay := 30 * time.Second

	for attempt := range maxRetries {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(serverUrl)

		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			logger.Info("server is ready")
			return nil
		}

		if resp != nil {
			resp.Body.Close()
		}

		if attempt < maxRetries-1 {
			logger.Debug("server is not ready, retrying", "attempt", attempt+1, "delay", delay)
			select {
			case <-time.After(delay):
				delay = time.Duration(float64(delay) * 1.5)
				if delay > maxDelay {
					delay = maxDelay
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return fmt.Errorf("server did not become ready after %d attempts", maxRetries)
}

func InitializeServer(ctx context.Context, gamePath string, version string) error {
	logger := logging.FromContext(ctx)
	logger.Info("initializing server for first-run config generation")

	serverCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- RunServer(serverCtx, gamePath, version)
	}()

	if err := WaitForServerReady(ctx); err != nil {
		cancel()
		<-serverErr
		return fmt.Errorf("server failed to become ready: %w", err)
	}

	logger.Info("server is ready, shutting down initialization instance")

	cancel()
	if err := <-serverErr; err != nil {
		logger.Debug("server shutdown result", "error", err)
	}

	return nil
}

func CreateSymlinks(ctx context.Context, gamePath string, dataPath string) error {
	logger := logging.FromContext(ctx)
	logger.Info("creating data persistence symlinks")

	if err := os.MkdirAll(filepath.Join(dataPath, "user"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dataPath, "logs"), 0755); err != nil {
		return err
	}

	userDir := filepath.Join(gamePath, "user")
	if err := os.RemoveAll(userDir); err != nil && !os.IsNotExist(err) {
		return err
	}

	logsDir := filepath.Join(gamePath, "Logs")
	if err := os.RemoveAll(logsDir); err != nil && !os.IsNotExist(err) {
		return err
	}

	logger.Debug("creating symlink", "src", filepath.Join(dataPath, "user"), "dst", userDir)
	if err := os.Symlink(filepath.Join(dataPath, "user"), userDir); err != nil {
		return fmt.Errorf("failed to create user symlink: %w", err)
	}

	logger.Debug("creating symlink", "src", filepath.Join(dataPath, "logs"), "dst", logsDir)
	if err := os.Symlink(filepath.Join(dataPath, "logs"), logsDir); err != nil {
		return fmt.Errorf("failed to create logs symlink: %w", err)
	}

	return nil
}

func SetupSignalHandler(ctx context.Context) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
}

func MergeConfigPatches(patchMaps ...map[string][]jsonpatch.Patch) map[string][]jsonpatch.Patch {
	result := make(map[string][]jsonpatch.Patch)
	for _, patchMap := range patchMaps {
		for key, patches := range patchMap {
			result[key] = append(result[key], patches...)
		}
	}
	return result
}

func GetConfigPatches(userPatches map[string][]jsonpatch.Patch) map[string][]jsonpatch.Patch {
	patchOverrides := map[string][]jsonpatch.Patch{
		"SPT_Data/Server/configs/http.json": {
			{Op: "replace", Path: "/ip", Value: "0.0.0.0"},
			{Op: "replace", Path: "/backendIp", Value: "0.0.0.0"},
		},
	}
	return MergeConfigPatches(userPatches, patchOverrides)
}

func GetServerExecutable(version string) (string, error) {
	major, err := GetMajorVersion(version)
	if err != nil {
		return "", err
	}

	if major < 4 {
		return "./SPT.Server.exe", nil
	}
	return "./SPT.Server.Linux", nil
}

func RunServer(ctx context.Context, gamePath string, version string) error {
	logger := logging.FromContext(ctx)

	executable, err := GetServerExecutable(version)
	if err != nil {
		return err
	}

	logger.Info("starting server", "executable", executable)
	return cmd.StreamWithOpts(ctx, cmd.CmdOpts{Cwd: gamePath}, executable)
}

func HealthCheck(ctx context.Context) error {
	return nil
}

func Main(ctx context.Context, opts Opts) error {
	if err := opts.Validate(); err != nil {
		return err
	}

	logger := logging.FromContext(ctx)

	version := opts.Version
	if version == "" {
		logger.Info("determining latest version")
		latestVersion, err := GetLatestVersion(ctx)
		if err != nil {
			return err
		}
		version = latestVersion
	}

	c, err := cache.New(&cache.Opts{Path: opts.CachePath})
	if err != nil {
		return err
	}

	if err := DownloadGame(ctx, c, version, opts.GamePath); err != nil {
		return err
	}

	if err := InstallMods(ctx, opts.ModUrls, opts.GamePath, c); err != nil {
		return err
	}

	if err := c.Finalize(ctx); err != nil {
		return err
	}

	if err := InitializeServer(ctx, opts.GamePath, version); err != nil {
		return err
	}

	finalPatches := GetConfigPatches(opts.ConfigPatches)
	if err := ApplyConfigPatches(ctx, opts.GamePath, finalPatches); err != nil {
		return err
	}

	if err := CreateSymlinks(ctx, opts.GamePath, opts.DataPath); err != nil {
		return err
	}

	SetupSignalHandler(ctx)

	if err := healthcheck.SetupHealthCheck(ctx, ":8880", HealthCheck); err != nil {
		return err
	}

	return RunServer(ctx, opts.GamePath, version)
}

func main() {
	cliutil.Run(
		cliutil.Setup(&cli.Command{
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "cache-path",
					Value:   "/cache",
					Sources: cli.EnvVars("CACHE_PATH"),
				},
				&cli.StringFlag{
					Name:    "config-patches",
					Sources: cli.EnvVars("CONFIG_PATCHES"),
				},
				&cli.StringFlag{
					Name:    "data-path",
					Value:   "/data",
					Sources: cli.EnvVars("DATA_PATH"),
				},
				&cli.StringFlag{
					Name:    "game-path",
					Value:   "/game",
					Sources: cli.EnvVars("GAME_PATH"),
				},
				&cli.StringSliceFlag{
					Name:    "mod-urls",
					Sources: cli.EnvVars("MOD_URLS"),
				},
				&cli.StringFlag{
					Name:    "version",
					Sources: cli.EnvVars("VERSION"),
				},
			},
			Action: func(ctx context.Context, c *cli.Command) error {
				configPatches := make(map[string][]jsonpatch.Patch)
				if patchesJson := c.String("config-patches"); patchesJson != "" {
					if err := json.Unmarshal([]byte(patchesJson), &configPatches); err != nil {
						return fmt.Errorf("failed to parse config patches: %w", err)
					}
				}

				return Main(ctx, Opts{
					CachePath:     c.String("cache-path"),
					ConfigPatches: configPatches,
					DataPath:      c.String("data-path"),
					GamePath:      c.String("game-path"),
					ModUrls:       c.StringSlice("mod-urls"),
					Version:       c.String("version"),
				})
			},
		}),
	)
}
