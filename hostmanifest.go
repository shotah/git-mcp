// Package main is the git-mcp stdio server.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/shotah/git-mcp/server"
)

func writeHostManifest(w io.Writer) error {
	return json.NewEncoder(w).Encode(map[string]any{
		"name":    server.ServerName,
		"command": "git-mcp",
		"args":    []string{"--root", "/workspace", "--tool-tier", "core"},
		"blurb":   "Local git only. No push, pull, or fetch.",
	})
}

func init() {
	if len(os.Args) > 1 && os.Args[1] == "host-manifest" {
		if err := writeHostManifest(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}
}
