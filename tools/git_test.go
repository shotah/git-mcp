package tools

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/shotah/git-mcp/server"
)

func TestToolNamesLocked(t *testing.T) {
	t.Parallel()
	re := regexp.MustCompile(`^[a-z]+_[a-z]+`)
	names, err := ToolNames(TierCore)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		ToolStatus: true, ToolDiff: true, ToolCommits: true, ToolStage: true, ToolCommit: true,
	}
	if len(names) != len(want) {
		t.Fatalf("names = %v", names)
	}
	for _, name := range names {
		if !re.MatchString(name) {
			t.Errorf("tool %q does not match ^[a-z]+_[a-z]+", name)
		}
		if strings.HasPrefix(name, server.ServerName) {
			t.Errorf("tool %q starts with server id %s", name, server.ServerName)
		}
		if name == "push" || strings.Contains(name, "push") || strings.Contains(name, "rebase") {
			t.Errorf("sharp git tool registered: %s", name)
		}
		delete(want, name)
	}
	for missing := range want {
		t.Errorf("missing tool %q", missing)
	}
	if _, err := ToolNames("write"); err == nil {
		t.Fatal("expected unknown tier to fail")
	}
}

func TestDescriptions(t *testing.T) {
	t.Parallel()
	s := server.New()
	root := t.TempDir()
	gitInit(t, root)
	if _, err := Register(s, root, TierCore); err != nil {
		t.Fatal(err)
	}
	got := listTools(t, s)
	want := map[string]string{
		ToolStatus: descStatus, ToolDiff: descDiff, ToolCommits: descCommits,
		ToolStage: descStage, ToolCommit: descCommit,
	}
	if len(got) != len(want) {
		t.Fatalf("registered %#v", got)
	}
	for name, prefix := range want {
		if !strings.HasPrefix(got[name], prefix) {
			t.Errorf("%s description %q", name, got[name])
		}
	}
}

func TestNotARepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	if err := WorkTree(context.Background(), t.TempDir()); err == nil {
		t.Fatal("non-repo was accepted")
	}
}

func TestStatusDiffStageCommit(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")

	ctx := context.Background()
	if err := WorkTree(ctx, root); err != nil {
		t.Fatal(err)
	}
	log, err := Commits(ctx, root, 0)
	if err != nil {
		t.Fatal(err)
	}
	if log != "no commits\n" {
		t.Fatalf("log = %q", log)
	}
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := Status(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, "note.txt") || !strings.Contains(status, "branch: main") {
		t.Fatalf("status = %q", status)
	}
	staged, err := Stage(ctx, root, []string{"note.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(staged, "note.txt") {
		t.Fatalf("stage = %q", staged)
	}
	if _, err := Diff(ctx, root, "--all", ""); err == nil {
		t.Fatal("revision flag was accepted")
	}
	committed, err := Commit(ctx, root, "add note")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(committed, "committed: ") {
		t.Fatalf("commit = %q", committed)
	}
	clean, err := Status(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(clean, "clean") {
		t.Fatalf("status after commit = %q", clean)
	}
	listed, err := Commits(ctx, root, 5)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(listed, "add note") {
		t.Fatalf("commits = %q", listed)
	}
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	diff, err := Diff(ctx, root, "", "note.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "+world") {
		t.Fatalf("diff = %q", diff)
	}
	shown, err := Diff(ctx, root, "HEAD", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(shown, "hello") {
		t.Fatalf("show = %q", shown)
	}
	if _, err := Stage(ctx, root, []string{"../outside"}); err == nil {
		t.Fatal("path escape was staged")
	}
}

func TestCleanRootAndEmptyRepo(t *testing.T) {
	if _, err := CleanRoot(""); err == nil {
		t.Fatal("empty root accepted")
	}
	if _, err := CleanRoot(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing root accepted")
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CleanRoot(file); err == nil {
		t.Fatal("file root accepted")
	}
	root := t.TempDir()
	got, err := CleanRoot(root)
	if err != nil || got == "" {
		t.Fatalf("CleanRoot: %v %q", err, got)
	}

	gitInit(t, root)
	ctx := context.Background()
	diff, err := Diff(ctx, root, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "clean") && diff == "" {
		t.Fatalf("empty repo diff = %q", diff)
	}
	if _, err := Diff(ctx, root, "HEAD:file", ""); err == nil {
		t.Fatal("revision with colon accepted")
	}
	if _, err := Commit(ctx, root, "   "); err == nil {
		t.Fatal("blank message accepted")
	}
	if _, err := Stage(ctx, root, nil); err == nil {
		t.Fatal("empty stage accepted")
	}
}

func TestCommitErrorAndClip(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
	ctx := context.Background()
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Stage(ctx, root, []string{"note.txt"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Commit(ctx, root, "add note"); err != nil {
		t.Fatal(err)
	}
	if _, err := Commit(ctx, root, "nothing staged"); err == nil {
		t.Fatal("empty commit succeeded")
	}
	clean, err := Diff(ctx, root, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(clean, "clean working tree") {
		t.Fatalf("clean diff = %q", clean)
	}
	big := strings.Repeat("line\n", 2000)
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	clipped, err := Diff(ctx, root, "", "note.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(clipped, "truncated") {
		t.Fatalf("diff was not clipped (%d bytes)", len(clipped))
	}
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret")
	if err := os.WriteFile(secret, []byte("no"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := Diff(ctx, root, "", "link"); err == nil {
		t.Fatal("symlink escape was diffed")
	}
	listed, err := Commits(ctx, root, 1000)
	if err != nil || !strings.Contains(listed, "add note") {
		t.Fatalf("commits = %q %v", listed, err)
	}
}

func TestToolHandlers(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)
	s := server.New()
	if _, err := Register(s, root, TierCore); err != nil {
		t.Fatal(err)
	}
	got := callTool(t, s, ToolStatus, map[string]any{})
	if !strings.Contains(got, "branch:") {
		t.Fatalf("status call = %q", got)
	}
	errText := callTool(t, s, ToolCommit, map[string]any{"message": " "})
	if !strings.Contains(errText, "message") {
		t.Fatalf("commit call = %q", errText)
	}
	errText = callTool(t, s, ToolStage, map[string]any{"paths": []any{}})
	if !strings.Contains(errText, "paths") {
		t.Fatalf("stage call = %q", errText)
	}
}

func callTool(t *testing.T, s *mcpserver.MCPServer, name string, args map[string]any) string {
	t.Helper()
	params := map[string]any{"name": name, "arguments": args}
	msg := map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": params}
	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	resp := s.HandleMessage(context.Background(), raw)
	switch r := resp.(type) {
	case mcp.JSONRPCResponse:
		result, ok := r.Result.(*mcp.CallToolResult)
		if !ok {
			t.Fatalf("result %T", r.Result)
		}
		if len(result.Content) == 0 {
			return ""
		}
		text, ok := result.Content[0].(mcp.TextContent)
		if !ok {
			t.Fatalf("content %T", result.Content[0])
		}
		return text.Text
	case mcp.JSONRPCError:
		t.Fatalf("protocol error %d: %s", r.Error.Code, r.Error.Message)
	default:
		t.Fatalf("response %T", resp)
	}
	return ""
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	cmd := exec.Command("git", "init", "-b", "main", dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
}

func listTools(t *testing.T, s *mcpserver.MCPServer) map[string]string {
	t.Helper()
	resp := s.HandleMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	result, ok := resp.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", resp)
	}
	listResult, ok := result.Result.(mcp.ListToolsResult)
	if !ok {
		t.Fatalf("expected ListToolsResult, got %T", result.Result)
	}
	names := make(map[string]string, len(listResult.Tools))
	for _, tool := range listResult.Tools {
		names[tool.Name] = tool.Description
	}
	return names
}
