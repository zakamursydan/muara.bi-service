package worker

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/hibiken/asynq"
	"muara.bi/core/internal/service"
	"muara.bi/core/pkg/logging"
)

// AlertWorker evaluates threshold queries and dispatches notifications
type AlertWorker struct {
	db         *sql.DB
	preparer   *service.SessionPreparer
	httpClient *http.Client
	logger     *logging.Logger
}

func NewAlertWorker(db *sql.DB, preparer *service.SessionPreparer, logger *logging.Logger) *AlertWorker {
	return &AlertWorker{
		db:         db,
		preparer:   preparer,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     logger,
	}
}

func (w *AlertWorker) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload AlertCheckPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal alert payload: %w", err)
	}

	w.logger.Info(fmt.Sprintf("evaluating alert: id=%s title=%q", payload.AlertID, payload.Title))

	// 1. Execute the guardrailed aggregate query against the tenant session
	result, err := w.preparer.Execute(ctx, service.PreparedQuery{
		WorkspaceID: payload.WorkspaceID,
		ProjectID:   payload.ProjectID,
		EnvTag:      payload.EnvTag,
		RawSQL:      payload.RawSQL,
		MaxRows:     1,
	})
	if err != nil {
		return fmt.Errorf("failed to execute alert query: %w", err)
	}

	if len(result.Rows) == 0 {
		w.logger.Info("alert query returned 0 rows, skipping condition check")
		return nil
	}

	// 3. Extract single numeric metric from first row first column
	val, err := extractNumericValue(result.Rows[0])
	if err != nil {
		return fmt.Errorf("cannot extract numeric value from alert query: %w", err)
	}

	// 4. Compare with threshold
	triggered := evaluateCondition(val, payload.Operator, payload.ThresholdValue)
	if !triggered {
		w.logger.Info(fmt.Sprintf("alert not triggered (metric=%f %s threshold=%f)", val, payload.Operator, payload.ThresholdValue))
		return nil
	}

	w.logger.Warn(fmt.Sprintf("ALERT TRIGGERED! (metric=%f %s threshold=%f)", val, payload.Operator, payload.ThresholdValue))

	// 5. Dispatch notification
	if err := w.dispatchNotification(ctx, payload, val); err != nil {
		w.logger.Error(fmt.Sprintf("failed to dispatch alert: %v", err))
		return err
	}

	// 6. Update last_triggered_at timestamp
	if payload.AlertID != "" && w.db != nil {
		_, _ = w.db.ExecContext(ctx, "UPDATE dashboard_alerts SET last_triggered_at = NOW() WHERE id = $1", payload.AlertID)
	}

	return nil
}

func extractNumericValue(row map[string]any) (float64, error) {
	for _, v := range row {
		switch num := v.(type) {
		case float64:
			return num, nil
		case float32:
			return float64(num), nil
		case int:
			return float64(num), nil
		case int64:
			return float64(num), nil
		case string:
			f, err := strconv.ParseFloat(num, 64)
			if err == nil {
				return f, nil
			}
		}
	}
	return 0, fmt.Errorf("no numeric column found in row: %v", row)
}

func evaluateCondition(val float64, op string, threshold float64) bool {
	switch op {
	case ">":
		return val > threshold
	case "<":
		return val < threshold
	case ">=":
		return val >= threshold
	case "<=":
		return val <= threshold
	case "==":
		return val == threshold
	case "!=":
		return val != threshold
	default:
		return val > threshold
	}
}

func (w *AlertWorker) dispatchNotification(ctx context.Context, payload AlertCheckPayload, metricVal float64) error {
	message := fmt.Sprintf("🚨 [muara.bi Alert Triggered]\nTitle: %s\nMetric: %f %s Threshold: %f\nEnvironment: %s\nTime: %s",
		payload.Title, metricVal, payload.Operator, payload.ThresholdValue, payload.EnvTag, time.Now().Format(time.RFC3339))

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
				"alert_id":     payload.AlertID,
				"title":        payload.Title,
				"metric_value": metricVal,
				"operator":     payload.Operator,
				"threshold":    payload.ThresholdValue,
				"env_tag":      payload.EnvTag,
				"triggered_at": time.Now().UTC().Format(time.RFC3339),
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
		w.logger.Info(fmt.Sprintf("Alert notification logged (no external dispatcher for channel=%s): %s", payload.Channel, message))
	}

	return nil
}
