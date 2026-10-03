// Package tools is the local git catalog. The host publishes these as git__status_get.
package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	defaultLog = 20
	maxLog     = 50
	maxDiff    = 8000

	errPathsRequired   = `paths is required, e.g. {"paths":["a.go"]}`
	errMessageRequired = `message is required, e.g. {"message":"fix the greeting"}`
)

// WorkTree reports whether root is a git working tree.
// The process should exit when this fails so the host can skip this server.
func WorkTree(ctx context.Context, root string) error {
	stdout, stderr, err := runGit(ctx, root, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = strings.TrimSpace(stdout)
		}
		if msg == "" {
			return fmt.Errorf("git: %w", err)
		}
		return fmt.Errorf("not a git work tree: %s", msg)
	}
	if strings.TrimSpace(stdout) != "true" {
		return errors.New("not a git work tree")
	}
	return nil
}

// Status is the working tree status.
func Status(ctx context.Context, root string) (string, error) {
	stdout, stderr, err := runGit(ctx, root, "status", "--porcelain=v1", "-b", "-uall")
	if err != nil {
		return "", gitErr(stderr, err)
	}
	return formatStatus(stdout), nil
}

// Diff shows the working tree against HEAD, or the patch of one revision.
func Diff(ctx context.Context, root, revision, path string) (string, error) {
	if err := validRev(revision); err != nil {
		return "", err
	}
	rel, err := diffRel(root, path)
	if err != nil {
		return "", err
	}
	if revision == "" {
		return diffWorkTree(ctx, root, rel)
	}
	args := []string{"show", "--format=", "--patch", "--no-ext-diff", revision}
	if rel != "" {
		args = append(args, "--", rel)
	}
	stdout, stderr, err := runGit(ctx, root, args...)
	if err != nil {
		return "", gitErr(stderr, err)
	}
	return clipFront(emptyDiff(stdout), maxDiff), nil
}

func diffRel(root, path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", nil
	}
	abs, err := Resolve(root, path)
	if err != nil {
		return "", err
	}
	return Rel(root, abs)
}

func diffWorkTree(ctx context.Context, root, rel string) (string, error) {
	args := []string{"diff", "--no-ext-diff", "HEAD"}
	if rel != "" {
		args = append(args, "--", rel)
	}
	stdout, stderr, err := runGit(ctx, root, args...)
	if err != nil && noHead(stderr) {
		// A repo with no commits has no HEAD. Show the index diff instead.
		stdout, stderr, err = runGit(ctx, root, diffArgs(rel)...)
	}
	if err != nil {
		return "", gitErr(stderr, err)
	}
	return clipFront(emptyDiff(stdout), maxDiff), nil
}

func diffArgs(rel string) []string {
	args := []string{"diff", "--no-ext-diff"}
	if rel != "" {
		args = append(args, "--", rel)
	}
	return args
}

// Commits lists recent commits. limit 0 uses the default.
func Commits(ctx context.Context, root string, limit int) (string, error) {
	if limit < 1 {
		limit = defaultLog
	}
	if limit > maxLog {
		limit = maxLog
	}
	stdout, stderr, err := runGit(ctx, root, "log", "-n", strconv.Itoa(limit), "--pretty=format:%H%x09%s")
	if err != nil {
		msg := strings.TrimSpace(stderr)
		if strings.Contains(msg, "does not have any commits") || strings.Contains(stdout, "does not have any commits") {
			return "no commits\n", nil
		}
		return "", gitErr(stderr, err)
	}
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return "no commits\n", nil
	}
	return stdout + "\n", nil
}

// Stage adds paths to the index. It does not commit.
func Stage(ctx context.Context, root string, paths []string) (string, error) {
	if len(paths) == 0 {
		return "", errors.New(errPathsRequired)
	}
	rels := make([]string, 0, len(paths))
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			return "", errors.New(errPathsRequired)
		}
		abs, err := Resolve(root, p)
		if err != nil {
			return "", err
		}
		rel, err := Rel(root, abs)
		if err != nil {
			return "", err
		}
		rels = append(rels, rel)
	}
	args := append([]string{"add", "--"}, rels...)
	_, stderr, err := runGit(ctx, root, args...)
	if err != nil {
		return "", gitErr(stderr, err)
	}
	return withStatus(ctx, root, "staged: "+strings.Join(rels, ", ")), nil
}

// Commit creates a commit from the index. It does not stage and it does not push.
func Commit(ctx context.Context, root, message string) (string, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return "", errors.New(errMessageRequired)
	}
	commitOut, commitErrOut, err := runGit(ctx, root, "commit", "-m", message)
	if err != nil {
		return "", commitFailure(commitOut, commitErrOut, err)
	}
	stdout, stderr, err := runGit(ctx, root, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", gitErr(stderr, err)
	}
	return withStatus(ctx, root, "committed: "+strings.TrimSpace(stdout)), nil
}

// withStatus appends the working tree so a short success line is enough to see what changed.
func withStatus(ctx context.Context, root, line string) string {
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}
	status, err := Status(ctx, root)
	if err != nil {
		return line
	}
	return line + status
}

func runGit(ctx context.Context, root string, args ...string) (string, string, error) {
	argv := append([]string{"-C", root}, args...)
	cmd := exec.CommandContext(ctx, "git", argv...) //nolint:gosec // G204: argv is a fixed subcommand plus a jailed path or a checked revision
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func validRev(revision string) error {
	if revision == "" {
		return nil
	}
	if strings.HasPrefix(revision, "-") || strings.ContainsAny(revision, " \t\r\n:") {
		return errors.New("invalid revision")
	}
	for _, r := range revision {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case strings.ContainsRune("._~^/@{}-", r):
		default:
			return errors.New("invalid revision")
		}
	}
	return nil
}

func formatStatus(out string) string {
	var b strings.Builder
	entries := 0
	for line := range strings.SplitSeq(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		if branch, ok := strings.CutPrefix(line, "## "); ok {
			branch = strings.TrimPrefix(branch, "No commits yet on ")
			if i := strings.Index(branch, "..."); i >= 0 {
				branch = branch[:i]
			}
			if i := strings.Index(branch, " ["); i >= 0 {
				branch = branch[:i]
			}
			fmt.Fprintf(&b, "branch: %s\n", branch)
			continue
		}
		entries++
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if entries == 0 {
		b.WriteString("clean\n")
	}
	return b.String()
}

func emptyDiff(s string) string {
	if strings.TrimSpace(s) == "" {
		return "clean working tree\n"
	}
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s
}

func noHead(stderr string) bool {
	msg := strings.ToLower(stderr)
	return strings.Contains(msg, "bad revision") || strings.Contains(msg, "ambiguous argument")
}

// commitFailure hides git's "use git add" hint. That hint is what sends the model into stage_update.
// Git writes the hint on stdout, so both streams are checked.
func commitFailure(stdout, stderr string, err error) error {
	msg := strings.TrimSpace(stdout + "\n" + stderr)
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "nothing to commit") ||
		strings.Contains(lower, "no changes added to commit") ||
		strings.Contains(lower, "nothing added to commit") {
		return errors.New("nothing to commit")
	}
	if msg == "" {
		return err
	}
	return errors.New(msg)
}

func gitErr(stderr string, err error) error {
	msg := strings.TrimSpace(stderr)
	if msg == "" {
		return err
	}
	return errors.New(msg)
}

func clipFront(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	return string([]rune(s)[:limit]) + "\n…[truncated]\n"
}
