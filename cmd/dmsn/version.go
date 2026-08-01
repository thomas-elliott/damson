package main

import (
	"context"

	"github.com/thomas-elliott/damson/internal/buildinfo"
	"github.com/thomas-elliott/damson/internal/toon"
)

func init() {
	register(&command{
		name:    "version",
		summary: "show client and server versions",
		usage: "usage: dmsn version\n\n" +
			"The server version needs a reachable server; without one, only the\n" +
			"client half is reported.",
		run: runVersion,
	})
}

func runVersion(ctx context.Context, e *Env) error {
	fields := []toon.KV{
		{Key: "client", Value: clientVersion()},
		{Key: "commit", Value: clientCommit()},
		{Key: "go", Value: goVersion()},
	}

	// The server half is a bonus, not a precondition: `version` has to work
	// before anyone has logged in.
	client, err := e.Client()
	if err == nil {
		var health struct {
			Version string `json:"version"`
		}
		var serverVersion any
		if hErr := client.GetJSON(ctx, "/api/health", nil, &health); hErr == nil {
			serverVersion = health.Version
		}
		fields = append(fields,
			toon.KV{Key: "server", Value: client.BaseURL},
			toon.KV{Key: "server_version", Value: serverVersion})
	}

	return e.Record("version", fields, []string{"dmsn", "dmsn whoami"})
}

func clientVersion() string {
	if v := buildinfo.Version(); v != "" {
		return v
	}
	return "dev"
}

func clientCommit() any {
	return blankToNil(buildinfo.Commit())
}

func goVersion() string {
	if g := buildinfo.GoVersion(); g != "" {
		return g
	}
	return "unknown"
}
