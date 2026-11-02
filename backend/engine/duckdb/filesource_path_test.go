package duckdb

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// The stored form is what travels between machines, so the rules that decide it
// are worth locking: a definition may keep a bundle-relative path, or an
// absolute one it admits belongs to this machine.

func TestCanonicalFileSourcePathKeepsBundleFilesRelative(t *testing.T) {
	anchor := filepath.Join(string(filepath.Separator), "work", "analysis")

	for _, tc := range []struct {
		path string
		want string
	}{
		{filepath.Join(anchor, "orders.csv"), "orders.csv"},
		{filepath.Join(anchor, "data", "2026", "orders.csv"), "data/2026/orders.csv"},
		// Already relative: unchanged, only normalised to slashes.
		{filepath.Join("data", "orders.csv"), "data/orders.csv"},
		// A redundant ./ or .. inside the bundle must not survive into the file.
		{filepath.Join(anchor, ".", "orders.csv"), "orders.csv"},
		{filepath.Join(anchor, "data", "..", "orders.csv"), "orders.csv"},
		// Outside the bundle folder: absolute, not rewritten into ../ hops.
		{filepath.Join(string(filepath.Separator), "work", "shared", "orders.csv"), "/work/shared/orders.csv"},
		{filepath.Join(string(filepath.Separator), "tmp", "orders.csv"), "/tmp/orders.csv"},
	} {
		got, err := CanonicalFileSourcePath(tc.path, anchor)
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		if got != tc.want {
			t.Errorf("CanonicalFileSourcePath(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestCanonicalFileSourcePathRefusesWhatItCannotPlace(t *testing.T) {
	anchor := filepath.Join(string(filepath.Separator), "work", "analysis")

	for _, path := range []string{"", "   "} {
		if _, err := CanonicalFileSourcePath(path, anchor); err == nil {
			t.Errorf("empty path %q accepted", path)
		}
	}
	// Without a bundle folder "relative" has no meaning, so it must not quietly
	// become "relative to the working directory of whoever started the process".
	if _, err := CanonicalFileSourcePath("orders.csv", ""); err == nil {
		t.Error("a relative path was canonicalised with no anchor")
	}
	if got, err := CanonicalFileSourcePath("/data/orders.csv", ""); err != nil || got != "/data/orders.csv" {
		t.Errorf("absolute path with no anchor: got %q, err %v", got, err)
	}
}

func TestResolveFileSourcePath(t *testing.T) {
	anchor := filepath.Join(string(filepath.Separator), "work", "analysis")

	for _, tc := range []struct {
		stored string
		want   string
	}{
		{"orders.csv", filepath.Join(anchor, "orders.csv")},
		{"data/2026/orders.csv", filepath.Join(anchor, "data", "2026", "orders.csv")},
		{"./orders.csv", filepath.Join(anchor, "orders.csv")},
		{"/data/elsewhere.csv", "/data/elsewhere.csv"},
		{"../escape.csv", ""}, // refused, see below
		{"data/../../escape.csv", ""},
	} {
		got, err := ResolveFileSourcePath(tc.stored, anchor)
		if tc.want == "" {
			if !errors.Is(err, ErrFileSourceOutsideAnchor) {
				t.Errorf("%q returned %v, want ErrFileSourceOutsideAnchor", tc.stored, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: %v", tc.stored, err)
		}
		if got != tc.want {
			t.Errorf("ResolveFileSourcePath(%q) = %q, want %q", tc.stored, got, tc.want)
		}
	}

	// A path that climbs and then comes back down is inside the anchor, so it is
	// legal: the rule is containment, not syntax.
	inside, err := ResolveFileSourcePath("data/../orders.csv", anchor)
	if err != nil || inside != filepath.Join(anchor, "orders.csv") {
		t.Errorf("climb back inside: %q, err %v", inside, err)
	}

	// No anchor means no bundle: relative paths have nothing to resolve against.
	if _, err := ResolveFileSourcePath("orders.csv", ""); err == nil ||
		strings.Contains(err.Error(), ErrFileSourceOutsideAnchor.Error()) {
		t.Errorf("relative path with no anchor returned %v, want a missing-anchor error", err)
	}
	if _, err := ResolveFileSourcePath("   ", anchor); err == nil {
		t.Error("blank path accepted")
	}
}

// Bundles move between Windows and macOS machines. A path authored on one is
// read on the other, so "absolute" cannot mean whatever filepath.IsAbs thinks
// about the platform this binary runs on.
func TestResolveFileSourcePathRecognisesBothAbsoluteForms(t *testing.T) {
	anchor := filepath.Join(string(filepath.Separator), "work", "analysis")

	for _, stored := range []string{`C:\Users\analyst\orders.csv`, `C:/Users/analyst/orders.csv`, `\\bastion\share\orders.csv`} {
		got, err := ResolveFileSourcePath(stored, anchor)
		if err != nil {
			t.Fatalf("%s: %v", stored, err)
		}
		if !filepath.IsAbs(got) && got != filepath.Clean(stored) {
			t.Errorf("%s was treated as bundle-relative: %q", stored, got)
		}
		if strings.HasPrefix(got, anchor) {
			t.Errorf("%s was joined onto the bundle folder: %q", stored, got)
		}
	}

	// Backslashes are not separators on Unix, so a hand-edited bundle path keeps
	// its shape there and simply resolves to a file that is not there. That is
	// the honest pre-flight answer, not a rewritten path.
	got, err := ResolveFileSourcePath(`data\orders.csv`, anchor)
	if err != nil {
		t.Fatalf(`data\orders.csv: %v`, err)
	}
	if got != filepath.Join(anchor, `data\orders.csv`) {
		t.Errorf(`backslash path became %q on Unix`, got)
	}
}

// A tilde is expanded by a shell, and there is no shell between a definition
// and the reader, so it is refused rather than stored and mis-read later.
func TestFileSourcePathRefusesTildeAndBlank(t *testing.T) {
	anchor := filepath.Join(string(filepath.Separator), "work", "analysis")

	for _, stored := range []string{"~", "~/orders.csv", `~\orders.csv`, "~/analysis/orders.csv"} {
		if _, err := ResolveFileSourcePath(stored, anchor); !errors.Is(err, ErrFileSourceHomeRelative) {
			t.Errorf("ResolveFileSourcePath(%q) returned %v, want ErrFileSourceHomeRelative", stored, err)
		}
		if _, err := CanonicalFileSourcePath(stored, anchor); !errors.Is(err, ErrFileSourceHomeRelative) {
			t.Errorf("CanonicalFileSourcePath(%q) returned %v, want ErrFileSourceHomeRelative", stored, err)
		}
	}
	// Only a leading "~" segment is a home reference: "tilde~/orders.csv" and
	// "data/~notes.csv" are ordinary names.
	for _, stored := range []string{"tilde~/orders.csv", "data/~notes.csv", "~orders.csv"} {
		if _, err := ResolveFileSourcePath(stored, anchor); err != nil {
			t.Errorf("%q refused: %v", stored, err)
		}
	}
}

// The round trip is what M3 will rely on: what a save writes must be what an
// open rebuilds, including after the bundle folder itself is relocated. Neither
// folder exists here — resolution is lexical, which is what lets pre-flight
// report "path tidak ada" for a file that is missing instead of failing early.
func TestFileSourcePathRoundTripsAcrossARelocatedBundle(t *testing.T) {
	old := filepath.Join(t.TempDir(), "analysis")
	fresh := filepath.Join(t.TempDir(), "analysis")

	stored, err := CanonicalFileSourcePath(filepath.Join(old, "data", "orders.csv"), old)
	if err != nil {
		t.Fatalf("canonicalise: %v", err)
	}
	if stored != "data/orders.csv" {
		t.Fatalf("stored form = %q, want data/orders.csv", stored)
	}

	got, err := ResolveFileSourcePath(stored, fresh)
	if err != nil {
		t.Fatalf("resolve against the moved bundle: %v", err)
	}
	if want := filepath.Join(fresh, "data", "orders.csv"); got != want {
		t.Errorf("resolved to %q, want %q", got, want)
	}
}
