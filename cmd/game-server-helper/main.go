package main

import (
	"context"
	"fmt"
	"os"

	"github.com/benfiola/game-server-images/internal/info"
	"github.com/benfiola/game-server-images/internal/logging"
	"github.com/urfave/cli/v3"
)

func main() {
	cli.VersionPrinter = func(cmd *cli.Command) {
		fmt.Fprint(cmd.Root().Writer, cmd.Root().Version)
	}

	command := &cli.Command{
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
		Commands: []*cli.Command{},
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
		Version: info.Version,
	}

	err := command.Run(context.Background(), os.Args)
	code := 0
	if err != nil {
		fmt.Printf("command failed, error: %v", err)
		code = 1
	}
	os.Exit(code)
}
