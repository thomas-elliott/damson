// Package config loads the server's bootstrap configuration from environment
// variables. Runtime-editable settings live in the database; see
// docs/archive/technical-design.md §14.
package config

import (
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
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
	OIDCIssuerURL string
	OIDCClientID  string
	OIDCAudience  string

	// Local mode
	JWTSigningKey      []byte
	JWTKeyFile         string // consulted when JWTSigningKey is empty
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

	// ImportWorkers caps the size of the background import goroutine pool.
	// Zero or negative values mean "use runtime.GOMAXPROCS(0)".
	ImportWorkers int

	// AuditPurgeInterval is how often the audit retention purge job sweeps
	// audit_log against per-library `library_settings.audit_retention_days`.
	// Defaults to 24h; an unparseable or non-positive value falls back to
	// the default.
	AuditPurgeInterval time.Duration
}

func Load() Config {
	// Optional: load ./.env if present. Real environment variables always
	// win — godotenv.Load does not overwrite existing values.
	_ = godotenv.Load()

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
		ListenAddr:         getenv("DAMSON_LISTEN_ADDR", ":8080"),
		BaseURL:            getenv("DAMSON_BASE_URL", "http://localhost:8080"),
		PostgresDSN:        mustGetSecret("DAMSON_POSTGRES_DSN"),
		AuthMode:           authMode,
		StorageMode:        storageMode,
		LogLevel:           getenv("DAMSON_LOG_LEVEL", "info"),
		GinMode:            os.Getenv("GIN_MODE"),
		ImportWorkers:      parseImportWorkers(os.Getenv("DAMSON_IMPORT_WORKERS")),
		AuditPurgeInterval: parseAuditPurgeInterval(os.Getenv("DAMSON_AUDIT_PURGE_INTERVAL")),
	}

	switch authMode {
	case AuthModeOIDC:
		cfg.OIDCIssuerURL = mustGetEnv("DAMSON_OIDC_ISSUER_URL")
		cfg.OIDCClientID = mustGetEnv("DAMSON_OIDC_CLIENT_ID")
		cfg.OIDCAudience = mustGetEnv("DAMSON_OIDC_AUDIENCE")
	case AuthModeLocal:
		// DAMSON_JWT_SIGNING_KEY is honoured verbatim when set; otherwise
		// main.go falls back to reading (or generating) JWTKeyFile. This
		// keeps the zero-config dev path working — the operator only has
		// to set DAMSON_POSTGRES_DSN.
		if raw := getSecret("DAMSON_JWT_SIGNING_KEY"); raw != "" {
			cfg.JWTSigningKey = []byte(raw)
		}
		cfg.JWTKeyFile = getenv("DAMSON_JWT_KEY_FILE", "./data/jwt.key")
		// Optional: if both are set, seed the first admin at startup. If
		// unset, the first browser hit will see `needs_bootstrap=true` on
		// /api/auth/mode and land on the in-app "create admin" page.
		cfg.LocalAdminEmail = os.Getenv("DAMSON_LOCAL_ADMIN_EMAIL")
		cfg.LocalAdminPassword = getSecret("DAMSON_LOCAL_ADMIN_PASSWORD")
	}

	switch storageMode {
	case StorageModeS3:
		cfg.S3Endpoint = mustGetEnv("DAMSON_S3_ENDPOINT")
		cfg.S3Region = mustGetEnv("DAMSON_S3_REGION")
		cfg.S3Bucket = mustGetEnv("DAMSON_S3_BUCKET")
		cfg.S3AccessKey = mustGetSecret("DAMSON_S3_ACCESS_KEY")
		cfg.S3SecretKey = mustGetSecret("DAMSON_S3_SECRET_KEY")
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

// getSecret returns the value of the named env var, or if `<key>_FILE` is set,
// the trimmed contents of that file. Supports the Docker Secrets pattern where
// secrets are mounted at /run/secrets/<name> and surfaced via a *_FILE env.
// If both the direct var and the _FILE variant are set, startup panics —
// ambiguous configuration is a deployment error.
func getSecret(key string) string {
	direct := os.Getenv(key)
	filePath := os.Getenv(key + "_FILE")
	if direct != "" && filePath != "" {
		log.Panicf("config: set either %s or %s_FILE, not both", key, key)
	}
	if filePath != "" {
		b, err := os.ReadFile(filePath)
		if err != nil {
			log.Panicf("config: read %s_FILE=%q: %v", key, filePath, err)
		}
		return strings.TrimRight(string(b), "\r\n")
	}
	return direct
}

func mustGetSecret(key string) string {
	v := getSecret(key)
	if v == "" {
		log.Panicf("config: required secret %s (or %s_FILE) is empty", key, key)
	}
	return v
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// parseImportWorkers reads the DAMSON_IMPORT_WORKERS env var. An empty,
// invalid, or non-positive value falls back to runtime.GOMAXPROCS(0) so a
// fresh deploy gets a sane default.
func parseImportWorkers(raw string) int {
	if raw == "" {
		return runtime.GOMAXPROCS(0)
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return runtime.GOMAXPROCS(0)
	}
	return n
}

const defaultAuditPurgeInterval = 24 * time.Hour

// parseAuditPurgeInterval reads DAMSON_AUDIT_PURGE_INTERVAL as a Go duration
// string ("24h", "30m", "12h"). Empty / unparseable / non-positive values
// fall back to defaultAuditPurgeInterval.
func parseAuditPurgeInterval(raw string) time.Duration {
	if raw == "" {
		return defaultAuditPurgeInterval
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return defaultAuditPurgeInterval
	}
	return d
}
