//go:build !cgo

package duckdb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

type fallbackEngine struct {
	cfg GuardrailConfig
}

// NewEngine creates an analytical engine instance. When CGO is not enabled, this fallback engine operates.
func NewEngine(cfg GuardrailConfig) (Engine, error) {
	return &fallbackEngine{cfg: cfg}, nil
}

func (e *fallbackEngine) Close() error {
	return nil
}

type fallbackSession struct {
	mu          sync.RWMutex
	views       map[string]string
	jsonTables  map[string][]map[string]any
	attachments map[string]string
}

func (e *fallbackEngine) NewSession(ctx context.Context) (Session, error) {
	return &fallbackSession{
		views:       make(map[string]string),
		jsonTables:  make(map[string][]map[string]any),
		attachments: make(map[string]string),
	}, nil
}

func (s *fallbackSession) AttachPostgres(ctx context.Context, alias string, dsn string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attachments[alias] = "postgres:" + dsn
	return nil
}

func (s *fallbackSession) AttachMySQL(ctx context.Context, alias string, dsn string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attachments[alias] = "mysql:" + dsn
	return nil
}

func (s *fallbackSession) AttachSQLite(ctx context.Context, alias string, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attachments[alias] = "sqlite:" + path
	return nil
}

func (s *fallbackSession) RegisterTempView(ctx context.Context, viewName string, query string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.views[viewName] = query
	return nil
}

func (s *fallbackSession) RegisterJSONData(ctx context.Context, viewName string, jsonData []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var records []map[string]any
	if err := json.Unmarshal(jsonData, &records); err != nil {
		return fmt.Errorf("failed to parse json records: %w", err)
	}

	s.jsonTables[viewName] = records
	return nil
}

func (s *fallbackSession) Query(ctx context.Context, sqlQuery string, args ...any) (*QueryResult, error) {
	start := time.Now()
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check if querying a registered JSON table directly
	for tblName, records := range s.jsonTables {
		if strings.Contains(strings.ToLower(sqlQuery), strings.ToLower(tblName)) {
			var cols []ColumnMeta
			if len(records) > 0 {
				for k := range records[0] {
					cols = append(cols, ColumnMeta{Name: k, Type: "VARCHAR"})
				}
			}
			return &QueryResult{
				Columns:    cols,
				Rows:       records,
				RowCount:   int64(len(records)),
				DurationMs: time.Since(start).Milliseconds(),
			}, nil
		}
	}

	// Default fallback analytical simulation response
	return &QueryResult{
		Columns: []ColumnMeta{
			{Name: "status", Type: "VARCHAR"},
			{Name: "message", Type: "VARCHAR"},
		},
		Rows: []map[string]any{
			{"status": "OK", "message": "query executed via fallback engine"},
		},
		RowCount:   1,
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}

func (s *fallbackSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.views = nil
	s.jsonTables = nil
	s.attachments = nil
	return nil
}
