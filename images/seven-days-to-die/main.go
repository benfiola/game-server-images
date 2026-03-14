package main

import (
	"context"
	"fmt"

	"github.com/benfiola/game-server-images/internal/cache"
	"github.com/benfiola/game-server-images/internal/cliutil"
	"github.com/benfiola/game-server-images/internal/steam"
	"github.com/urfave/cli/v3"
)

const (
	AppId   = 294420
	DepotId = 294422
)

type Opts struct {
	CachePath  string
	DataPath   string
	GamePath   string
	ManifestId int
}

func DownloadGame(ctx context.Context, cache *cache.Cache, manifestId int, path string) error {
	key := fmt.Sprintf("sdtd-%d", manifestId)
	if !cache.Exists(ctx, key) {
		if err := steam.Download(ctx, AppId, DepotId, manifestId, path); err != nil {
			return err
		}
		if err := cache.Put(ctx, key, path); err != nil {
			return err
		}
	} else {
		if err := cache.Get(ctx, key, path); err != nil {
			return err
		}
	}
	return nil
}

func Main(ctx context.Context, opts Opts) error {
	cache, err := cache.New(&cache.Opts{
		Path: opts.CachePath,
	})
	if err != nil {
		return err
	}

	if err := DownloadGame(ctx, cache, opts.ManifestId, opts.GamePath); err != nil {
		return err
	}

	if err := cache.Finalize(ctx); err != nil {
		return err
	}

	return nil
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
			},
			Action: func(ctx context.Context, c *cli.Command) error {
				return Main(ctx, Opts{
					CachePath:  c.String("cache-path"),
					DataPath:   c.String("data-path"),
					GamePath:   c.String("game-path"),
					ManifestId: c.Int("manifest-id"),
				})
			},
		}),
	)
}
