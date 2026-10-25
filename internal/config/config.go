// Package config loads the server's bootstrap configuration from environment
// variables. Runtime-editable settings live in the database.
package config

import (
	"errors"
	"fmt"
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

	GinMode string

	// ImportQueueWorkers / PreviewQueueWorkers cap the number of concurrent
	// River workers on each queue. Unset means runtime.GOMAXPROCS(0).
	ImportQueueWorkers  int
	PreviewQueueWorkers int

	// ImportJobTimeout / PreviewJobTimeout bound how long a single job may run
	// before its context is cancelled. Unset (zero) means the queue default
	// (30 min). Imports can be multi-GB, hence the generous default.
	ImportJobTimeout  time.Duration
	PreviewJobTimeout time.Duration

	// AuditPurgeInterval is how often the audit retention purge job sweeps
	// audit_log against per-library `library_settings.audit_retention_days`.
	// Defaults to 24h.
	AuditPurgeInterval time.Duration

	// UploadJanitorInterval is how often abandoned resumable upload sessions
	// are expired and their backing multipart uploads aborted. Defaults to 1h.
	UploadJanitorInterval time.Duration

	// OrphanMinAge is the minimum age an object must reach before the manual
	// storage-cleanup sweep will consider it orphaned. It guards against
	// deleting in-flight uploads whose DB rows have not yet committed. Defaults
	// to 7 days.
	OrphanMinAge time.Duration

	// StoreLookupInterval is the least time between two lookups to one store,
	// for every caller. Defaults to 2s.
	StoreLookupInterval time.Duration

	// TrustedProxies lists the proxies (IPs or CIDRs) whose X-Forwarded-For
	// names the client, from the comma-separated DAMSON_TRUSTED_PROXIES. Empty
	// trusts none: the client IP is the connection's address.
	TrustedProxies []string
}

// Load reads the configuration from the environment. A required variable
// that is missing, or a value that does not parse, is an error naming the
// variable; every such error is reported at once.
func Load() (Config, error) {
	// Optional: load ./.env if present. Real environment variables always
	// win — godotenv.Load does not overwrite existing values.
	_ = godotenv.Load()

	var l loader

	authMode := AuthMode(getenv("DAMSON_AUTH_MODE", string(AuthModeLocal)))
	switch authMode {
	case AuthModeOIDC, AuthModeLocal:
	default:
		l.fail("DAMSON_AUTH_MODE must be 'oidc' or 'local', got %q", authMode)
	}

	storageMode := StorageMode(getenv("DAMSON_STORAGE_MODE", string(StorageModeLocal)))
	switch storageMode {
	case StorageModeS3, StorageModeLocal:
	default:
		l.fail("DAMSON_STORAGE_MODE must be 's3' or 'local', got %q", storageMode)
	}

	cfg := Config{
		ListenAddr:            getenv("DAMSON_LISTEN_ADDR", ":8080"),
		BaseURL:               getenv("DAMSON_BASE_URL", "http://localhost:8080"),
		PostgresDSN:           l.mustGetSecret("DAMSON_POSTGRES_DSN"),
		AuthMode:              authMode,
		StorageMode:           storageMode,
		GinMode:               os.Getenv("GIN_MODE"),
		ImportQueueWorkers:    l.positiveInt("DAMSON_IMPORT_QUEUE_WORKERS", runtime.GOMAXPROCS(0)),
		PreviewQueueWorkers:   l.positiveInt("DAMSON_PREVIEW_QUEUE_WORKERS", runtime.GOMAXPROCS(0)),
		ImportJobTimeout:      l.duration("DAMSON_IMPORT_JOB_TIMEOUT", 0),
		PreviewJobTimeout:     l.duration("DAMSON_PREVIEW_JOB_TIMEOUT", 0),
		AuditPurgeInterval:    l.duration("DAMSON_AUDIT_PURGE_INTERVAL", defaultAuditPurgeInterval),
		OrphanMinAge:          l.duration("DAMSON_ORPHAN_MIN_AGE", defaultOrphanMinAge),
		UploadJanitorInterval: l.duration("DAMSON_UPLOAD_JANITOR_INTERVAL", defaultUploadJanitorInterval),
		StoreLookupInterval:   l.duration("DAMSON_STORE_LOOKUP_INTERVAL", defaultStoreLookupInterval),
		TrustedProxies:        parseList(os.Getenv("DAMSON_TRUSTED_PROXIES")),
	}

	switch authMode {
	case AuthModeOIDC:
		cfg.OIDCIssuerURL = l.mustGetEnv("DAMSON_OIDC_ISSUER_URL")
		cfg.OIDCClientID = l.mustGetEnv("DAMSON_OIDC_CLIENT_ID")
		cfg.OIDCAudience = l.mustGetEnv("DAMSON_OIDC_AUDIENCE")
	case AuthModeLocal:
		// DAMSON_JWT_SIGNING_KEY is honoured verbatim when set; otherwise
		// main.go falls back to reading (or generating) JWTKeyFile. This
		// keeps the zero-config dev path working — the operator only has
		// to set DAMSON_POSTGRES_DSN.
		if raw := l.getSecret("DAMSON_JWT_SIGNING_KEY"); raw != "" {
			cfg.JWTSigningKey = []byte(raw)
		}
		cfg.JWTKeyFile = getenv("DAMSON_JWT_KEY_FILE", "./data/jwt.key")
		// Optional: if both are set, seed the first admin at startup. If
		// unset, the first browser hit will see `needs_bootstrap=true` on
		// /api/auth/mode and land on the in-app "create admin" page.
		cfg.LocalAdminEmail = os.Getenv("DAMSON_LOCAL_ADMIN_EMAIL")
		cfg.LocalAdminPassword = l.getSecret("DAMSON_LOCAL_ADMIN_PASSWORD")
	}

	switch storageMode {
	case StorageModeS3:
		cfg.S3Endpoint = l.mustGetEnv("DAMSON_S3_ENDPOINT")
		cfg.S3Region = l.mustGetEnv("DAMSON_S3_REGION")
		cfg.S3Bucket = l.mustGetEnv("DAMSON_S3_BUCKET")
		cfg.S3AccessKey = l.mustGetSecret("DAMSON_S3_ACCESS_KEY")
		cfg.S3SecretKey = l.mustGetSecret("DAMSON_S3_SECRET_KEY")
		cfg.S3ForcePathStyle = l.bool("DAMSON_S3_FORCE_PATH_STYLE", true)
	case StorageModeLocal:
		cfg.StorageLocalPath = getenv("DAMSON_STORAGE_LOCAL_PATH", "./data/blobs")
	}

	if err := errors.Join(l.errs...); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// loader collects every configuration error, so one failed start reports all
// of them rather than the first.
type loader struct {
	errs []error
}

func (l *loader) fail(format string, args ...any) {
	l.errs = append(l.errs, fmt.Errorf("config: "+format, args...))
}

func (l *loader) mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		l.fail("required env var %s is empty", key)
	}
	return v
}

// getSecret returns the value of the named env var, or if `<key>_FILE` is set,
// the trimmed contents of that file. Supports the Docker Secrets pattern where
// secrets are mounted at /run/secrets/<name> and surfaced via a *_FILE env.
// Setting both the direct var and the _FILE variant is an error.
func (l *loader) getSecret(key string) string {
	direct := os.Getenv(key)
	filePath := os.Getenv(key + "_FILE")
	if direct != "" && filePath != "" {
		l.fail("set either %s or %s_FILE, not both", key, key)
		return ""
	}
	if filePath != "" {
		b, err := os.ReadFile(filePath)
		if err != nil {
			l.fail("read %s_FILE=%q: %v", key, filePath, err)
			return ""
		}
		return strings.TrimRight(string(b), "\r\n")
	}
	return direct
}

func (l *loader) mustGetSecret(key string) string {
	failed := len(l.errs)
	v := l.getSecret(key)
	if v == "" && len(l.errs) == failed {
		l.fail("required secret %s (or %s_FILE) is empty", key, key)
	}
	return v
}

// positiveInt reads a whole number above zero. Empty means fallback.
func (l *loader) positiveInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		l.fail("%s must be a whole number above zero, got %q", key, raw)
		return fallback
	}
	return n
}

// duration reads a positive Go duration string ("30m", "24h"). Empty means
// fallback.
func (l *loader) duration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		l.fail("%s must be a positive duration such as 30m or 24h, got %q", key, raw)
		return fallback
	}
	return d
}

// bool reads true or false. Empty means fallback.
func (l *loader) bool(key string, fallback bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		l.fail("%s must be true or false, got %q", key, raw)
		return fallback
	}
	return b
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

const (
	defaultUploadJanitorInterval = time.Hour
	defaultStoreLookupInterval   = 2 * time.Second
	defaultAuditPurgeInterval    = 24 * time.Hour
	defaultOrphanMinAge          = 7 * 24 * time.Hour
)

// parseList splits a comma-separated value, dropping blank entries.
func parseList(raw string) []string {
	var out []string
	for _, v := range strings.Split(raw, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
