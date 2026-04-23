package storage

import (
	"context"
	"fmt"

	"github.com/thomas-elliott/damson/internal/config"
)

// New constructs a Service from the supplied config. Picks between the local
// filesystem and S3 backends based on cfg.StorageMode.
func New(ctx context.Context, cfg config.Config) (Service, error) {
	switch cfg.StorageMode {
	case config.StorageModeLocal:
		return NewLocalFS(cfg.StorageLocalPath, cfg.BaseURL)
	case config.StorageModeS3:
		return NewS3(ctx, S3Config{
			Endpoint:       cfg.S3Endpoint,
			Region:         cfg.S3Region,
			Bucket:         cfg.S3Bucket,
			AccessKey:      cfg.S3AccessKey,
			SecretKey:      cfg.S3SecretKey,
			ForcePathStyle: cfg.S3ForcePathStyle,
		})
	default:
		return nil, fmt.Errorf("unknown storage mode %q", cfg.StorageMode)
	}
}
