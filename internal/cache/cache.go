package cache

import "context"

type Opts struct {
	Path string
}

type Cache struct {
	Path string
}

func New(opts *Opts) (*Cache, error) {
	return &Cache{
		Path: opts.Path,
	}, nil
}

func (c *Cache) Exists(ctx context.Context, key string) bool {
	return false
}

func (c *Cache) Finalize(ctx context.Context) error {
	return nil
}

func (c *Cache) Get(ctx context.Context, key string, outputPath string) error {
	return nil
}

func (c *Cache) Initialize(ctx context.Context) error {
	return nil
}

func (c *Cache) Put(ctx context.Context, key string, inputPath string) error {
	return nil
}
