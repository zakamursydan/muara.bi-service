//go:build cgo

package duckdb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every rule in filesource_path.go exists because of something DuckDB does with
// a path string. These tests measure it on the real reader, so a future DuckDB
// upgrade that changes the behaviour fails here instead of silently reading the
// wrong file on a user's machine.

func writeIn(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func countRows(t *testing.T, session Session, ctx context.Context, view string) int64 {
	t.Helper()
	res, err := session.Query(ctx, fmt.Sprintf("SELECT count(*) AS n FROM %s", view), QueryOptions{})
	if err != nil {
		t.Fatalf("count %s: %v", view, err)
	}
	n, ok := res.Rows[0]["n"].(int64)
	if !ok {
		t.Fatalf("count %s returned %#v", view, res.Rows[0]["n"])
	}
	return n
}

// A relative literal is resolved against the working directory — not once when
// the view is created, but again on every read. Two folders each holding
// orders.csv prove it: the same view answers 1 row and then 4 rows with nothing
// but a chdir in between. This is why a definition must never reach DuckDB as a
// relative path.
func TestRelativeFileSourcePathFollowsTheWorkingDirectory(t *testing.T) {
	bundle := t.TempDir()
	elsewhere := t.TempDir()
	writeIn(t, bundle, "orders.csv", "a\n1\n2\n3\n4\n")
	writeIn(t, elsewhere, "orders.csv", "a\n99\n")

	session := openSession(t, DefaultGuardrailConfig())
	ctx := context.Background()

	t.Chdir(elsewhere)
	if err := session.RegisterFileSource(ctx, "orders", "orders.csv"); err != nil {
		t.Fatalf("register a relative path: %v", err)
	}
	if got := countRows(t, session, ctx, "orders"); got != 1 {
		t.Fatalf("read from cwd=elsewhere: %d rows, want the 1-row file there", got)
	}

	t.Chdir(bundle)
	if got := countRows(t, session, ctx, "orders"); got != 4 {
		t.Fatalf("the same view after chdir: %d rows, want 4 — the relative literal followed the working directory", got)
	}
}

// The rule as it will be used: resolve to an absolute path first, and the file
// behind a definition stops depending on where the process happened to start.
func TestResolvedAbsolutePathReadsTheSameFileFromAnyDirectory(t *testing.T) {
	bundle := t.TempDir()
	source := writeIn(t, bundle, "orders.csv", "a\n1\n2\n3\n")

	session := openSession(t, DefaultGuardrailConfig())
	ctx := context.Background()

	// What a bundle stores, and what the desktop will rebuild at open time.
	stored, err := CanonicalFileSourcePath(source, bundle)
	if err != nil {
		t.Fatalf("canonicalise: %v", err)
	}
	resolved, err := ResolveFileSourcePath(stored, bundle)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	for _, dir := range []string{t.TempDir(), filepath.Dir(bundle)} {
		t.Chdir(dir)
		if err := session.RegisterFileSource(ctx, "orders", resolved); err != nil {
			t.Fatalf("register %s from %s: %v", resolved, dir, err)
		}
		if got := countRows(t, session, ctx, "orders"); got != 3 {
			t.Errorf("from %s: %d rows, want 3", dir, got)
		}
	}
}

// The three readers glob their argument, so a stored path with a star is not a
// typo — it is a second file nobody asked for. Measured here through SQL
// because FileSourceQuery now refuses to build it at all.
func TestFileSourceRefusesGlobPatterns(t *testing.T) {
	dir := t.TempDir()
	writeIn(t, dir, "x1.csv", "a\n1\n")
	writeIn(t, dir, "x2.csv", "a\n1\n2\n")

	session := openSession(t, DefaultGuardrailConfig())
	ctx := context.Background()

	pattern := filepath.ToSlash(filepath.Join(dir, "x*.csv"))
	res, err := session.Query(ctx, "SELECT count(*) AS n FROM read_csv_auto('"+strings.ReplaceAll(pattern, "'", "''")+"')", QueryOptions{})
	if err != nil {
		t.Fatalf("the glob read itself: %v", err)
	}
	if res.Rows[0]["n"] != int64(3) {
		t.Fatalf("glob read returned %#v, want 3 rows from two files", res.Rows[0]["n"])
	}

	for _, path := range []string{pattern, filepath.ToSlash(filepath.Join(dir, "x?.csv"))} {
		if _, err := FileSourceQuery(path); !errors.Is(err, ErrFileSourceGlobPattern) {
			t.Errorf("%q returned %v, want ErrFileSourceGlobPattern", path, err)
		}
		if err := session.RegisterFileSource(ctx, "globbed", path); !errors.Is(err, ErrFileSourceGlobPattern) {
			t.Errorf("RegisterFileSource(%q) returned %v, want ErrFileSourceGlobPattern", path, err)
		}
	}

	// A bracket is a character class, so even a real filename can be a pattern:
	// "log[1].csv" reads log1.csv, and the literal name only matches by luck.
	if _, err := FileSourceQuery(filepath.ToSlash(filepath.Join(dir, "log[1].csv"))); !errors.Is(err, ErrFileSourceGlobPattern) {
		t.Errorf("bracket path returned %v, want ErrFileSourceGlobPattern", err)
	}
}

// DuckDB does not expand "~": it searches for the literal pattern, so the error
// names a path with a tilde in it. Nothing between a definition and the reader
// is a shell.
func TestFileSourceDoesNotExpandTilde(t *testing.T) {
	session := openSession(t, DefaultGuardrailConfig())
	ctx := context.Background()

	tilde := "~/muara-no-such-file-9c1a.csv"
	err := session.RegisterFileSource(ctx, "tilde", tilde)
	if err == nil {
		t.Fatal("a tilde path was registered")
	}
	if !strings.Contains(err.Error(), tilde) {
		t.Errorf("DuckDB searched something other than the literal: %v", err)
	}
	if _, err := ResolveFileSourcePath(tilde, t.TempDir()); !errors.Is(err, ErrFileSourceHomeRelative) {
		t.Errorf("resolve %q returned %v, want ErrFileSourceHomeRelative", tilde, err)
	}
}
