package duckdb

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// A file source is the one part of a definition that does not survive being
// carried to another machine: the rows live in a path, and a path belongs to a
// filesystem. So the form stored in a bundle is relative to the bundle's own
// folder (BRD §5.5) and the absolute form is rebuilt at open time. Nothing here
// touches DuckDB — the bundle folder is a desktop concept, and the web product
// stores server paths, so both get the same rule from the same code.

// ErrFileSourceHomeRelative is returned for "~/..." paths. DuckDB does not
// expand a tilde — it searches for the literal pattern "~/..." — so storing one
// would fail with an error naming a path the user never wrote, or, worse, join
// onto the bundle folder as if "~" were a directory.
var ErrFileSourceHomeRelative = fmt.Errorf("file source path starts with ~ and no shell expands it here")

// ErrFileSourceOutsideAnchor is returned when a stored path climbs above the
// bundle folder. A bundle received from someone else must not be able to aim
// this app at an arbitrary readable file on this machine.
var ErrFileSourceOutsideAnchor = fmt.Errorf("file source path escapes the bundle folder")

// windowsSourcePath matches "C:\data\orders.csv" and "C:/data/orders.csv". It
// has to be recognised on every platform: filepath.IsAbs answers only for the
// platform it was compiled for, so a bundle authored on Windows would otherwise
// look like a relative path and get joined to the bundle folder.
var windowsSourcePath = regexp.MustCompile(`^[A-Za-z]:[/\\]`)

// isSourcePathAbsolute reports whether path already carries its own root.
func isSourcePathAbsolute(path string) bool {
	return filepath.IsAbs(path) || strings.HasPrefix(path, "/") ||
		strings.HasPrefix(path, `\\`) || windowsSourcePath.MatchString(path)
}

// isHomeRelative reports whether the path leans on a shell to become absolute.
func isHomeRelative(path string) bool {
	return path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`)
}

// ResolveFileSourcePath turns the path stored in a definition into one DuckDB
// can open, against anchorDir — the folder holding the bundle. An absolute path
// is returned cleaned; a relative one is joined and must stay inside the
// anchor.
//
// Relative paths are never resolved against the process working directory.
// DuckDB does exactly that when handed a relative literal, so a definition read
// from another folder would quietly point at a different file — or fail with an
// error naming a path the user never wrote.
//
// An empty anchorDir means "no bundle": only absolute paths are accepted.
func ResolveFileSourcePath(stored, anchorDir string) (string, error) {
	trimmed := strings.TrimSpace(stored)
	if trimmed == "" {
		return "", fmt.Errorf("file source path is empty")
	}
	if isHomeRelative(trimmed) {
		return "", fmt.Errorf("%w: %q", ErrFileSourceHomeRelative, trimmed)
	}
	if isSourcePathAbsolute(trimmed) {
		return filepath.Clean(trimmed), nil
	}
	if anchorDir == "" {
		return "", fmt.Errorf("file source %q is relative but no bundle folder was given", trimmed)
	}

	anchor := filepath.Clean(anchorDir)
	// FromSlash because the canonical stored form always uses forward slashes,
	// on every platform, so a bundle stays readable after crossing OSes.
	joined := filepath.Join(anchor, filepath.FromSlash(trimmed))
	rel, err := filepath.Rel(anchor, joined)
	if err != nil {
		return "", fmt.Errorf("%w: %q", ErrFileSourceOutsideAnchor, trimmed)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q, put the file inside %s or point the source at it again",
			ErrFileSourceOutsideAnchor, trimmed, anchor)
	}
	return joined, nil
}

// CanonicalFileSourcePath is the reverse: the form to keep in a definition. A
// file inside the bundle folder becomes a forward-slash relative path, which is
// what makes the bundle portable. A file outside it stays absolute rather than
// being rewritten into "../" hops — the definition then says plainly that it
// belongs to this machine, and the pre-flight re-pointing step (BRD §5.5) is the
// way to make it portable later.
//
// An empty anchorDir returns the cleaned absolute path.
func CanonicalFileSourcePath(path, anchorDir string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("file source path is empty")
	}
	if isHomeRelative(trimmed) {
		return "", fmt.Errorf("%w: %q", ErrFileSourceHomeRelative, trimmed)
	}
	if anchorDir == "" {
		if !isSourcePathAbsolute(trimmed) {
			return "", fmt.Errorf("file source %q is relative but no bundle folder was given", trimmed)
		}
		return filepath.Clean(trimmed), nil
	}

	absolute, err := ResolveFileSourcePath(trimmed, anchorDir)
	if err != nil {
		return "", err
	}
	anchor := filepath.Clean(anchorDir)
	rel, err := filepath.Rel(anchor, absolute)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(absolute), nil
	}
	return filepath.ToSlash(rel), nil
}
