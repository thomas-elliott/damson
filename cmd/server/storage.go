package main

import (
	"context"
	"fmt"

	"github.com/thomas-elliott/damson/internal/config"
	"github.com/thomas-elliott/damson/internal/storage"
)

// newStorage builds the object store that cfg.StorageMode names.
func newStorage(ctx context.Context, cfg config.Config) (storage.Service, error) {
	switch cfg.StorageMode {
	case config.StorageModeLocal:
		return storage.NewLocalFS(cfg.StorageLocalPath)
	case config.StorageModeS3:
		return storage.NewS3(ctx, storage.S3Config{
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
