package agent

import (
	"os"
	"path/filepath"
	"strings"
)

// ExpandAndResolve returns absolute cleaned path with tilde expansion and resolved symlinks.
func ExpandAndResolve(p string) (string, error) {
	if p == "" {
		return "", nil
	}
	expanded := p
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		expanded = filepath.Join(home, p[1:])
	}
	abs, err := filepath.Abs(expanded)
	if err != nil {
		abs = filepath.Clean(expanded)
	}
	if realPath, err := filepath.EvalSymlinks(abs); err == nil {
		return filepath.Clean(realPath), nil
	}
	return filepath.Clean(abs), nil
}

// NormalizePath cleans and resolves symlinks if possible for comparison.
func NormalizePath(p string) string {
	res, err := ExpandAndResolve(p)
	if err != nil {
		return filepath.Clean(p)
	}
	return res
}

// PathsMatch checks if two directory paths refer to the same folder.
func PathsMatch(p1, p2 string) bool {
	return NormalizePath(p1) == NormalizePath(p2)
}

// PathVariants returns possible variants of a path (resolved realpath, non-symlink abs, etc.)
// to reliably catch stored references regardless of how the agent recorded it.
func PathVariants(p string) []string {
	var variants []string
	seen := make(map[string]bool)

	add := func(s string) {
		s = filepath.Clean(s)
		if s != "" && !seen[s] {
			seen[s] = true
			variants = append(variants, s)
		}
	}

	add(p)
	if strings.HasPrefix(p, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			add(filepath.Join(home, p[1:]))
		}
	}
	if abs, err := filepath.Abs(p); err == nil {
		add(abs)
	}
	if realPath, err := filepath.EvalSymlinks(p); err == nil {
		add(realPath)
	}

	return variants
}
