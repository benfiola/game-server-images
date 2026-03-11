package main

import (
	"context"
	"fmt"

	"github.com/benfiola/game-server-images/internal/cliutil"
	"github.com/urfave/cli/v3"
)

func main() {
	cliutil.Run(
		cliutil.Setup(&cli.Command{
			Action: func(ctx context.Context, c *cli.Command) error {
				fmt.Printf("here")
				return nil
			},
		}),
	)
}
