package main

import (
	"context"

	"github.com/benfiola/game-server-images/internal/cliutil"
	"github.com/urfave/cli/v3"
)

func Main() error {
	return nil
}

func main() {
	cliutil.Run(
		cliutil.Setup(&cli.Command{
			Action: func(ctx context.Context, c *cli.Command) error {
				return Main()
			},
		}),
	)
}
