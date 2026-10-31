//go:build !cgo

package duckdb

import "context"

// This build has no analytical engine at all. DuckDB is a C library, so the only
// honest behaviour here is to fail: a stub that returned rows would let a
// CGO-less release build report fabricated numbers as real results.
// The types remain declared so the package still compiles (and `go vet`, lint
// and editor tooling work) on machines without a C toolchain.

type disabledEngine struct{}

// NewEngine always fails without CGO.
func NewEngine(cfg GuardrailConfig) (Engine, error) {
	return nil, ErrCGODisabled
}

func (disabledEngine) NewSession(ctx context.Context) (Session, error) { return nil, ErrCGODisabled }
func (disabledEngine) Close() error                                    { return ErrCGODisabled }

type disabledSession struct{}

func (disabledSession) AttachPostgres(ctx context.Context, alias string, dsn string) error {
	return ErrCGODisabled
}

func (disabledSession) AttachMySQL(ctx context.Context, alias string, dsn string) error {
	return ErrCGODisabled
}

func (disabledSession) AttachSQLite(ctx context.Context, alias string, path string) error {
	return ErrCGODisabled
}

func (disabledSession) RegisterTempView(ctx context.Context, viewName string, query string) error {
	return ErrCGODisabled
}

func (disabledSession) RegisterJSONData(ctx context.Context, viewName string, jsonData []byte) error {
	return ErrCGODisabled
}

func (disabledSession) Query(ctx context.Context, sqlQuery string, opts QueryOptions, args ...any) (*QueryResult, error) {
	return nil, ErrCGODisabled
}

func (disabledSession) Close() error { return ErrCGODisabled }

var (
	_ Engine  = disabledEngine{}
	_ Session = disabledSession{}
)
