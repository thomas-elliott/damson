package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The test binary re-runs itself with this set, and then runs main for real.
const runMainEnv = "DAMSON_TEST_RUN_MAIN"

// A config error, missing or invalid, ends the process with exit 1 and one
// JSON log line naming the variable, never a panic and its stack.
func TestMain_ConfigErrorExitsWithAnSlogLine(t *testing.T) {
	if os.Getenv(runMainEnv) == "1" {
		main()
		return
	}
	cases := map[string]struct {
		env      []string
		variable string
	}{
		"invalid": {[]string{"DAMSON_POSTGRES_DSN=postgres://unused", "DAMSON_IMPORT_QUEUE_WORKERS=abc"}, "DAMSON_IMPORT_QUEUE_WORKERS"},
		"missing": {[]string{"DAMSON_POSTGRES_DSN="}, "DAMSON_POSTGRES_DSN"},
		"logging": {[]string{"DAMSON_POSTGRES_DSN=postgres://unused", "DAMSON_LOG_LEVEL=loud"}, "DAMSON_LOG_LEVEL"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestMain_ConfigErrorExitsWithAnSlogLine$")
			cmd.Dir = t.TempDir() // no .env to load
			cmd.Env = append(os.Environ(), runMainEnv+"=1", "DAMSON_LOG_FORMAT=json", "DAMSON_POSTGRES_DSN_FILE=")
			cmd.Env = append(cmd.Env, tc.env...)
			var stderr strings.Builder
			cmd.Stderr = &stderr
			out, err := cmd.Output()

			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Fatalf("exit: %v, want status 1 (stdout %q, stderr %q)", err, out, stderr.String())
			}
			if strings.Contains(stderr.String(), "panic") || strings.Contains(string(out), "goroutine ") {
				t.Fatalf("startup panicked: %s", stderr.String())
			}
			var rec map[string]any
			if err := json.Unmarshal([]byte(strings.TrimSpace(string(out))), &rec); err != nil {
				t.Fatalf("stdout is not one JSON line: %v (%q)", err, out)
			}
			if rec["level"] != "ERROR" || !strings.Contains(rec["err"].(string), tc.variable) {
				t.Fatalf("log line %v does not name %s at ERROR", rec, tc.variable)
			}
		})
	}
}
