package main

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/thomas-elliott/damson/internal/axi"
	"github.com/thomas-elliott/damson/internal/toon"
)

// maxTokenBytes bounds a token read from stdin so a stray pipe can't be read
// into memory unbounded.
const maxTokenBytes = 4 << 10

// meResponse mirrors GET /api/me. Every wire shape in this package is a local
// struct: importing internal/handlers/api would drag gin into the client
// binary, so the mirrors are hand-kept and guarded by golden fixtures.
type meResponse struct {
	ID             uint   `json:"id"`
	Email          string `json:"email"`
	DisplayName    string `json:"display_name"`
	Role           string `json:"role"`
	CurrentLibrary *struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
		Role string `json:"role"`
	} `json:"current_library"`
	Token *struct {
		Prefix    string     `json:"prefix"`
		Label     string     `json:"label"`
		Scope     string     `json:"scope"`
		ExpiresAt *time.Time `json:"expires_at"`
	} `json:"token"`
}

func init() {
	register(&command{
		name:    "login",
		summary: "store credentials for a damson server",
		usage: "usage: dmsn login --server URL --token TOKEN\n" +
			"       dmsn login --server URL --token -      (read the token from stdin)\n\n" +
			"Verifies the token against the server before writing anything, and\n" +
			"never echoes it back. The file is written with owner-only permissions.\n\n" +
			"A token passed as --token is visible to other users via the process\n" +
			"list and lands in shell history. Prefer `--token -` or DAMSON_TOKEN.\n\n" +
			"Create a token in the damson SPA under Settings → Access tokens.",
		run: runLogin,
	})
	register(&command{
		name:    "logout",
		summary: "discard stored credentials",
		usage:   "usage: dmsn logout",
		run:     runLogout,
	})
	register(&command{
		name:    "whoami",
		summary: "show the active identity, library and token",
		usage:   "usage: dmsn whoami",
		run:     runWhoami,
	})
}

func runLogin(ctx context.Context, e *Env) error {
	cfg, err := LoadConfig()
	if err != nil {
		return axi.Wrap(axi.CodeIO, err, "%v", err)
	}
	flagToken, err := resolveTokenFlag(e)
	if err != nil {
		return err
	}
	server := firstNonEmpty(e.Globals.server, envOr("DAMSON_SERVER"), cfg.Server)
	token := firstNonEmpty(flagToken, envOr("DAMSON_TOKEN"), cfg.Token)

	if server == "" {
		return axi.Usagef(
			[]string{"dmsn login --server https://damson.example.com --token <token>"},
			"--server is required")
	}
	if token == "" {
		return axi.Usagef(
			[]string{
				fmt.Sprintf("dmsn login --server %s --token <token>", server),
				"create a token in the damson SPA under Settings → Access tokens",
			},
			"--token is required")
	}

	client, err := NewClient(server, token, e.Globals.timeout)
	if err != nil {
		return err
	}
	var me meResponse
	if err := client.GetJSON(ctx, "/api/me", nil, &me); err != nil {
		return loginRejected(err)
	}

	path, err := SaveConfig(Config{Server: client.BaseURL, Token: token})
	if err != nil {
		return axi.Wrap(axi.CodeIO, err, "%v", err)
	}

	// The secret is never echoed — only where it now lives.
	fields := []toon.KV{
		{Key: "server", Value: client.BaseURL},
		{Key: "user", Value: identity(me)},
		{Key: "library", Value: libraryName(me)},
		{Key: "scope", Value: tokenScope(me)},
		{Key: "config", Value: displayPath(path)},
	}
	if w := cleartextWarning(client.BaseURL); w != "" {
		fields = append(fields, toon.KV{Key: "warning", Value: toon.Text(w)})
	}
	return e.Record("login", fields, []string{"dmsn", "dmsn assets", "dmsn whoami"})
}

// resolveTokenFlag reads the token, supporting `--token -` so the secret can
// be piped in rather than sitting in argv where `ps` and shell history see it.
func resolveTokenFlag(e *Env) (string, error) {
	raw := strings.TrimSpace(e.Globals.token)
	if raw != "-" {
		return raw, nil
	}
	if e.In == nil {
		return "", axi.Usagef(nil, "--token - needs the token on stdin")
	}
	b, err := io.ReadAll(io.LimitReader(e.In, maxTokenBytes))
	if err != nil {
		return "", axi.Wrap(axi.CodeIO, err, "read token from stdin")
	}
	tok := strings.TrimSpace(string(b))
	if tok == "" {
		return "", axi.Usagef(
			[]string{"printf %s \"$TOKEN\" | dmsn login --server <url> --token -"},
			"--token - was given but stdin was empty")
	}
	return tok, nil
}

// cleartextWarning flags a bearer that would travel unencrypted. Localhost is
// exempt: it never leaves the machine.
func cleartextWarning(base string) string {
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "http" {
		return ""
	}
	switch host := u.Hostname(); host {
	case "localhost", "127.0.0.1", "::1":
		return ""
	default:
		return "this server is http, so the token is sent unencrypted on every request"
	}
}

// loginRejected keeps a failed verification actionable: the two things that
// are actually wrong are the URL and the token, so say which to check.
func loginRejected(err error) error {
	e := axi.AsError(err)
	switch e.Code {
	case axi.CodeAuth:
		return &axi.Error{
			Code:    axi.CodeAuth,
			Message: "the server rejected that token; nothing was saved",
			Help: []string{
				"check the token was copied whole, including the damson_pat_ prefix",
				"create a replacement in the damson SPA under Settings → Access tokens",
			},
			Err: err,
		}
	case axi.CodeForbidden:
		return &axi.Error{
			Code:    axi.CodeForbidden,
			Message: "that token is not permitted here; nothing was saved",
			Err:     err,
		}
	default:
		return err
	}
}

func runLogout(_ context.Context, e *Env) error {
	path, err := ClearConfig()
	if err != nil {
		return axi.Wrap(axi.CodeIO, err, "%v", err)
	}
	return e.Record("logout", []toon.KV{
		{Key: "config", Value: displayPath(path)},
		{Key: "removed", Value: true},
	}, []string{"dmsn login --server <url> --token <token>"})
}

func runWhoami(ctx context.Context, e *Env) error {
	client, err := e.Client()
	if err != nil {
		return err
	}
	var me meResponse
	if err := client.GetJSON(ctx, "/api/me", nil, &me); err != nil {
		return err
	}

	fields := []toon.KV{
		{Key: "server", Value: client.BaseURL},
		{Key: "user", Value: identity(me)},
		{Key: "email", Value: blankToNil(me.Email)},
		{Key: "server_role", Value: me.Role},
		{Key: "library", Value: libraryName(me)},
		{Key: "library_role", Value: libraryRole(me)},
	}
	if me.Token != nil {
		fields = append(fields,
			toon.KV{Key: "token", Value: me.Token.Prefix},
			toon.KV{Key: "token_label", Value: blankToNil(me.Token.Label)},
			toon.KV{Key: "scope", Value: me.Token.Scope},
			toon.KV{Key: "expires", Value: expiryValue(me.Token.ExpiresAt)},
		)
	} else {
		// An interactive bearer works, but it expires in an hour and is not
		// what this tool is meant to run on. Say so rather than showing blanks.
		fields = append(fields, toon.KV{Key: "token", Value: toon.Text("not a personal access token")})
	}

	help := []string{"dmsn", "dmsn assets"}
	if me.Token != nil && me.Token.Scope == "read" {
		help = append(help, "this token is read-only; mutations will fail with code: forbidden")
	}
	return e.Record("whoami", fields, help)
}

func identity(me meResponse) any {
	if s := firstNonEmpty(me.DisplayName, me.Email); s != "" {
		return s
	}
	return nil
}

func libraryName(me meResponse) any {
	if me.CurrentLibrary == nil {
		return nil
	}
	return me.CurrentLibrary.Name
}

func libraryRole(me meResponse) any {
	if me.CurrentLibrary == nil {
		return nil
	}
	return me.CurrentLibrary.Role
}

func tokenScope(me meResponse) any {
	if me.Token == nil {
		return nil
	}
	return me.Token.Scope
}

func expiryValue(t *time.Time) any {
	if t == nil {
		return toon.Text("never")
	}
	return *t
}

func blankToNil(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func envOr(key string) string { return strings.TrimSpace(os.Getenv(key)) }

// writeHelp emits the contextual help block, honouring --no-help / --quiet.
// Suppressed under --json, where the block would corrupt the document.
func writeHelp(e *Env, lines []string) error {
	if len(lines) == 0 || e.Globals.noHelp || e.Globals.quiet || e.Globals.json {
		return nil
	}
	fmt.Fprint(e.Out, (&toon.Doc{}).Lines("help", lines).String())
	return nil
}
