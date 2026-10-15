package worker

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/xuri/excelize/v2"
	"muara.bi/core/internal/duckdb"
	"muara.bi/core/pkg/logging"
)

// ExportWorker handles CSV, JSON, and multi-sheet Excel generation
type ExportWorker struct {
	db     *sql.DB
	engine duckdb.Engine
	logger *logging.Logger
}

func NewExportWorker(db *sql.DB, engine duckdb.Engine, logger *logging.Logger) *ExportWorker {
	return &ExportWorker{
		db:     db,
		engine: engine,
		logger: logger,
	}
}

func (w *ExportWorker) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload ExportJobPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal export payload: %w", err)
	}

	w.logger.Info(fmt.Sprintf("processing export job: id=%s format=%s", payload.ExportJobID, payload.Format))

	// Mark status in progress if job ID provided
	if payload.ExportJobID != "" && w.db != nil {
		_, _ = w.db.ExecContext(ctx, "UPDATE export_jobs SET status = 'PROCESSING' WHERE id = $1", payload.ExportJobID)
	}

	// 1. Guardrail validation
	if err := duckdb.ValidateQuery(payload.RawSQL); err != nil {
		w.updateJobFailed(ctx, payload.ExportJobID, err.Error())
		return err
	}

	// 2. Transform parameters
	paramResult, err := duckdb.TransformParameters(payload.RawSQL, payload.Parameters)
	if err != nil {
		w.updateJobFailed(ctx, payload.ExportJobID, err.Error())
		return err
	}

	// 3. Execute query
	session, err := w.engine.NewSession(ctx)
	if err != nil {
		w.updateJobFailed(ctx, payload.ExportJobID, err.Error())
		return err
	}
	defer session.Close()

	result, err := session.Query(ctx, paramResult.ParameterizedSQL, paramResult.OrderedArgs...)
	if err != nil {
		w.updateJobFailed(ctx, payload.ExportJobID, err.Error())
		return err
	}

	// 4. Output path
	outPath := payload.OutputFile
	if outPath == "" {
		ext := strings.ToLower(payload.Format)
		if ext == "xlsx" {
			ext = "xlsx"
		}
		outPath = filepath.Join(os.TempDir(), fmt.Sprintf("export_%s_%d.%s", payload.ExportJobID, time.Now().Unix(), ext))
	}

	// Ensure directory exists
	_ = os.MkdirAll(filepath.Dir(outPath), 0755)

	// 5. Generate formatted file
	formatUpper := strings.ToUpper(strings.TrimSpace(payload.Format))
	switch formatUpper {
	case "CSV":
		err = w.exportCSV(outPath, result)
	case "JSON":
		err = w.exportJSON(outPath, result)
	case "XLSX":
		sheetName := payload.SheetTitle
		if sheetName == "" {
			sheetName = "Data"
		}
		err = w.exportXLSX(outPath, sheetName, result)
	default:
		err = fmt.Errorf("unsupported export format: %s", payload.Format)
	}

	if err != nil {
		w.updateJobFailed(ctx, payload.ExportJobID, err.Error())
		return err
	}

	// 6. Update export_jobs record
	if payload.ExportJobID != "" && w.db != nil {
		_, _ = w.db.ExecContext(ctx, `
			UPDATE export_jobs 
			SET status = 'COMPLETED', file_url = $1, row_count = $2 
			WHERE id = $3`,
			outPath, result.RowCount, payload.ExportJobID,
		)
	}

	w.logger.Info(fmt.Sprintf("export job succeeded: file=%s rows=%d", outPath, result.RowCount))
	return nil
}

func (w *ExportWorker) exportCSV(filePath string, result *duckdb.QueryResult) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create csv file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Header
	header := make([]string, len(result.Columns))
	for i, col := range result.Columns {
		header[i] = col.Name
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Rows
	for _, row := range result.Rows {
		record := make([]string, len(result.Columns))
		for i, col := range result.Columns {
			val := row[col.Name]
			if val == nil {
				record[i] = ""
			} else {
				record[i] = fmt.Sprintf("%v", val)
			}
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

func (w *ExportWorker) exportJSON(filePath string, result *duckdb.QueryResult) error {
	bytes, err := json.MarshalIndent(result.Rows, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal json: %w", err)
	}
	return os.WriteFile(filePath, bytes, 0644)
}

func (w *ExportWorker) exportXLSX(filePath string, sheetName string, result *duckdb.QueryResult) error {
	f := excelize.NewFile()
	defer func() {
		_ = f.Close()
	}()

	defaultSheet := f.GetSheetName(0)
	f.SetSheetName(defaultSheet, sheetName)

	// Header styling: bold, light gray background
	style, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "#FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#0891b2"}, Pattern: 1}, // Brand Cyan
	})

	for colIdx, col := range result.Columns {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		_ = f.SetCellValue(sheetName, cell, col.Name)
		if err == nil {
			_ = f.SetCellStyle(sheetName, cell, cell, style)
		}
	}

	for rowIdx, row := range result.Rows {
		for colIdx, col := range result.Columns {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			_ = f.SetCellValue(sheetName, cell, row[col.Name])
		}
	}

	return f.SaveAs(filePath)
}

func (w *ExportWorker) updateJobFailed(ctx context.Context, jobID, reason string) {
	if jobID != "" && w.db != nil {
		_, _ = w.db.ExecContext(ctx, "UPDATE export_jobs SET status = 'FAILED' WHERE id = $1", jobID)
	}
	w.logger.Error(fmt.Sprintf("export job failed: id=%s reason=%s", jobID, reason))
}
