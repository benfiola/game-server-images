package cache

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func (c *Cache) getCachePath(key string) string {
	return filepath.Join(c.Path, fmt.Sprintf("%s.squashfs", key))
}

func (c *Cache) getKey(cachePath string) string {
	return strings.TrimSuffix(filepath.Base(cachePath), ".squashfs")
}

func (c *Cache) runCommand(ctx context.Context, cmd ...string) error {
	execCmd := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	if err := execCmd.Run(); err != nil {
		return err
	}
	return nil
}

func (c *Cache) Exists(ctx context.Context, key string) bool {
	_, err := os.Stat(c.getCachePath(key))
	return err == nil
}

func (c *Cache) Get(ctx context.Context, key string, outputPath string) error {
	cachePath := c.getCachePath(key)

	if _, err := os.Stat(cachePath); err != nil {
		return fmt.Errorf("cache entry not found: %w", err)
	}

	if err := c.runCommand(ctx, "unsquashfs", "-f", "-d", outputPath, cachePath); err != nil {
		return fmt.Errorf("failed to extract cache entry: %w", err)
	}

	c.AccessedKeys[key] = true

	return nil
}

func (c *Cache) Put(ctx context.Context, key string, inputPath string) error {
	cachePath := c.getCachePath(key)

	if err := c.runCommand(ctx, "mksquashfs", inputPath, cachePath); err != nil {
		return fmt.Errorf("failed to create cache entry: %w", err)
	}

	c.AccessedKeys[key] = true

	return nil
}

func (c *Cache) Finalize(ctx context.Context) error {
	paths, err := filepath.Glob(fmt.Sprintf("%s/*.squashfs", c.Path))
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, path := range paths {
		key := c.getKey(path)
		if !c.AccessedKeys[key] {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to clean up stale cache entry %q: %w", key, err)
			}
		}
	}

	return nil
}
