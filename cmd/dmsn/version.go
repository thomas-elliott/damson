package main

import (
	"context"
	"runtime/debug"
	"sync"

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
			Status  string `json:"status"`
			Version string `json:"version"`
		}
		if hErr := client.GetJSON(ctx, "/api/health", nil, &health); hErr == nil {
			fields = append(fields,
				toon.KV{Key: "server", Value: client.BaseURL},
				toon.KV{Key: "server_version", Value: health.Version})
		} else {
			fields = append(fields,
				toon.KV{Key: "server", Value: client.BaseURL},
				toon.KV{Key: "server_version", Value: nil})
		}
	}

	return e.Record("version", fields, []string{"dmsn", "dmsn whoami"})
}

var buildInfo = sync.OnceValues(debug.ReadBuildInfo)

func clientVersion() string {
	info, ok := buildInfo()
	if !ok {
		return "dev"
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

func clientCommit() any {
	info, ok := buildInfo()
	if !ok {
		return nil
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && s.Value != "" {
			if len(s.Value) > 12 {
				return s.Value[:12]
			}
			return s.Value
		}
	}
	return nil
}

func goVersion() string {
	if info, ok := buildInfo(); ok && info.GoVersion != "" {
		return info.GoVersion
	}
	return "unknown"
}
