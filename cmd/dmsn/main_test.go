package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thomas-elliott/damson/internal/axi"
)

// isolate points the config at a throwaway file and clears the environment
// overrides, so a test never reads or writes the developer's real credentials.
func isolate(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("DAMSON_CONFIG", path)
	t.Setenv("DAMSON_SERVER", "")
	t.Setenv("DAMSON_TOKEN", "")
	return path
}

// exec runs the CLI as main would and returns its output and exit code.
func exec(t *testing.T, args ...string) (string, int) {
	t.Helper()
	var out bytes.Buffer
	code := run(context.Background(), args, &out)
	return out.String(), code
}

// fakeServer stands in for damson. handlers is keyed by path.
func fakeServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for path, h := range handlers {
		mux.HandleFunc(path, h)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// requireBearer asserts the request carried the expected token, so a handler
// path that forgets to authorise is caught rather than passing silently.
func requireBearer(t *testing.T, r *http.Request, want string) {
	t.Helper()
	if got := r.Header.Get("Authorization"); got != "Bearer "+want {
		t.Errorf("Authorization = %q, want bearer %q", got, want)
	}
}

const testToken = "damson_pat_testsecretvalue"

func meHandler(t *testing.T, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requireBearer(t, r, testToken)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

const meBody = `{
  "id": 1, "email": "t@example.com", "display_name": "Thomas", "role": "user",
  "needs_profile": false,
  "current_library": {"id": 7, "name": "Main", "role": "admin"},
  "token": {"prefix": "damson_pat_abcd1234", "label": "laptop", "scope": "write"}
}`

func TestRun_UnknownCommandIsAUsageError(t *testing.T) {
	isolate(t)
	out, code := exec(t, "frobnicate")
	if code != axi.ExitUsage {
		t.Fatalf("exit = %d, want %d", code, axi.ExitUsage)
	}
	if !strings.Contains(out, "code: usage") || !strings.Contains(out, "frobnicate") {
		t.Fatalf("output should name the bad command:\n%s", out)
	}
	if !strings.Contains(out, "help[") {
		t.Fatalf("a usage error must offer a way forward:\n%s", out)
	}
}

func TestRun_UnknownFlagIsAUsageError(t *testing.T) {
	isolate(t)
	out, code := exec(t, "whoami", "--nope")
	if code != axi.ExitUsage {
		t.Fatalf("exit = %d, want %d", code, axi.ExitUsage)
	}
	if !strings.Contains(out, "code: usage") {
		t.Fatalf("got:\n%s", out)
	}
}

// --help is a request, not a failure: one screen, exit 0, on every command.
func TestRun_HelpExitsZeroEverywhere(t *testing.T) {
	isolate(t)
	for _, args := range [][]string{
		{"help"}, {"--help"}, {"-h"},
		{"login", "--help"}, {"logout", "--help"},
		{"whoami", "--help"}, {"version", "--help"},
	} {
		out, code := exec(t, args...)
		if code != axi.ExitOK {
			t.Fatalf("%v: exit = %d, want 0", args, code)
		}
		if strings.TrimSpace(out) == "" {
			t.Fatalf("%v: help produced no output", args)
		}
		if strings.Contains(out, "code: usage") {
			t.Fatalf("%v: help rendered as an error:\n%s", args, out)
		}
	}
}

// A bare invocation shows live state, not a help screen (AXI principle 8).
func TestRun_BareInvocationShowsStateNotHelpScreen(t *testing.T) {
	isolate(t)
	out, code := exec(t)
	if code != axi.ExitOK {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.HasPrefix(out, "dmsn:\n") {
		t.Fatalf("expected the home block first:\n%s", out)
	}
	if !strings.Contains(out, "connected: false") {
		t.Fatalf("not-signed-in is a state that must be stated:\n%s", out)
	}
	if strings.Contains(out, "usage: dmsn [command]") {
		t.Fatalf("bare invocation printed the help screen:\n%s", out)
	}
}

// A leading flag is an option on the home view, not a command name.
func TestRun_LeadingFlagIsNotACommand(t *testing.T) {
	isolate(t)
	out, code := exec(t, "--quiet")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if strings.Contains(out, "unknown command") {
		t.Fatalf("--quiet was treated as a command:\n%s", out)
	}
}

func TestGlobalOutputFlagsAreHonoured(t *testing.T) {
	isolate(t)

	t.Run("json is valid json", func(t *testing.T) {
		out, _ := exec(t, "version", "--json")
		var v map[string]any
		if err := json.Unmarshal([]byte(out), &v); err != nil {
			t.Fatalf("--json did not produce JSON: %v\n%s", err, out)
		}
		if _, ok := v["version"]; !ok {
			t.Fatalf("missing the version block: %v", v)
		}
	})

	t.Run("fields projects", func(t *testing.T) {
		out, code := exec(t, "version", "--fields", "client")
		if code != axi.ExitOK {
			t.Fatalf("exit %d: %s", code, out)
		}
		if strings.Contains(out, "go:") {
			t.Fatalf("--fields did not restrict the projection:\n%s", out)
		}
	})

	t.Run("unknown field exits 2 and lists the valid ones", func(t *testing.T) {
		out, code := exec(t, "version", "--fields", "bogus")
		if code != axi.ExitUsage {
			t.Fatalf("exit = %d, want %d", code, axi.ExitUsage)
		}
		if !strings.Contains(out, "available fields") {
			t.Fatalf("got:\n%s", out)
		}
	})

	t.Run("no-help drops the help block", func(t *testing.T) {
		out, _ := exec(t, "version", "--no-help")
		if strings.Contains(out, "help[") {
			t.Fatalf("--no-help left a help block:\n%s", out)
		}
	})
}

func TestLogin_VerifiesBeforeWritingAndNeverEchoesTheToken(t *testing.T) {
	path := isolate(t)
	srv := fakeServer(t, map[string]http.HandlerFunc{"/api/me": meHandler(t, meBody)})

	out, code := exec(t, "login", "--server", srv.URL, "--token", testToken)
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if strings.Contains(out, testToken) {
		t.Fatalf("login echoed the secret:\n%s", out)
	}
	if !strings.Contains(out, "Thomas") || !strings.Contains(out, "Main") {
		t.Fatalf("login should confirm who and where:\n%s", out)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("config not written: %v", err)
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatalf("config is not valid JSON: %v", err)
	}
	if cfg.Token != testToken || cfg.Server != srv.URL {
		t.Fatalf("config = %+v", cfg)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("config mode = %04o, want 0600 — a credential must not be group- or world-readable", perm)
	}
}

// A rejected token must leave the existing config untouched: a failed login
// that also logged you out would be worse than useless.
func TestLogin_RejectedTokenWritesNothing(t *testing.T) {
	path := isolate(t)
	if _, err := SaveConfig(Config{Server: "https://old.example.com", Token: "damson_pat_previous"}); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	srv := fakeServer(t, map[string]http.HandlerFunc{
		"/api/me": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
		},
	})

	out, code := exec(t, "login", "--server", srv.URL, "--token", "damson_pat_wrong")
	if code != axi.ExitError {
		t.Fatalf("exit = %d, want %d", code, axi.ExitError)
	}
	if !strings.Contains(out, "nothing was saved") {
		t.Fatalf("the failure must say the config was left alone:\n%s", out)
	}
	if strings.Contains(out, "damson_pat_wrong") {
		t.Fatalf("the rejected secret was echoed:\n%s", out)
	}

	b, _ := os.ReadFile(path)
	var cfg Config
	_ = json.Unmarshal(b, &cfg)
	if cfg.Token != "damson_pat_previous" || cfg.Server != "https://old.example.com" {
		t.Fatalf("a failed login overwrote the config: %+v", cfg)
	}
}

// `--token -` keeps the secret out of argv, where ps and shell history see it.
func TestLogin_TokenFromStdin(t *testing.T) {
	isolate(t)
	srv := fakeServer(t, map[string]http.HandlerFunc{"/api/me": meHandler(t, meBody)})

	var out bytes.Buffer
	g := &globals{server: srv.URL, token: "-"}
	env := &Env{Out: &out, In: strings.NewReader(testToken + "\n"), Globals: g}
	if err := runLogin(context.Background(), env); err != nil {
		t.Fatalf("login: %v", err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Token != testToken {
		t.Fatalf("stdin token not stored, got %q", cfg.Token)
	}

	t.Run("empty stdin is a usage error", func(t *testing.T) {
		env := &Env{Out: &bytes.Buffer{}, In: strings.NewReader("  \n"), Globals: &globals{server: srv.URL, token: "-"}}
		err := runLogin(context.Background(), env)
		if err == nil || axi.AsError(err).Code != axi.CodeUsage {
			t.Fatalf("got %v, want a usage error", err)
		}
	})
}

func TestLogin_WarnsOnCleartext(t *testing.T) {
	isolate(t)
	// httptest serves plain http on 127.0.0.1, which is exempt — so exercise
	// the classifier directly for the cases a real deployment hits.
	for _, tc := range []struct {
		url      string
		wantWarn bool
	}{
		{"http://damson.lan", true},
		{"http://192.168.1.10:8080", true},
		{"https://damson.example.com", false},
		{"http://localhost:8080", false},
		{"http://127.0.0.1:8080", false},
	} {
		got := cleartextWarning(tc.url) != ""
		if got != tc.wantWarn {
			t.Fatalf("cleartextWarning(%q) warned=%v, want %v", tc.url, got, tc.wantWarn)
		}
	}
}

func TestLogin_MissingInputsAreUsageErrors(t *testing.T) {
	isolate(t)
	out, code := exec(t, "login", "--token", testToken)
	if code != axi.ExitUsage || !strings.Contains(out, "--server is required") {
		t.Fatalf("exit=%d out=%s", code, out)
	}
	out, code = exec(t, "login", "--server", "https://example.com")
	if code != axi.ExitUsage || !strings.Contains(out, "--token is required") {
		t.Fatalf("exit=%d out=%s", code, out)
	}
}

func TestWhoami(t *testing.T) {
	isolate(t)
	srv := fakeServer(t, map[string]http.HandlerFunc{"/api/me": meHandler(t, meBody)})
	if _, err := SaveConfig(Config{Server: srv.URL, Token: testToken}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	out, code := exec(t, "whoami")
	if code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	for _, want := range []string{"Thomas", "Main", "admin", "damson_pat_abcd1234", "scope: write", "expires: \"never\""} {
		if !strings.Contains(out, want) {
			t.Fatalf("whoami omitted %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, testToken) {
		t.Fatalf("whoami leaked the full secret:\n%s", out)
	}
}

// A read-only token has to say so before the user discovers it via a 403.
func TestWhoami_FlagsAReadOnlyToken(t *testing.T) {
	isolate(t)
	body := strings.Replace(meBody, `"scope": "write"`, `"scope": "read"`, 1)
	srv := fakeServer(t, map[string]http.HandlerFunc{"/api/me": meHandler(t, body)})
	if _, err := SaveConfig(Config{Server: srv.URL, Token: testToken}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	out, _ := exec(t, "whoami")
	if !strings.Contains(out, "scope: read") {
		t.Fatalf("got:\n%s", out)
	}
	if !strings.Contains(out, "read-only") {
		t.Fatalf("a read token must warn that mutations will fail:\n%s", out)
	}
}

func TestWhoami_NoTokenIsAnAuthErrorWithRecovery(t *testing.T) {
	isolate(t)
	out, code := exec(t, "whoami")
	if code != axi.ExitError {
		t.Fatalf("exit = %d, want %d", code, axi.ExitError)
	}
	if !strings.Contains(out, "code: auth") {
		t.Fatalf("got:\n%s", out)
	}
	if !strings.Contains(out, "dmsn login") {
		t.Fatalf("the error must say how to recover:\n%s", out)
	}
}

func TestLogout_RemovesTheConfigAndIsIdempotent(t *testing.T) {
	path := isolate(t)
	if _, err := SaveConfig(Config{Server: "https://example.com", Token: testToken}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if out, code := exec(t, "logout"); code != axi.ExitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("config still present: %v", err)
	}
	// Logging out twice is not a failure.
	if out, code := exec(t, "logout"); code != axi.ExitOK {
		t.Fatalf("second logout exit = %d: %s", code, out)
	}
}

func TestVersion_WorksWithoutAServer(t *testing.T) {
	isolate(t)
	out, code := exec(t, "version")
	if code != axi.ExitOK {
		t.Fatalf("version must work before login; exit = %d: %s", code, out)
	}
	if !strings.Contains(out, "client:") || !strings.Contains(out, "go:") {
		t.Fatalf("got:\n%s", out)
	}
}

func TestVersion_ReportsTheServerBuild(t *testing.T) {
	isolate(t)
	srv := fakeServer(t, map[string]http.HandlerFunc{
		"/api/health": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"status":"ok","version":"v9.9.9"}`))
		},
	})
	if _, err := SaveConfig(Config{Server: srv.URL, Token: testToken}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	out, _ := exec(t, "version")
	if !strings.Contains(out, "server_version: v9.9.9") {
		t.Fatalf("got:\n%s", out)
	}
}

// An unreachable server is reported as such, with the transport cause intact.
func TestClient_NetworkFailureIsClassified(t *testing.T) {
	isolate(t)
	if _, err := SaveConfig(Config{Server: "http://127.0.0.1:1", Token: testToken}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	out, code := exec(t, "whoami")
	if code != axi.ExitError {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, "code: network") {
		t.Fatalf("got:\n%s", out)
	}
}

// The server's own 4xx message has to reach the user — it is the actionable
// half of a bad request.
func TestClient_ServerMessageSurvives(t *testing.T) {
	isolate(t)
	srv := fakeServer(t, map[string]http.HandlerFunc{
		"/api/me": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"token is read-only"}`))
		},
	})
	if _, err := SaveConfig(Config{Server: srv.URL, Token: testToken}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	out, code := exec(t, "whoami")
	if code != axi.ExitError {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, "token is read-only") {
		t.Fatalf("the server's message was lost:\n%s", out)
	}
}

func TestCredentialResolutionOrder(t *testing.T) {
	isolate(t)
	if _, err := SaveConfig(Config{Server: "https://from-config", Token: "damson_pat_config"}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	t.Run("config is the floor", func(t *testing.T) {
		e := &Env{Globals: &globals{}}
		c, err := e.Client()
		if err != nil {
			t.Fatal(err)
		}
		if c.BaseURL != "https://from-config" || c.Token != "damson_pat_config" {
			t.Fatalf("got %s / %s", c.BaseURL, c.Token)
		}
	})

	t.Run("environment beats config", func(t *testing.T) {
		t.Setenv("DAMSON_SERVER", "https://from-env")
		t.Setenv("DAMSON_TOKEN", "damson_pat_env")
		e := &Env{Globals: &globals{}}
		c, err := e.Client()
		if err != nil {
			t.Fatal(err)
		}
		if c.BaseURL != "https://from-env" || c.Token != "damson_pat_env" {
			t.Fatalf("got %s / %s", c.BaseURL, c.Token)
		}
	})

	t.Run("flag beats everything", func(t *testing.T) {
		t.Setenv("DAMSON_SERVER", "https://from-env")
		t.Setenv("DAMSON_TOKEN", "damson_pat_env")
		e := &Env{Globals: &globals{server: "https://from-flag", token: "damson_pat_flag"}}
		c, err := e.Client()
		if err != nil {
			t.Fatal(err)
		}
		if c.BaseURL != "https://from-flag" || c.Token != "damson_pat_flag" {
			t.Fatalf("got %s / %s", c.BaseURL, c.Token)
		}
	})
}

func TestNewClient_RejectsAMalformedServer(t *testing.T) {
	for _, bad := range []string{"", "   ", "not a url", "example.com"} {
		if _, err := NewClient(bad, testToken, 0); err == nil {
			t.Fatalf("NewClient(%q) should have failed", bad)
		} else if axi.AsError(err).Code != axi.CodeUsage {
			t.Fatalf("NewClient(%q): code = %q, want usage", bad, axi.AsError(err).Code)
		}
	}
}

func TestSaveConfig_TrailingSlashIsNormalised(t *testing.T) {
	isolate(t)
	srv := fakeServer(t, map[string]http.HandlerFunc{"/api/me": meHandler(t, meBody)})
	if _, code := exec(t, "login", "--server", srv.URL+"/", "--token", testToken); code != axi.ExitOK {
		t.Fatal("login failed")
	}
	cfg, _ := LoadConfig()
	if strings.HasSuffix(cfg.Server, "/") {
		t.Fatalf("trailing slash survived: %q — request paths would double up", cfg.Server)
	}
}
