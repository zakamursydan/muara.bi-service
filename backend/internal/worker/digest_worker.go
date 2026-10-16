package worker

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/hibiken/asynq"
	"muara.bi/core/internal/duckdb"
	"muara.bi/core/internal/exporter"
	"muara.bi/core/internal/repository/pgsql"
	"muara.bi/core/internal/service"
	"muara.bi/core/pkg/logging"
)

// DigestWorker renders a multi-sheet spreadsheet report and dispatches it
type DigestWorker struct {
	db            *sql.DB
	dashboardRepo pgsql.DashboardRepositoryProvider
	widgetRepo    pgsql.WidgetRepositoryProvider
	preparer      *service.SessionPreparer
	httpClient    *http.Client
	logger        *logging.Logger
}

func NewDigestWorker(
	db *sql.DB,
	dashboardRepo pgsql.DashboardRepositoryProvider,
	widgetRepo pgsql.WidgetRepositoryProvider,
	preparer *service.SessionPreparer,
	logger *logging.Logger,
) *DigestWorker {
	return &DigestWorker{
		db:            db,
		dashboardRepo: dashboardRepo,
		widgetRepo:    widgetRepo,
		preparer:      preparer,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		logger:        logger,
	}
}

func (w *DigestWorker) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload DigestDispatchPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal digest payload: %w", err)
	}

	w.logger.Info(fmt.Sprintf("generating scheduled digest for dashboard=%s title=%q channel=%s",
		payload.DashboardID, payload.Title, payload.Channel))

	// 1. Fetch dashboard
	dash, err := w.dashboardRepo.GetDashboardByID(ctx, payload.WorkspaceID, payload.DashboardID)
	if err != nil {
		return fmt.Errorf("failed to load dashboard for digest: %w", err)
	}
	if dash == nil {
		return fmt.Errorf("dashboard %s not found", payload.DashboardID)
	}

	// 2. Fetch widgets
	widgets, err := w.widgetRepo.ListWidgetsByDashboard(ctx, payload.DashboardID)
	if err != nil {
		return fmt.Errorf("failed to load widgets for digest: %w", err)
	}

	// 3. Execute all widgets to produce sheets
	var sheets []exporter.Sheet
	var widgetSummaries []string

	for _, widget := range widgets {
		res, err := w.preparer.Execute(ctx, service.PreparedQuery{
			WorkspaceID: payload.WorkspaceID,
			ProjectID:   payload.ProjectID,
			EnvTag:      payload.EnvTag,
			RawSQL:      widget.RawSQL,
			MaxRows:     10000,
		})
		if err != nil {
			w.logger.Warn(fmt.Sprintf("digest widget %s failed: %v", widget.ID, err))
			continue
		}

		sheets = append(sheets, exporter.NewSheet(widget.Title, res))
		widgetSummaries = append(widgetSummaries, fmt.Sprintf("• %s: %d rows", widget.Title, res.RowCount))
	}

	// If no sheets could be run, create a placeholder sheet
	if len(sheets) == 0 {
		sheets = append(sheets, exporter.NewSheet("Summary", &duckdb.QueryResult{
			Columns: []duckdb.ColumnMeta{
				{Name: "Dashboard", Type: "VARCHAR"},
				{Name: "GeneratedAt", Type: "VARCHAR"},
			},
			Rows: []map[string]any{
				{"Dashboard": dash.Title, "GeneratedAt": time.Now().Format(time.RFC3339)},
			},
			RowCount: 1,
		}))
	}

	// 4. Generate multi-sheet XLSX report
	tmpDir := os.TempDir()
	outPath := filepath.Join(tmpDir, fmt.Sprintf("digest_%s_%d.xlsx", payload.DashboardID, time.Now().Unix()))
	defer os.Remove(outPath)

	if _, err := exporter.WriteFile(outPath, exporter.FormatXLSX, sheets); err != nil {
		return fmt.Errorf("failed to export multi-sheet digest xlsx: %w", err)
	}

	// 5. Dispatch notification
	if err := w.dispatch(ctx, payload, dash.Title, widgetSummaries, outPath); err != nil {
		w.logger.Error(fmt.Sprintf("failed to dispatch digest: %v", err))
		return err
	}

	// 6. Update last_triggered_at
	if payload.AlertID != "" && w.db != nil {
		_, _ = w.db.ExecContext(ctx, "UPDATE dashboard_alerts SET last_triggered_at = NOW() WHERE id = $1", payload.AlertID)
	}

	w.logger.Info(fmt.Sprintf("scheduled digest dispatched successfully for dashboard %s", payload.DashboardID))
	return nil
}

func (w *DigestWorker) dispatch(
	ctx context.Context,
	payload DigestDispatchPayload,
	dashboardTitle string,
	summaries []string,
	filePath string,
) error {
	summaryText := "No widgets rendered."
	if len(summaries) > 0 {
		summaryText = fmt.Sprintf("Rendered %d widgets:\n", len(summaries))
		for _, s := range summaries {
			summaryText += s + "\n"
		}
	}

	message := fmt.Sprintf("📊 [muara.bi Scheduled Digest]\nDashboard: %s\nTitle: %s\nEnvironment: %s\nTime: %s\n\n%s",
		dashboardTitle, payload.Title, payload.EnvTag, time.Now().Format("2006-01-02 15:04:05 MST"), summaryText)

	var recipient map[string]any
	_ = json.Unmarshal([]byte(payload.RecipientJSON), &recipient)

	switch payload.Channel {
	case "SLACK":
		webhookURL, _ := recipient["webhook_url"].(string)
		if webhookURL != "" {
			body, _ := json.Marshal(map[string]string{"text": message})
			req, _ := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := w.httpClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
		}
	case "TELEGRAM":
		botToken, _ := recipient["bot_token"].(string)
		chatID, _ := recipient["chat_id"].(string)
		if botToken != "" && chatID != "" {
			apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
			body, _ := json.Marshal(map[string]string{"chat_id": chatID, "text": message})
			req, _ := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := w.httpClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
		}
	case "WEBHOOK":
		webhookURL, _ := recipient["webhook_url"].(string)
		if webhookURL != "" {
			payloadBytes, _ := json.Marshal(map[string]any{
				"alert_id":        payload.AlertID,
				"dashboard_id":    payload.DashboardID,
				"dashboard_title": dashboardTitle,
				"title":           payload.Title,
				"env_tag":         payload.EnvTag,
				"summaries":       summaries,
				"generated_at":    time.Now().UTC().Format(time.RFC3339),
			})
			req, _ := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(payloadBytes))
			req.Header.Set("Content-Type", "application/json")
			resp, err := w.httpClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
		}
	default:
		w.logger.Info(fmt.Sprintf("Digest notification logged (no external webhook for channel=%s): %s", payload.Channel, message))
	}

	return nil
}
