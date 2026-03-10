package cliutil

import (
	"context"
	"fmt"
	"os"

	"github.com/benfiola/game-server-images/internal/info"
	"github.com/benfiola/game-server-images/internal/logging"
	"github.com/urfave/cli/v3"
)

func Setup(command *cli.Command) *cli.Command {
	flags := []cli.Flag{
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
	}
	command.Flags = append(command.Flags, flags...)

	before := command.Before
	command.Before = func(ctx context.Context, c *cli.Command) (context.Context, error) {
		logFormat := c.String("log-format")
		logLevel := c.String("log-level")

		logger, err := logging.New(&logging.Opts{
			Format: logFormat,
			Level:  logLevel,
		})
		if err != nil {
			return nil, err
		}

		sctx := logging.WithLogger(ctx, logger)
		if before != nil {
			return before(sctx, c)
		} else {
			return sctx, nil
		}
	}

	command.Version = info.Version

	return command
}

func Run(c *cli.Command) {
	cli.VersionPrinter = func(cmd *cli.Command) {
		fmt.Fprint(cmd.Root().Writer, cmd.Root().Version)
	}

	if err := c.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(255)
	} else {
		os.Exit(0)
	}
}
