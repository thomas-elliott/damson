package config

import (
	"strings"
	"testing"
	"time"
)

// setBaseEnv clears every variable Load reads and sets the one it requires,
// so a case changes only what it is about.
func setBaseEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"DAMSON_AUTH_MODE", "DAMSON_STORAGE_MODE", "DAMSON_POSTGRES_DSN_FILE",
		"DAMSON_IMPORT_QUEUE_WORKERS", "DAMSON_PREVIEW_QUEUE_WORKERS",
		"DAMSON_IMPORT_JOB_TIMEOUT", "DAMSON_PREVIEW_JOB_TIMEOUT",
		"DAMSON_AUDIT_PURGE_INTERVAL", "DAMSON_ORPHAN_MIN_AGE",
		"DAMSON_UPLOAD_JANITOR_INTERVAL", "DAMSON_STORE_LOOKUP_INTERVAL",
		"DAMSON_JWT_SIGNING_KEY", "DAMSON_JWT_SIGNING_KEY_FILE",
		"DAMSON_LOCAL_ADMIN_PASSWORD", "DAMSON_LOCAL_ADMIN_PASSWORD_FILE",
	} {
		t.Setenv(k, "")
	}
	t.Setenv("DAMSON_POSTGRES_DSN", "postgres://example")
}

func TestLoad_DefaultsWhenUnset(t *testing.T) {
	setBaseEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OrphanMinAge != 7*24*time.Hour || cfg.AuditPurgeInterval != 24*time.Hour ||
		cfg.UploadJanitorInterval != time.Hour || cfg.StoreLookupInterval != 2*time.Second {
		t.Fatalf("defaults drifted: %+v", cfg)
	}
	if cfg.ImportQueueWorkers <= 0 || cfg.ImportJobTimeout != 0 {
		t.Fatalf("queue defaults drifted: workers %d, timeout %v", cfg.ImportQueueWorkers, cfg.ImportJobTimeout)
	}
}

func TestLoad_ReadsValidValues(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("DAMSON_IMPORT_QUEUE_WORKERS", "3")
	t.Setenv("DAMSON_ORPHAN_MIN_AGE", "336h")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ImportQueueWorkers != 3 || cfg.OrphanMinAge != 14*24*time.Hour {
		t.Fatalf("got workers %d, min age %v", cfg.ImportQueueWorkers, cfg.OrphanMinAge)
	}
}

// An invalid value used to fall back to the default silently. It is an error
// naming the variable now, and every bad variable is reported in one go.
func TestLoad_RejectsInvalidValues(t *testing.T) {
	cases := map[string]string{
		"DAMSON_IMPORT_QUEUE_WORKERS":  "abc",
		"DAMSON_PREVIEW_QUEUE_WORKERS": "0",
		"DAMSON_IMPORT_JOB_TIMEOUT":    "soon",
		"DAMSON_ORPHAN_MIN_AGE":        "-5h",
		"DAMSON_AUTH_MODE":             "ldap",
		"DAMSON_STORAGE_MODE":          "ftp",
	}
	setBaseEnv(t)
	for k, v := range cases {
		t.Setenv(k, v)
	}
	_, err := Load()
	if err == nil {
		t.Fatal("Load accepted invalid values")
	}
	for k := range cases {
		if !strings.Contains(err.Error(), k) {
			t.Errorf("error does not name %s: %v", k, err)
		}
	}
}

func TestLoad_RejectsMissingRequiredValues(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("DAMSON_POSTGRES_DSN", "")
	t.Setenv("DAMSON_AUTH_MODE", "oidc")
	_, err := Load()
	if err == nil {
		t.Fatal("Load accepted a missing DSN")
	}
	for _, k := range []string{"DAMSON_POSTGRES_DSN", "DAMSON_OIDC_ISSUER_URL"} {
		if !strings.Contains(err.Error(), k) {
			t.Errorf("error does not name %s: %v", k, err)
		}
	}
}

func TestLoad_RejectsBothSecretForms(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("DAMSON_POSTGRES_DSN_FILE", "/run/secrets/dsn")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "DAMSON_POSTGRES_DSN_FILE") {
		t.Fatalf("got %v, want an error naming both forms", err)
	}
	if strings.Contains(err.Error(), "required secret") {
		t.Errorf("one mistake reported twice: %v", err)
	}
}
