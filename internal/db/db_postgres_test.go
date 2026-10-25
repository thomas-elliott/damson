package db_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/thomas-elliott/damson/internal/db"
	"github.com/thomas-elliott/damson/internal/testdb"
)

// GORM's lines reach the log as JSON, and neither a slow query nor a failed
// one carries the values bound to it.
func TestOpen_LogsSQLAsJSONWithoutParameterValues(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	gdb, err := db.Open(testdb.RequireDSN(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := gdb.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	const secret = "hunter2-secret-value"
	var out string
	if err := gdb.Raw("SELECT ?::text FROM pg_sleep(0.25)", secret).Scan(&out).Error; err != nil {
		t.Fatalf("slow query: %v", err)
	}
	if out != secret {
		t.Fatalf("query returned %q", out)
	}
	if err := gdb.Exec("SELECT ?::int FROM no_such_table", secret).Error; err == nil {
		t.Fatal("failing query succeeded")
	}
	// A unique violation is answered with 409, so it is not an error line.
	if err := gdb.Exec("DO $$ BEGIN RAISE unique_violation; END $$").Error; err == nil {
		t.Fatal("unique violation succeeded")
	}

	var levels []string
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var rec struct {
			Level string `json:"level"`
			Trace struct {
				SQL string `json:"sql"`
			} `json:"trace"`
		}
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("log line is not JSON: %v (%q)", err, line)
		}
		if strings.Contains(line, secret) {
			t.Fatalf("log line carries a parameter value: %s", line)
		}
		if !strings.Contains(rec.Trace.SQL, "$1") {
			t.Errorf("SQL lost its placeholder: %s", line)
		}
		levels = append(levels, rec.Level)
	}
	if strings.Join(levels, ",") != "WARN,ERROR" {
		t.Fatalf("got levels %v, want the slow query at WARN and the failure at ERROR", levels)
	}
}
