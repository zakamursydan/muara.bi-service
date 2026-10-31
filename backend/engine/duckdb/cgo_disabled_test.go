//go:build !cgo

package duckdb

import (
	"context"
	"errors"
	"testing"
)

// A binary built without CGO must refuse to answer, never simulate an answer.
// This is the regression guard for the stub engine that used to return rows like
// {"status":"OK"} and silently serve them as real analytics.
func TestEngine_WithoutCGORefuses(t *testing.T) {
	engine, err := NewEngine(DefaultGuardrailConfig())
	if !errors.Is(err, ErrCGODisabled) {
		t.Fatalf("expected ErrCGODisabled, got engine=%v err=%v", engine, err)
	}

	if session, err := (disabledEngine{}).NewSession(context.Background()); err == nil || session != nil {
		t.Fatalf("disabled engine must not open sessions, got session=%v err=%v", session, err)
	}

	res, err := disabledSession{}.Query(context.Background(), "SELECT 1", QueryOptions{})
	if !errors.Is(err, ErrCGODisabled) {
		t.Fatalf("expected refusal from stub session, got res=%+v err=%v", res, err)
	}
	if res != nil {
		t.Fatalf("stub session must not return a result set, got %+v", res)
	}
}
