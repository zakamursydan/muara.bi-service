package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/duckdb"
	"muara.bi/core/internal/exporter"
	"muara.bi/core/internal/service"
	"muara.bi/core/pkg/logging"
)

// ExportWorker renders one or more analytical queries into a downloadable file.
type ExportWorker struct {
	db       *sql.DB
	preparer *service.SessionPreparer
	access   service.AccessServiceProvider
	logger   *logging.Logger
}

func NewExportWorker(
	db *sql.DB,
	preparer *service.SessionPreparer,
	access service.AccessServiceProvider,
	logger *logging.Logger,
) *ExportWorker {
	return &ExportWorker{db: db, preparer: preparer, access: access, logger: logger}
}

func (w *ExportWorker) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload ExportJobPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal export payload: %w", err)
	}

	format, err := exporter.NormalizeFormat(payload.Format)
	if err != nil {
		w.markFailed(ctx, payload.ExportJobID, err.Error())
		return err
	}
	if len(payload.Sheets) == 0 {
		err = fmt.Errorf("export job has no sheets to render")
		w.markFailed(ctx, payload.ExportJobID, err.Error())
		return err
	}

	w.logger.Info(fmt.Sprintf("processing export job: id=%s format=%s sheets=%d",
		payload.ExportJobID, format, len(payload.Sheets)))
	w.setStatus(ctx, payload.ExportJobID, "PROCESSING", "", nil)

	// Exports are user-initiated, so the requesting user's row filters apply.
	// Resolving here keeps a VIEWER's download as narrow as what they see.
	access, err := w.access.Resolve(ctx, service.Scope{
		WorkspaceID: payload.WorkspaceID,
		ProjectID:   payload.ProjectID,
		UserID:      payload.UserID,
		Attributes:  payload.UserAttributes,
	})
	if err != nil {
		w.markFailed(ctx, payload.ExportJobID, err.Error())
		return err
	}
	if !access.Can(entity.PermQueryExecute) {
		err = fmt.Errorf("user %s may not export data from project %s", payload.UserID, payload.ProjectID)
		w.markFailed(ctx, payload.ExportJobID, err.Error())
		return err
	}

	sheets, err := w.renderSheets(ctx, payload, access.RLSRules)
	if err != nil {
		w.markFailed(ctx, payload.ExportJobID, err.Error())
		return err
	}

	outPath := payload.OutputFile
	if outPath == "" {
		outPath = filepath.Join(os.TempDir(), fmt.Sprintf("muara_export_%s_%d.%s",
			payload.ExportJobID, time.Now().Unix(), exporter.FileExtension(format)))
	}
	if dir := filepath.Dir(outPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			w.markFailed(ctx, payload.ExportJobID, err.Error())
			return fmt.Errorf("failed to create export directory: %w", err)
		}
	}

	rowCount, err := exporter.WriteFile(outPath, format, sheets)
	if err != nil {
		w.markFailed(ctx, payload.ExportJobID, err.Error())
		return err
	}

	w.setStatus(ctx, payload.ExportJobID, "COMPLETED", outPath, &rowCount)
	w.logger.Info(fmt.Sprintf("export job succeeded: file=%s rows=%d", outPath, rowCount))
	return nil
}

func (w *ExportWorker) renderSheets(ctx context.Context, payload ExportJobPayload, rlsRules []duckdb.RLSRule) ([]exporter.Sheet, error) {
	specs := payload.Sheets
	if !strings.EqualFold(payload.Format, exporter.FormatXLSX) {
		// CSV and JSON are single-sheet formats.
		specs = specs[:1]
	}

	sheets := make([]exporter.Sheet, 0, len(specs))
	for _, spec := range specs {
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			name = "Data"
		}

		result, err := w.preparer.Execute(ctx, service.PreparedQuery{
			WorkspaceID: payload.WorkspaceID,
			ProjectID:   payload.ProjectID,
			EnvTag:      payload.EnvTag,
			RawSQL:      spec.RawSQL,
			Parameters:  spec.Parameters,
			RLSRules:    rlsRules,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to execute export query %q: %w", name, err)
		}
		sheets = append(sheets, exporter.NewSheet(name, result))
	}
	return sheets, nil
}

func (w *ExportWorker) setStatus(ctx context.Context, jobID, status, fileURL string, rowCount *int64) {
	if jobID == "" || w.db == nil {
		return
	}

	if status == "COMPLETED" {
		_, _ = w.db.ExecContext(ctx,
			`UPDATE export_jobs SET status = $1, file_url = $2, row_count = $3 WHERE id = $4`,
			status, fileURL, rowCount, jobID)
		return
	}
	_, _ = w.db.ExecContext(ctx, `UPDATE export_jobs SET status = $1 WHERE id = $2`, status, jobID)
}

func (w *ExportWorker) markFailed(ctx context.Context, jobID, reason string) {
	w.setStatus(ctx, jobID, "FAILED", "", nil)
	w.logger.Error(fmt.Sprintf("export job failed: id=%s reason=%s", jobID, reason))
}
