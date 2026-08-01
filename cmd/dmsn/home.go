package main

import (
	"context"
	"os"

	"github.com/thomas-elliott/damson/internal/axi"
	"github.com/thomas-elliott/damson/internal/toon"
)

// homeCommand is what a bare `dmsn` runs. Per AXI principle 8 it shows live
// state rather than a help screen, so an agent that types the binary name
// learns what it is connected to in one call.
var homeCommand = &command{
	name:    "",
	summary: "show the current library at a glance",
	usage:   "usage: dmsn",
	hidden:  true,
	run:     runHome,
}

func runHome(ctx context.Context, e *Env) error {
	self := Block{Key: binaryName, Fields: []toon.KV{
		{Key: "version", Value: clientVersion()},
		{Key: "path", Value: displayPath(selfPath())},
		{Key: "about", Value: toon.Text("manage a damson asset library: upload, tag, collect, curate")},
	}}

	client, err := e.Client()
	if err != nil {
		// Not signed in is a state, not a failure: say so, and say what to do.
		return e.Records([]Block{self, disconnected(err)}, []string{
			"dmsn login --server <url> --token <token>",
			"dmsn help",
		})
	}

	var me meResponse
	if err := client.GetJSON(ctx, "/api/me", nil, &me); err != nil {
		return e.Records([]Block{self, disconnected(err)}, []string{
			"dmsn whoami",
			"dmsn login --server <url> --token <token>",
		})
	}

	library := Block{Key: "library", Fields: []toon.KV{
		{Key: "connected", Value: true},
		{Key: "server", Value: client.BaseURL},
		{Key: "name", Value: libraryName(me)},
		{Key: "user", Value: identity(me)},
	}}
	return e.Records([]Block{self, library}, []string{
		"dmsn assets",
		"dmsn upload <path> --as asset",
		"dmsn whoami",
		"dmsn help",
	})
}

func disconnected(err error) Block {
	return Block{Key: "library", Fields: []toon.KV{
		{Key: "connected", Value: false},
		{Key: "reason", Value: toon.Text(axi.AsError(err).Message)},
	}}
}

// selfPath resolves the running binary, falling back to the bare name when
// the OS won't say.
func selfPath() string {
	p, err := os.Executable()
	if err != nil || p == "" {
		return binaryName
	}
	return p
}
