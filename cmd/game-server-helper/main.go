package main

import (
	"context"
	"fmt"
	"os"

	cachepkg "github.com/benfiola/game-server-images/internal/cache"
	"github.com/benfiola/game-server-images/internal/info"
	"github.com/benfiola/game-server-images/internal/jsonpatch"
	"github.com/benfiola/game-server-images/internal/logging"
	"github.com/urfave/cli/v3"
)

const cacheKey = "cache"

func main() {
	cli.VersionPrinter = func(cmd *cli.Command) {
		fmt.Fprint(cmd.Root().Writer, cmd.Root().Version)
	}

	command := &cli.Command{
		Version: info.Version,

		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "log-format",
				Sources: cli.EnvVars("LOG_FORMAT"),
				Value:   "text",
			},
			&cli.StringFlag{
				Name:    "log-level",
				Sources: cli.EnvVars("LOG_LEVEL"),
				Value:   "info",
			},
		},
		Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
			format := c.String("log-format")
			level := c.String("log-level")
			logger, err := logging.New(&logging.Opts{Format: format, Level: level})
			if err != nil {
				return ctx, err
			}

			sctx := logging.WithLogger(ctx, logger)
			return sctx, nil
		},

		Commands: []*cli.Command{
			{
				Name: "cache",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "path",
						Sources: cli.EnvVars("CACHE_PATH"),
						Value:   "/cache",
					},
				},
				Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
					path := c.String("path")

					cache, err := cachepkg.New(&cachepkg.Opts{
						Path: path,
					})
					if err != nil {
						return nil, err
					}

					sctx := context.WithValue(ctx, cacheKey, cache)
					return sctx, nil
				},
				Commands: []*cli.Command{
					{
						Name: "exists",
						Arguments: []cli.Argument{
							&cli.StringArg{
								Name: "key",
							},
						},
						Action: func(ctx context.Context, c *cli.Command) error {
							key := c.StringArg("key")

							cache := ctx.Value(cacheKey).(*cachepkg.Cache)
							code := 0
							if !cache.Exists(ctx, key) {
								code = 1
							}

							return cli.Exit("", code)
						},
					},
					{
						Name: "initialize",
						Action: func(ctx context.Context, c *cli.Command) error {
							cache := ctx.Value(cacheKey).(*cachepkg.Cache)
							return cache.Initialize(ctx)
						},
					},
					{
						Name: "get",
						Arguments: []cli.Argument{
							&cli.StringArg{
								Name: "key",
							},
							&cli.StringArg{
								Name: "output-path",
							},
						},
						Action: func(ctx context.Context, c *cli.Command) error {
							key := c.StringArg("key")
							outputPath := c.StringArg("output-path")

							cache := ctx.Value(cacheKey).(*cachepkg.Cache)
							if err := cache.Get(ctx, key, outputPath); err != nil {
								return err
							}

							return nil
						},
					},
					{
						Name: "put",
						Arguments: []cli.Argument{
							&cli.StringArg{
								Name: "key",
							},
							&cli.StringArg{
								Name: "input-path",
							},
						},
						Action: func(ctx context.Context, c *cli.Command) error {
							key := c.StringArg("key")
							inputPath := c.StringArg("input-path")

							cache := ctx.Value(cacheKey).(*cachepkg.Cache)
							return cache.Put(ctx, key, inputPath)
						},
					},
					{
						Name: "finalize",
						Action: func(ctx context.Context, c *cli.Command) error {
							cache := ctx.Value(cacheKey).(*cachepkg.Cache)
							return cache.Finalize(ctx)
						},
					},
				},
			},
			{
				Name: "json-patch",
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name: "patch-file",
					},
				},
				Action: func(ctx context.Context, c *cli.Command) error {
					patchFile := c.StringArg("patch-file")

					jsonPatcher, err := jsonpatch.New(&jsonpatch.Opts{})
					if err != nil {
						return err
					}
					return jsonPatcher.ApplyPatchesFromFile(ctx, patchFile)
				},
			},
		},
	}

	err := command.Run(context.Background(), os.Args)
	code := 0
	if err != nil {
		fmt.Printf("command failed, error: %v\n", err)
		code = -1
	}
	os.Exit(code)
}
