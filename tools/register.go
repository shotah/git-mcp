package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// Tool names — service_verb, no server-id prefix.
// Host mcp.toml name is git → git__status_get, git__diff_get, …
const (
	ToolStatus  = "status_get"
	ToolDiff    = "diff_get"
	ToolCommits = "commits_list"
	ToolStage   = "stage_update"
	ToolCommit  = "commit_create"

	TierCore = "core"
)

const (
	descStatus  = "Show the working tree status."
	descDiff    = "Show the working tree diff, or the diff for one revision."
	descCommits = "List recent commits (git log)."
	descStage   = "Stage paths (git add) for the next commit."
	descCommit  = "Create a commit from the index. Call only when the user asked for a commit."
)

// ToolNames is the core catalog. Push, pull, fetch, rebase, and reset are not registered.
func ToolNames(tier string) ([]string, error) {
	switch tier {
	case "", TierCore:
		return []string{ToolStatus, ToolDiff, ToolCommits, ToolStage, ToolCommit}, nil
	default:
		return nil, fmt.Errorf("invalid --tool-tier %q (want core)", tier)
	}
}

// Register publishes the core tools. root is an already-cleaned jail.
func Register(s *mcpserver.MCPServer, root, tier string) (int, error) {
	names, err := ToolNames(tier)
	if err != nil {
		return 0, err
	}
	for _, name := range names {
		switch name {
		case ToolStatus:
			s.AddTool(mcp.NewTool(name,
				mcp.WithDescription(descStatus),
				mcp.WithReadOnlyHintAnnotation(true),
			), func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return textCall(Status(ctx, root))
			})
		case ToolDiff:
			s.AddTool(mcp.NewTool(name,
				mcp.WithDescription(descDiff),
				mcp.WithString("revision", mcp.Description("Commit or rev to show. Empty shows the working tree against HEAD.")),
				mcp.WithString("path", mcp.Description("Optional path limit, relative to the workspace root.")),
				mcp.WithReadOnlyHintAnnotation(true),
			), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return textCall(Diff(ctx, root, strings.TrimSpace(req.GetString("revision", "")), req.GetString("path", "")))
			})
		case ToolCommits:
			s.AddTool(mcp.NewTool(name,
				mcp.WithDescription(descCommits),
				mcp.WithNumber("limit", mcp.Description("Maximum commits. Default 20, max 50.")),
				mcp.WithReadOnlyHintAnnotation(true),
			), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return textCall(Commits(ctx, root, req.GetInt("limit", 0)))
			})
		case ToolStage:
			s.AddTool(mcp.NewTool(name,
				mcp.WithDescription(descStage+" Does not commit."),
				mcp.WithArray("paths", mcp.Required(), mcp.Description("Paths to stage, relative to the workspace root."), mcp.WithStringItems()),
				mcp.WithDestructiveHintAnnotation(true),
			), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				paths, err := req.RequireStringSlice("paths")
				if err != nil || len(paths) == 0 {
					return mcp.NewToolResultError("paths is required"), nil
				}
				return textCall(Stage(ctx, root, paths))
			})
		case ToolCommit:
			s.AddTool(mcp.NewTool(name,
				mcp.WithDescription(descCommit+" Does not stage and does not push."),
				mcp.WithString("message", mcp.Required(), mcp.Description("Commit message.")),
				mcp.WithDestructiveHintAnnotation(true),
			), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				message, err := req.RequireString("message")
				if err != nil || strings.TrimSpace(message) == "" {
					return mcp.NewToolResultError("message is required"), nil
				}
				return textCall(Commit(ctx, root, message))
			})
		default:
			return 0, fmt.Errorf("unknown tool %q", name)
		}
	}
	return len(names), nil
}

func textCall(text string, err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(text), nil
}
