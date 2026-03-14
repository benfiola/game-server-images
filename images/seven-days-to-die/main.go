package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/benfiola/game-server-images/internal/archive"
	"github.com/benfiola/game-server-images/internal/cache"
	"github.com/benfiola/game-server-images/internal/cliutil"
	"github.com/benfiola/game-server-images/internal/cmd"
	"github.com/benfiola/game-server-images/internal/http"
	"github.com/benfiola/game-server-images/internal/logging"
	"github.com/benfiola/game-server-images/internal/steam"
	"github.com/urfave/cli/v3"
)

const (
	AppId              = 294420
	DepotId            = 294422
	TelnetAddr         = "localhost:8081"
	WebDashboardPort   = "8080"
	ReadyPattern       = "Press 'help' to get a list of all commands. Press 'exit' to end session."
	ReadTimeout        = 5 * time.Second
	ConnectionAttempts = 10
	ConnectionDelay    = 1 * time.Second
)

type Opts struct {
	CachePath          string
	DataPath           string
	GamePath           string
	ManifestId         int
	DeleteDefaultMods  bool
	ModUrls            []string
	RootUrls           []string
	AutoRestart        *time.Duration
	AutoRestartMessage string
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
	xss := XmlServerSettings{Properties: []XmlServerProperty{}}
	for name, value := range ss {
		xss.Properties = append(xss.Properties, XmlServerProperty{Name: name, Value: value})
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
	start := time.Now()
	data := ""
	buf := make([]byte, 128)

	for {
		now := time.Now()
		if now.Sub(start) >= timeout {
			return fmt.Errorf("timed out reading until pattern: %s", pattern)
		}

		remaining := timeout - now.Sub(start)
		if err := conn.netConn.SetReadDeadline(time.Now().Add(remaining)); err != nil {
			return err
		}

		read, err := conn.netConn.Read(buf)
		if err != nil {
			return err
		}

		data += string(buf[:read])
		logger.Debug("read from telnet", "pattern", pattern, "data", strings.TrimSpace(data))

		if strings.Contains(data, pattern) {
			return nil
		}
	}
}

type dialServerCb func(*TelnetConn) error

func DialServer(ctx context.Context, cb dialServerCb) error {
	logger := logging.FromContext(ctx)
	logger.Info("dialing server", "addr", TelnetAddr)

	var nconn net.Conn
	var err error

	for attempt := 0; attempt < ConnectionAttempts; attempt++ {
		nconn, err = net.Dial("tcp", TelnetAddr)
		if err == nil {
			break
		}
		logger.Debug("connection attempt failed", "attempt", attempt+1, "error", err)
		time.Sleep(ConnectionDelay)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to server after %d attempts: %w", ConnectionAttempts, err)
	}

	conn := &TelnetConn{ctx: ctx, netConn: nconn}
	defer conn.netConn.Close()

	if err := conn.ReadUntilPattern(ReadyPattern, ReadTimeout); err != nil {
		return err
	}

	return cb(conn)
}

func ShutdownServer(ctx context.Context) error {
	logger := logging.FromContext(ctx)
	logger.Info("shutting down server")
	return DialServer(ctx, func(conn *TelnetConn) error {
		_, err := conn.netConn.Write([]byte("shutdown\n"))
		return err
	})
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

func GetDefaultServerSettings(ctx context.Context, gameDir string) (ServerSettings, error) {
	logger := logging.FromContext(ctx)
	filePath := filepath.Join(gameDir, "serverconfig.xml")
	logger.Info("reading default server settings", "path", filePath)

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

func MergeServerSettings(items ...ServerSettings) ServerSettings {
	result := make(ServerSettings)
	for _, item := range items {
		for k, v := range item {
			result[k] = v
		}
	}
	return result
}

func WriteServerSettings(ctx context.Context, settings ServerSettings, path string) error {
	logger := logging.FromContext(ctx)
	logger.Info("writing server settings", "path", path)

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

func InstallMods(ctx context.Context, path string, mods ...string) error {
	logger := logging.FromContext(ctx)

	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}

	for _, modURL := range mods {
		modName := filepath.Base(modURL)
		logger.Info("installing mod", "path", path, "mod", modURL)

		tempDir := filepath.Join(path, ".tmp")
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			return err
		}
		defer os.RemoveAll(tempDir)

		downloadPath := filepath.Join(tempDir, modName)
		if err := http.Download(ctx, modURL, downloadPath); err != nil {
			return fmt.Errorf("failed to download mod: %w", err)
		}

		if err := archive.Extract(ctx, downloadPath, path); err != nil {
			return fmt.Errorf("failed to extract mod: %w", err)
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
		logger.Warn("failed to chmod server binary", "path", serverBin, "error", err)
	}

	return nil
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

func HealthCheck(ctx context.Context) error {
	logger := logging.FromContext(ctx)
	err := DialServer(ctx, func(conn *TelnetConn) error {
		return nil
	})
	if err != nil {
		logger.Error("health check failed", "error", err)
	} else {
		logger.Info("health check passed")
	}
	return err
}

func Main(ctx context.Context, opts Opts) error {
	c, err := cache.New(&cache.Opts{Path: opts.CachePath})
	if err != nil {
		return err
	}

	if err := DownloadGame(ctx, c, opts.ManifestId, opts.GamePath); err != nil {
		return err
	}

	if err := c.Finalize(ctx); err != nil {
		return err
	}

	if opts.DeleteDefaultMods {
		if err := DeleteDefaultMods(ctx, opts.GamePath); err != nil {
			return err
		}
	}

	if len(opts.RootUrls) > 0 {
		if err := InstallMods(ctx, opts.GamePath, opts.RootUrls...); err != nil {
			return err
		}
	}

	modsDir := filepath.Join(opts.GamePath, "Mods")
	if len(opts.ModUrls) > 0 {
		if err := InstallMods(ctx, modsDir, opts.ModUrls...); err != nil {
			return err
		}
	}

	defaultSettings, err := GetDefaultServerSettings(ctx, opts.GamePath)
	if err != nil {
		return err
	}

	envSettings := GetEnvServerSettings(ctx)

	mergedSettings := MergeServerSettings(
		defaultSettings,
		ServerSettings{
			"WebDashboardEnabled": "true",
		},
		envSettings,
		ServerSettings{
			"TelnetEnabled":    "true",
			"TelnetPort":       "8081",
			"UserDataFolder":   opts.DataPath,
			"WebDashboardPort": WebDashboardPort,
		},
	)

	configPath := filepath.Join(os.TempDir(), "serverconfig.xml")
	if err := WriteServerSettings(ctx, mergedSettings, configPath); err != nil {
		return err
	}

	if opts.AutoRestart != nil {
		go func() {
			time.Sleep(*opts.AutoRestart - time.Minute)
			message := opts.AutoRestartMessage
			if message == "" {
				message = "Restarting server in 1 minute"
			}
			DialServer(ctx, func(conn *TelnetConn) error {
				_, err := conn.netConn.Write([]byte(fmt.Sprintf("say \"%s\"\n", message)))
				return err
			})
			time.Sleep(time.Minute)
			ShutdownServer(ctx)
		}()
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
				&cli.StringFlag{
					Name:    "auto-restart-message",
					Value:   "Restarting server in 1 minute",
					Sources: cli.EnvVars("AUTO_RESTART_MESSAGE"),
				},
			},
			Action: func(ctx context.Context, c *cli.Command) error {
				autoRestart := c.Duration("auto-restart")
				var autoRestartPtr *time.Duration
				if autoRestart > 0 {
					autoRestartPtr = &autoRestart
				}

				return Main(ctx, Opts{
					CachePath:          c.String("cache-path"),
					DataPath:           c.String("data-path"),
					GamePath:           c.String("game-path"),
					ManifestId:         c.Int("manifest-id"),
					DeleteDefaultMods:  c.Bool("delete-default-mods"),
					ModUrls:            c.StringSlice("mod-urls"),
					RootUrls:           c.StringSlice("root-urls"),
					AutoRestart:        autoRestartPtr,
					AutoRestartMessage: c.String("auto-restart-message"),
				})
			},
		}),
	)
}
