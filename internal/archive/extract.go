package archive

import (
	"context"

	"github.com/benfiola/game-server-images/internal/cmd"
)

func Extract(ctx context.Context, source string, dest string) error {
	return cmd.Stream(ctx, "bsdtar", "-x", "-f", source, "-C", dest)
}
