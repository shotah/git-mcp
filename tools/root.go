package tools

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CleanRoot resolves --root to a real directory. The returned path is the jail.
func CleanRoot(root string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", errors.New("--root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("root: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("root is not a directory")
	}
	return resolved, nil
}

// Resolve maps p into root. Symlinks are followed and must stay inside root.
func Resolve(root, p string) (string, error) {
	if strings.ContainsRune(p, 0) {
		return "", errors.New("invalid path")
	}
	p = strings.TrimSpace(p)
	if p == "" {
		p = "."
	}
	var lexical string
	if filepath.IsAbs(p) {
		lexical = filepath.Clean(p)
	} else {
		lexical = filepath.Clean(filepath.Join(root, p))
	}
	if err := inside(root, lexical); err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, lexical)
	if err != nil {
		return "", err
	}
	if rel == "." {
		return root, nil
	}
	cur := root
	for part := range strings.SplitSeq(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return "", errors.New("path escapes workspace")
		}
		next := filepath.Join(cur, part)
		fi, err := os.Lstat(next)
		if err != nil {
			return "", err
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(next)
			if err != nil {
				return "", err
			}
			if err := inside(root, resolved); err != nil {
				return "", err
			}
			cur = resolved
			continue
		}
		cur = next
	}
	if err := inside(root, cur); err != nil {
		return "", err
	}
	return cur, nil
}

func inside(root, path string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return errors.New("path escapes workspace")
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("path escapes workspace")
	}
	return nil
}

// Rel returns p relative to root, using forward slashes.
func Rel(root, abs string) (string, error) {
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", err
	}
	if err := inside(root, abs); err != nil {
		return "", err
	}
	if strings.ContainsAny(rel, "\r\n") {
		return "", errors.New("invalid path")
	}
	return filepath.ToSlash(rel), nil
}
