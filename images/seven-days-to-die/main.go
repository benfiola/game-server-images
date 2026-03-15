package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/benfiola/game-server-images/internal/archive"
	"github.com/benfiola/game-server-images/internal/cache"
	"github.com/benfiola/game-server-images/internal/cliutil"
	"github.com/benfiola/game-server-images/internal/cmd"
	"github.com/benfiola/game-server-images/internal/datatransform"
	"github.com/benfiola/game-server-images/internal/http"
	"github.com/benfiola/game-server-images/internal/logging"
	"github.com/benfiola/game-server-images/internal/steam"
	"github.com/urfave/cli/v3"
)

const (
	AppId                    = 294420
	DepotId                  = 294422
	WebDashboardPort         = 8080
	TelnetPort               = 8081
	TelnetReadyPattern       = "Press 'help' to get a list of all commands. Press 'exit' to end session."
	TelnetReadTimeout        = 10 * time.Second
	TelnetConnectionAttempts = 10
	TelnetConnectionDelay    = 1 * time.Second
	TelnetConnectionBackoff  = 2.0
	TelmetMaxConnectionDelay = 30 * time.Second
)

type Opts struct {
	CachePath         string
	DataPath          string
	GamePath          string
	ManifestId        int
	DeleteDefaultMods bool
	Mods              []Mod
	AutoRestart       time.Duration
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
	if o.ManifestId <= 0 {
		return fmt.Errorf("manifest id must be positive")
	}
	return nil
}

type Mod struct {
	Url  string
	Root bool
}

type ServerSettings map[string]string

type XmlServerSettings struct {
	XMLName    xml.Name            `xml:"ServerSettings"`
	Properties []XmlServerProperty `xml:"property"`
}

type XmlServerProperty struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

func (ss ServerSettings) Xml() XmlServerSettings {
	xss := XmlServerSettings{Properties: make([]XmlServerProperty, 0, len(ss))}
	keys := make([]string, 0, len(ss))
	for name := range ss {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		xss.Properties = append(xss.Properties, XmlServerProperty{Name: name, Value: ss[name]})
	}
	return xss
}

func (xss *XmlServerSettings) Map() ServerSettings {
	data := make(ServerSettings)
	for _, p := range xss.Properties {
		data[p.Name] = p.Value
	}
	return data
}

type TelnetConn struct {
	netConn net.Conn
	ctx     context.Context
}

func (conn *TelnetConn) ReadUntilPattern(pattern string, timeout time.Duration) error {
	logger := logging.FromContext(conn.ctx)
	if err := conn.netConn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}

	var data strings.Builder
	buf := make([]byte, 4096)

	for {
		read, err := conn.netConn.Read(buf)
		if err != nil {
			if err == io.EOF {
				return fmt.Errorf("connection closed before finding pattern: %s", pattern)
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return fmt.Errorf("timed out reading until pattern: %s", pattern)
			}
			return fmt.Errorf("failed to read from telnet: %w", err)
		}

		data.Write(buf[:read])
		logger.Debug("read from telnet", "pattern", pattern, "data", strings.TrimSpace(data.String()))

		if strings.Contains(data.String(), pattern) {
			return nil
		}
	}
}

func CombineMods(modUrls []string, rootUrls []string) []Mod {
	mods := make([]Mod, 0, len(modUrls)+len(rootUrls))

	for _, url := range modUrls {
		mods = append(mods, Mod{Url: url, Root: false})
	}

	for _, url := range rootUrls {
		mods = append(mods, Mod{Url: url, Root: true})
	}

	return mods
}

func GetDefaultServerSettings(ctx context.Context, gameDir string) (ServerSettings, error) {
	logger := logging.FromContext(ctx)
	filePath := filepath.Join(gameDir, "serverconfig.xml")
	logger.Debug("reading default server settings", "path", filePath)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read server config: %w", err)
	}

	xss := &XmlServerSettings{}
	if err := xml.Unmarshal(data, xss); err != nil {
		return nil, fmt.Errorf("failed to parse server config: %w", err)
	}

	return xss.Map(), nil
}

func GetEnvServerSettings(ctx context.Context) ServerSettings {
	logger := logging.FromContext(ctx)
	data := make(ServerSettings)
	prefix := "SETTING_"

	for _, item := range os.Environ() {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 || !strings.HasPrefix(parts[0], prefix) {
			continue
		}
		key := strings.TrimPrefix(parts[0], prefix)
		data[key] = parts[1]
	}

	logger.Info("loaded environment server settings", "count", len(data))
	return data
}

func GetServerSettings(ctx context.Context, gamePath string, dataPath string) (ServerSettings, error) {
	defaultSettings, err := GetDefaultServerSettings(ctx, gamePath)
	if err != nil {
		return nil, err
	}

	envSettings := GetEnvServerSettings(ctx)

	return datatransform.ShallowMerge(
		defaultSettings,
		ServerSettings{
			"WebDashboardEnabled": "true",
		},
		envSettings,
		ServerSettings{
			"TelnetEnabled":    "true",
			"TelnetPort":       strconv.Itoa(TelnetPort),
			"UserDataFolder":   dataPath,
			"WebDashboardPort": strconv.Itoa(WebDashboardPort),
		},
	), nil
}

func WriteServerSettings(ctx context.Context, settings ServerSettings, path string) error {
	logger := logging.FromContext(ctx)
	logger.Debug("writing server settings", "path", path)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	xmlSettings := settings.Xml()
	data, err := xml.MarshalIndent(xmlSettings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal server settings: %w", err)
	}

	if err := os.WriteFile(path, append([]byte(xml.Header), data...), 0644); err != nil {
		return fmt.Errorf("failed to write server settings: %w", err)
	}

	return nil
}

func DownloadGame(ctx context.Context, c *cache.Cache, manifestId int, path string) error {
	logger := logging.FromContext(ctx)
	key := fmt.Sprintf("sdtd-%d", manifestId)

	if !c.Exists(ctx, key) {
		logger.Info("downloading game", "app", AppId, "depot", DepotId, "manifest", manifestId)
		if err := steam.Download(ctx, AppId, DepotId, manifestId, path); err != nil {
			return err
		}
		if err := c.Put(ctx, key, path); err != nil {
			return err
		}
	} else {
		logger.Info("using cached game", "app", AppId, "depot", DepotId, "manifest", manifestId)
		if err := c.Get(ctx, key, path); err != nil {
			return err
		}
	}

	serverBin := filepath.Join(path, "7DaysToDieServer.x86_64")
	if err := os.Chmod(serverBin, 0755); err != nil {
		logger.Error("failed to chmod server binary", "path", serverBin, "error", err)
	}

	return nil
}

func InstallMods(ctx context.Context, cache *cache.Cache, gamePath string, mods ...Mod) error {
	logger := logging.FromContext(ctx)

	for _, mod := range mods {
		var installPath string
		if mod.Root {
			installPath = gamePath
		} else {
			installPath = filepath.Join(gamePath, "Mods")
		}

		if err := os.MkdirAll(installPath, 0755); err != nil {
			return err
		}

		modName := filepath.Base(mod.Url)
		key := fmt.Sprintf("mod-%s", mod.Url)
		logger.Info("installing mod", "path", installPath, "mod", mod.Url, "root", mod.Root)

		if !cache.Exists(ctx, key) {
			tempDir := filepath.Join(installPath, ".tmp")
			if err := os.MkdirAll(tempDir, 0755); err != nil {
				return err
			}

			downloadPath := filepath.Join(tempDir, modName)
			if err := http.Download(ctx, mod.Url, downloadPath); err != nil {
				return fmt.Errorf("failed to download mod: %w", err)
			}

			if err := archive.Extract(ctx, downloadPath, installPath); err != nil {
				return fmt.Errorf("failed to extract mod: %w", err)
			}

			if err := cache.Put(ctx, key, installPath); err != nil {
				return err
			}

			os.RemoveAll(tempDir)
		} else {
			logger.Info("using cached mod", "mod", mod.Url)
			if err := cache.Get(ctx, key, installPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func DeleteDefaultMods(ctx context.Context, gameDir string) error {
	logger := logging.FromContext(ctx)
	logger.Info("deleting default mods")

	modsPath := filepath.Join(gameDir, "Mods")
	if err := os.RemoveAll(modsPath); err != nil {
		return fmt.Errorf("failed to remove mods directory: %w", err)
	}

	if err := os.MkdirAll(modsPath, 0755); err != nil {
		return fmt.Errorf("failed to recreate mods directory: %w", err)
	}

	return nil
}

type dialServerCb func(*TelnetConn) error

func DialServer(ctx context.Context, cb dialServerCb) error {
	logger := logging.FromContext(ctx)
	addr := fmt.Sprintf("localhost:%d", TelnetPort)
	logger.Debug("dialing server", "addr", addr)

	var nconn net.Conn
	var err error
	backoffDelay := TelnetConnectionDelay

	for attempt := range TelnetConnectionAttempts {
		nconn, err = net.Dial("tcp", addr)
		if err == nil {
			break
		}
		logger.Debug("connection attempt failed", "attempt", attempt+1, "delay", backoffDelay, "error", err)
		time.Sleep(backoffDelay)

		backoffDelay = min(
			time.Duration(float64(backoffDelay)*TelnetConnectionBackoff),
			TelmetMaxConnectionDelay,
		)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to server after %d attempts: %w", TelnetConnectionAttempts, err)
	}

	conn := &TelnetConn{ctx: ctx, netConn: nconn}
	defer conn.netConn.Close()

	if err := conn.ReadUntilPattern(TelnetReadyPattern, TelnetReadTimeout); err != nil {
		return err
	}

	return cb(conn)
}

func StartServer(ctx context.Context, gameDir string, configPath string) error {
	logger := logging.FromContext(ctx)
	logger.Info("starting server", "config", configPath)

	env := append(os.Environ(), "LD_LIBRARY_PATH=.")
	cmdArgs := []string{
		"./7DaysToDieServer.x86_64",
		"-batchmode",
		fmt.Sprintf("-configfile=%s", configPath),
		"-dedicated",
		"-logfile", "-",
		"-nographics",
		"-quit",
	}

	return cmd.StreamWithOpts(ctx, cmd.CmdOpts{Cwd: gameDir, Env: env}, cmdArgs...)
}

func ShutdownServer(ctx context.Context) error {
	logger := logging.FromContext(ctx)
	logger.Info("shutting down server")
	return DialServer(ctx, func(conn *TelnetConn) error {
		cmd := "shutdown\n"
		_, err := conn.netConn.Write([]byte(cmd))
		return err
	})
}

func SetupSignalHandler(ctx context.Context) {
	logger := logging.FromContext(ctx)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigChan
		logger.Info("received signal", "signal", sig)
		ShutdownServer(ctx)
	}()
}

func SetupAutoRestart(ctx context.Context, autoRestart time.Duration) {
	go func() {
		time.Sleep(autoRestart - time.Minute)
		message := "Restarting server in 1 minute"
		DialServer(ctx, func(conn *TelnetConn) error {
			cmd := fmt.Sprintf("say \"%s\"\n", message)
			_, err := conn.netConn.Write([]byte(cmd))
			return err
		})
		time.Sleep(time.Minute)
		ShutdownServer(ctx)
	}()
}

func Main(ctx context.Context, opts Opts) error {
	if err := opts.Validate(); err != nil {
		return err
	}

	c, err := cache.New(&cache.Opts{Path: opts.CachePath})
	if err != nil {
		return err
	}

	if err := DownloadGame(ctx, c, opts.ManifestId, opts.GamePath); err != nil {
		return err
	}

	if opts.DeleteDefaultMods {
		if err := DeleteDefaultMods(ctx, opts.GamePath); err != nil {
			return err
		}
	}

	if len(opts.Mods) > 0 {
		if err := InstallMods(ctx, c, opts.GamePath, opts.Mods...); err != nil {
			return err
		}
	}

	if err := c.Finalize(ctx); err != nil {
		return err
	}

	serverSettings, err := GetServerSettings(ctx, opts.GamePath, opts.DataPath)
	if err != nil {
		return err
	}

	configPath := filepath.Join(os.TempDir(), "serverconfig.xml")
	if err := WriteServerSettings(ctx, serverSettings, configPath); err != nil {
		return err
	}

	if opts.AutoRestart > 0 {
		SetupAutoRestart(ctx, opts.AutoRestart)
	}

	SetupSignalHandler(ctx)

	return StartServer(ctx, opts.GamePath, configPath)
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
					Name:    "data-path",
					Value:   "/data",
					Sources: cli.EnvVars("DATA_PATH"),
				},
				&cli.StringFlag{
					Name:    "game-path",
					Value:   "/sdtd",
					Sources: cli.EnvVars("GAME_PATH"),
				},
				&cli.IntFlag{
					Name:     "manifest-id",
					Required: true,
					Sources:  cli.EnvVars("MANIFEST_ID"),
				},
				&cli.BoolFlag{
					Name:    "delete-default-mods",
					Sources: cli.EnvVars("DELETE_DEFAULT_MODS"),
				},
				&cli.StringSliceFlag{
					Name:    "mod-urls",
					Sources: cli.EnvVars("MOD_URLS"),
				},
				&cli.StringSliceFlag{
					Name:    "root-urls",
					Sources: cli.EnvVars("ROOT_URLS"),
				},
				&cli.DurationFlag{
					Name:    "auto-restart",
					Sources: cli.EnvVars("AUTO_RESTART"),
				},
			},
			Action: func(ctx context.Context, c *cli.Command) error {
				mods := CombineMods(c.StringSlice("mod-urls"), c.StringSlice("root-urls"))

				return Main(ctx, Opts{
					CachePath:         c.String("cache-path"),
					DataPath:          c.String("data-path"),
					GamePath:          c.String("game-path"),
					ManifestId:        c.Int("manifest-id"),
					DeleteDefaultMods: c.Bool("delete-default-mods"),
					Mods:              mods,
					AutoRestart:       c.Duration("auto-restart"),
				})
			},
		}),
	)
}
