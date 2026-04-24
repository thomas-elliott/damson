// Package config loads the server's bootstrap configuration from environment
// variables. Runtime-editable settings live in the database; see
// docs/technical-design.md §14.
package config

import (
	"log"
	"os"
	"strings"
)

// AuthMode selects the authenticator implementation. Exclusive — one or the
// other, never both, picked at startup and never changed.
type AuthMode string

const (
	AuthModeOIDC  AuthMode = "oidc"
	AuthModeLocal AuthMode = "local"
)

// StorageMode selects the blob/thumbnail storage backend.
type StorageMode string

const (
	StorageModeS3    StorageMode = "s3"
	StorageModeLocal StorageMode = "local"
)

type Config struct {
	ListenAddr string
	BaseURL    string

	PostgresDSN string

	AuthMode AuthMode

	// OIDC mode
	OIDCIssuerURL   string
	OIDCClientID    string
	OIDCAudience    string
	BootstrapAdmins []string

	// Local mode
	JWTSigningKey      []byte
	LocalAdminEmail    string
	LocalAdminPassword string

	StorageMode StorageMode

	// S3 mode
	S3Endpoint       string
	S3Region         string
	S3Bucket         string
	S3AccessKey      string
	S3SecretKey      string
	S3ForcePathStyle bool

	// Local storage mode
	StorageLocalPath string

	LogLevel string
	GinMode  string
}

func Load() Config {
	authMode := AuthMode(getenv("DAMSON_AUTH_MODE", string(AuthModeLocal)))
	switch authMode {
	case AuthModeOIDC, AuthModeLocal:
	default:
		log.Panicf("config: DAMSON_AUTH_MODE must be 'oidc' or 'local', got %q", authMode)
	}

	storageMode := StorageMode(getenv("DAMSON_STORAGE_MODE", string(StorageModeLocal)))
	switch storageMode {
	case StorageModeS3, StorageModeLocal:
	default:
		log.Panicf("config: DAMSON_STORAGE_MODE must be 's3' or 'local', got %q", storageMode)
	}

	cfg := Config{
		ListenAddr:  getenv("DAMSON_LISTEN_ADDR", ":8080"),
		BaseURL:     getenv("DAMSON_BASE_URL", "http://localhost:8080"),
		PostgresDSN: mustGetEnv("DAMSON_POSTGRES_DSN"),
		AuthMode:    authMode,
		StorageMode: storageMode,
		LogLevel:    getenv("DAMSON_LOG_LEVEL", "info"),
		GinMode:     os.Getenv("GIN_MODE"),
	}

	switch authMode {
	case AuthModeOIDC:
		cfg.OIDCIssuerURL = mustGetEnv("DAMSON_OIDC_ISSUER_URL")
		cfg.OIDCClientID = mustGetEnv("DAMSON_OIDC_CLIENT_ID")
		cfg.OIDCAudience = mustGetEnv("DAMSON_OIDC_AUDIENCE")
		cfg.BootstrapAdmins = splitCSV(os.Getenv("DAMSON_BOOTSTRAP_ADMIN_EMAILS"))
	case AuthModeLocal:
		cfg.JWTSigningKey = []byte(mustGetEnv("DAMSON_JWT_SIGNING_KEY"))
		// Optional: if both are set, seed the first admin at startup. If
		// unset, the first browser hit will see `needs_bootstrap=true` on
		// /api/auth/mode and land on the in-app "create admin" page.
		cfg.LocalAdminEmail = os.Getenv("DAMSON_LOCAL_ADMIN_EMAIL")
		cfg.LocalAdminPassword = os.Getenv("DAMSON_LOCAL_ADMIN_PASSWORD")
	}

	switch storageMode {
	case StorageModeS3:
		cfg.S3Endpoint = mustGetEnv("DAMSON_S3_ENDPOINT")
		cfg.S3Region = mustGetEnv("DAMSON_S3_REGION")
		cfg.S3Bucket = mustGetEnv("DAMSON_S3_BUCKET")
		cfg.S3AccessKey = mustGetEnv("DAMSON_S3_ACCESS_KEY")
		cfg.S3SecretKey = mustGetEnv("DAMSON_S3_SECRET_KEY")
		cfg.S3ForcePathStyle = getenv("DAMSON_S3_FORCE_PATH_STYLE", "true") == "true"
	case StorageModeLocal:
		cfg.StorageLocalPath = getenv("DAMSON_STORAGE_LOCAL_PATH", "./data/blobs")
	}

	return cfg
}

func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Panicf("config: required env var %s is empty", key)
	}
	return v
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
