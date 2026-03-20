package cache

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/benfiola/game-server-images/internal/cmd"
	"github.com/benfiola/game-server-images/internal/logging"
)

type Opts struct {
	Path string
}

type Cache struct {
	Path         string
	AccessedKeys map[string]bool
}

func New(opts *Opts) (*Cache, error) {
	if err := os.MkdirAll(opts.Path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &Cache{
		AccessedKeys: map[string]bool{},
		Path:         opts.Path,
	}, nil
}

func (c *Cache) normalizeKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", hash)
}

func (c *Cache) getCachePath(key string) string {
	return filepath.Join(c.Path, fmt.Sprintf("%s.squashfs", key))
}

func (c *Cache) getKey(cachePath string) string {
	return strings.TrimSuffix(filepath.Base(cachePath), ".squashfs")
}

func (c *Cache) Exists(ctx context.Context, key string) bool {
	_, err := os.Stat(c.getCachePath(c.normalizeKey(key)))
	return err == nil
}

func (c *Cache) Get(ctx context.Context, key string, outputPath string) error {
	key = c.normalizeKey(key)
	cachePath := c.getCachePath(key)

	if _, err := os.Stat(cachePath); err != nil {
		return fmt.Errorf("cache entry not found: %w", err)
	}

	if _, err := cmd.Capture(ctx, "unsquashfs", "-f", "-d", outputPath, cachePath); err != nil {
		return fmt.Errorf("failed to extract cache entry: %w", err)
	}

	c.AccessedKeys[key] = true

	return nil
}

func (c *Cache) Put(ctx context.Context, key string, inputPath string) error {
	key = c.normalizeKey(key)
	cachePath := c.getCachePath(key)

	if _, err := cmd.Capture(ctx, "mksquashfs", inputPath, cachePath); err != nil {
		return fmt.Errorf("failed to create cache entry: %w", err)
	}

	c.AccessedKeys[key] = true

	return nil
}

func (c *Cache) Finalize(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	paths, err := filepath.Glob(fmt.Sprintf("%s/*.squashfs", c.Path))
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, path := range paths {
		key := c.getKey(path)
		if !c.AccessedKeys[key] {
			logger.Debug("removing unushed cache entry", "key", key)
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to clean up stale cache entry %q: %w", key, err)
			}
		}
	}

	return nil
}
