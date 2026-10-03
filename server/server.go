// Package server initializes the MCP server.
package server

import (
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// ServerName is the host mcp.toml id. Tools are exposed as git__status_get.
const ServerName = "git"

// ServerVersion is overwritten at link time (-X github.com/shotah/git-mcp/server.ServerVersion=...).
var ServerVersion = "0.1.0"

// Instructions is returned on initialize. The host shows it before the tool list.
const Instructions = "Show status, diffs, and recent commits. Call stage_update or commit_create only when the user asked to stage or commit. Do not push."

// New creates a stdio MCP server with tool capabilities.
func New() *mcpserver.MCPServer {
	return mcpserver.NewMCPServer(
		ServerName,
		ServerVersion,
		mcpserver.WithToolCapabilities(true),
		mcpserver.WithInstructions(Instructions),
	)
}
