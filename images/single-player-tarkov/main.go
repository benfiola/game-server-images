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
	"syscall"
	"time"

	"github.com/benfiola/game-server-images/internal/archive"
	"github.com/benfiola/game-server-images/internal/cache"
	"github.com/benfiola/game-server-images/internal/cliutil"
	"github.com/benfiola/game-server-images/internal/cmd"
	"github.com/benfiola/game-server-images/internal/datatransform"
	httputil "github.com/benfiola/game-server-images/internal/http"
	"github.com/benfiola/game-server-images/internal/logging"
	"github.com/urfave/cli/v3"
)

const (
	ServerPort     = 6969
	HealthCheckURL = "http://localhost:6969/client/version/validate"
)

type Opts struct {
	CachePath     string
	DataPath      string
	GamePath      string
	Version       string
	ModUrls       []string
	ConfigPatches map[string][]datatransform.Patch
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

func GetLatestVersion(ctx context.Context) (string, error) {
	logger := logging.FromContext(ctx)
	logger.Debug("fetching tags from GitHub")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/sp-tarkov/server/tags?per_page=100")
	if err != nil {
		return "", fmt.Errorf("failed to fetch tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	type Tag struct {
		Name string `json:"name"`
	}

	var tags []Tag
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return "", fmt.Errorf("failed to parse tags: %w", err)
	}

	for _, tag := range tags {
		if semverRegex.MatchString(tag.Name) {
			return tag.Name, nil
		}
	}

	return "", fmt.Errorf("failed to find latest version")
}

func BuildOrFetchGame(ctx context.Context, c *cache.Cache, version string, gamePath string) error {
	logger := logging.FromContext(ctx)

	if version == "" {
		logger.Info("determining latest version")
		latestVersion, err := GetLatestVersion(ctx)
		if err != nil {
			return err
		}
		version = latestVersion
	}

	cacheKey := fmt.Sprintf("spt-%s", version)

	if !c.Exists(ctx, cacheKey) {
		logger.Info("building SPT from source", "version", version)

		tempDir, err := os.MkdirTemp("", "spt-source-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tempDir)

		logger.Debug("cloning repository", "url", "https://github.com/sp-tarkov/server")
		if err := cmd.Stream(ctx, "git", "clone", "--depth", "1", "--branch", version, "https://github.com/sp-tarkov/server", tempDir); err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}

		logger.Debug("setting up git LFS")
		if err := cmd.StreamWithOpts(ctx, cmd.CmdOpts{Cwd: tempDir}, "git", "lfs", "install"); err != nil {
			return fmt.Errorf("failed to install git LFS: %w", err)
		}

		if err := cmd.StreamWithOpts(ctx, cmd.CmdOpts{Cwd: tempDir}, "git", "lfs", "pull"); err != nil {
			return fmt.Errorf("failed to pull git LFS assets: %w", err)
		}

		logger.Debug("installing npm dependencies")
		if err := cmd.StreamWithOpts(ctx, cmd.CmdOpts{Cwd: tempDir}, "npm", "install"); err != nil {
			return fmt.Errorf("failed to install dependencies: %w", err)
		}

		logger.Debug("building SPT")
		if err := cmd.StreamWithOpts(ctx, cmd.CmdOpts{Cwd: tempDir}, "npm", "run", "build:release"); err != nil {
			return fmt.Errorf("failed to build SPT: %w", err)
		}

		buildPath := filepath.Join(tempDir, "build")
		if err := os.Rename(buildPath, gamePath); err != nil {
			return err
		}

		logger.Info("caching built SPT", "key", cacheKey)
		if err := c.Put(ctx, cacheKey, gamePath); err != nil {
			return fmt.Errorf("failed to cache build: %w", err)
		}
	} else {
		logger.Info("using cached SPT build", "version", version)
		if err := c.Get(ctx, cacheKey, gamePath); err != nil {
			return fmt.Errorf("failed to extract from cache: %w", err)
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
		return fmt.Errorf("failed to create mods directory: %w", err)
	}

	for _, modUrl := range modUrls {
		logger.Info("installing mod", "url", modUrl)

		if c.Exists(ctx, modUrl) {
			logger.Debug("mod already cached", "url", modUrl)
			if err := c.Get(ctx, modUrl, modsDir); err != nil {
				return fmt.Errorf("failed to extract mod from cache: %w", err)
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

func ApplyConfigPatches(ctx context.Context, gamePath string, patches map[string][]datatransform.Patch) error {
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
			return fmt.Errorf("failed to read config file %s: %w", filePath, err)
		}

		var original map[string]interface{}
		if err := json.Unmarshal(data, &original); err != nil {
			return fmt.Errorf("failed to parse config file %s: %w", filePath, err)
		}

		var patched map[string]interface{}
		if err := datatransform.ApplyPatches(original, filePatch, &patched); err != nil {
			return fmt.Errorf("failed to apply patches to %s: %w", filePath, err)
		}

		patchedData, err := json.MarshalIndent(patched, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal patched config %s: %w", filePath, err)
		}

		if err := os.WriteFile(fullPath, patchedData, 0644); err != nil {
			return fmt.Errorf("failed to write patched config %s: %w", filePath, err)
		}

		logger.Debug("config patch applied", "path", filePath)
	}

	return nil
}

func WaitForServerReady(ctx context.Context) error {
	logger := logging.FromContext(ctx)
	logger.Info("waiting for server to be ready", "url", HealthCheckURL)

	maxRetries := 30
	delay := 1 * time.Second
	maxDelay := 30 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(HealthCheckURL)

		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			logger.Info("server is ready")
			return nil
		}

		if resp != nil {
			resp.Body.Close()
		}

		if attempt < maxRetries-1 {
			logger.Debug("health check failed, retrying", "attempt", attempt+1, "delay", delay)
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

func InitializeServer(ctx context.Context, gamePath string) error {
	logger := logging.FromContext(ctx)
	logger.Info("initializing server for first-run config generation")

	// TODO: Start server in background, wait for readiness, then gracefully shutdown
	// This requires proper signal handling to send shutdown to the server process
	// For now, we'll skip this phase as it requires more complex process management
	logger.Warn("server initialization skipped - requires proper process management")

	return nil
}

func CreateSymlinks(ctx context.Context, gamePath string, dataPath string) error {
	logger := logging.FromContext(ctx)
	logger.Info("creating data persistence symlinks")

	if err := os.MkdirAll(filepath.Join(dataPath, "user"), 0755); err != nil {
		return fmt.Errorf("failed to create data/user directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dataPath, "logs"), 0755); err != nil {
		return fmt.Errorf("failed to create data/logs directory: %w", err)
	}

	userDir := filepath.Join(gamePath, "user")
	if err := os.RemoveAll(userDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove existing user directory: %w", err)
	}

	logsDir := filepath.Join(gamePath, "Logs")
	if err := os.RemoveAll(logsDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove existing Logs directory: %w", err)
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

func RunServer(ctx context.Context, gamePath string) error {
	return cmd.StreamWithOpts(ctx, cmd.CmdOpts{Cwd: gamePath}, "./server")
}

func Main(ctx context.Context, opts Opts) error {
	if err := opts.Validate(); err != nil {
		return err
	}

	cache, err := cache.New(&cache.Opts{Path: opts.CachePath})
	if err != nil {
		return err
	}

	if err := BuildOrFetchGame(ctx, cache, opts.Version, opts.GamePath); err != nil {
		return err
	}

	if err := InstallMods(ctx, opts.ModUrls, opts.GamePath, cache); err != nil {
		return err
	}

	if err := cache.Finalize(ctx); err != nil {
		return err
	}

	if err := InitializeServer(ctx, opts.GamePath); err != nil {
		return err
	}

	if err := ApplyConfigPatches(ctx, opts.GamePath, opts.ConfigPatches); err != nil {
		return err
	}

	if err := CreateSymlinks(ctx, opts.GamePath, opts.DataPath); err != nil {
		return err
	}

	SetupSignalHandler(ctx)

	return RunServer(ctx, opts.GamePath)
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
					Sources: cli.EnvVars("SPT_VERSION"),
				},
			},
			Action: func(ctx context.Context, c *cli.Command) error {
				configPatches := make(map[string][]datatransform.Patch)
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
