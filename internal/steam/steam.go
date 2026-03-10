package steam

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"

	"github.com/benfiola/game-server-images/internal/cmd"
)

func Download(ctx context.Context, appId int, depotId int, manifestId int, output string) error {
	return cmd.Stream(ctx, "DepotDownloader", "-app", strconv.Itoa(appId), "-depot", strconv.Itoa(depotId), "-manifest", strconv.Itoa(manifestId), "-dir", output)
}

var regexpManifest = regexp.MustCompile(`(?m)^Manifest ([\d]+).*$`)

func GetLatestManifestId(ctx context.Context, appId int, depotId int) (int, error) {
	tempdir, err := os.MkdirTemp("", "depotdownloader-*")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(tempdir)

	output, err := cmd.Capture(ctx, "DepotDownloader", "-app", strconv.Itoa(appId), "-depot", strconv.Itoa(depotId), "-manifest-only", "-dir", tempdir)
	if err != nil {
		return 0, err
	}
	match := regexpManifest.FindStringSubmatch(output)
	if match == nil {
		return 0, fmt.Errorf("latest manifest for app %d and depot %d not found", appId, depotId)
	}
	manifestId, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, err
	}
	return manifestId, nil
}
