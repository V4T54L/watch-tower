package domain

import "context"

type Client interface {
	Upload(ctx context.Context, key string, data []byte) error
	Download(ctx context.Context, key string) ([]byte, error)
}
