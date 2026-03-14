package steam

import (
	"context"
	"strconv"

	"github.com/benfiola/game-server-images/internal/cmd"
)

func Download(ctx context.Context, appId int, depotId int, manifestId int, output string) error {
	return cmd.Stream(ctx, "DepotDownloader", "-app", strconv.Itoa(appId), "-depot", strconv.Itoa(depotId), "-manifest", strconv.Itoa(manifestId), "-dir", output)
}
