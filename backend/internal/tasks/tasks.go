package tasks

import (
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
)

// Queue definitions with priority weights
const (
	QueueCritical = "critical" // Weight: 6 (Interactive alerts, SLA critical)
	QueueDefault  = "default"  // Weight: 3 (Standard queries, scheduled reports)
	QueueHeavy    = "heavy"    // Weight: 1 (Multi-sheet XLSX exports, big data aggregations)
)

// Task type names
const (
	TypeQueryExecute   = "query:execute"
	TypeExportJob      = "export:generate"
	TypeAlertCheck     = "alert:check"
	TypeDigestDispatch = "digest:dispatch"
)

// QueryExecutePayload holds parameters for async heavy query execution.
type QueryExecutePayload struct {
	WorkspaceID    string         `json:"workspace_id"`
	ProjectID      string         `json:"project_id"`
	UserID         string         `json:"user_id"`
	EnvTag         string         `json:"env_tag"`
	RawSQL         string         `json:"raw_sql"`
	Parameters     map[string]any `json:"parameters"`
	UserAttributes map[string]any `json:"user_attributes"`
	MaxRows        int            `json:"max_rows"`
	CacheTTL       time.Duration  `json:"cache_ttl"`
	ForceRefresh   bool           `json:"force_refresh"`
}

// ExportSheetSpec is one query whose result becomes one exported sheet.
type ExportSheetSpec struct {
	Name       string         `json:"name"`
	RawSQL     string         `json:"raw_sql"`
	Parameters map[string]any `json:"parameters"`
}

// ExportJobPayload holds parameters for streaming export to CSV, JSON, or XLSX.
type ExportJobPayload struct {
	ExportJobID    string            `json:"export_job_id"`
	WorkspaceID    string            `json:"workspace_id"`
	ProjectID      string            `json:"project_id"`
	UserID         string            `json:"user_id"`
	UserAttributes map[string]any    `json:"user_attributes"`
	EnvTag         string            `json:"env_tag"`
	Format         string            `json:"format"`
	Sheets         []ExportSheetSpec `json:"sheets"`
	OutputFile     string            `json:"output_file"`
}

// AlertCheckPayload holds criteria for threshold condition evaluation
type AlertCheckPayload struct {
	AlertID        string  `json:"alert_id"`
	WorkspaceID    string  `json:"workspace_id"`
	ProjectID      string  `json:"project_id"`
	DashboardID    string  `json:"dashboard_id"`
	WidgetID       string  `json:"widget_id"`
	Title          string  `json:"title"`
	RawSQL         string  `json:"raw_sql"`
	Operator       string  `json:"operator"`
	ThresholdValue float64 `json:"threshold_value"`
	Channel        string  `json:"channel"`
	RecipientJSON  string  `json:"recipient_json"`
	EnvTag         string  `json:"env_tag"`
}

// DigestDispatchPayload holds criteria for a scheduled dashboard digest
type DigestDispatchPayload struct {
	AlertID       string `json:"alert_id"`
	WorkspaceID   string `json:"workspace_id"`
	ProjectID     string `json:"project_id"`
	DashboardID   string `json:"dashboard_id"`
	Title         string `json:"title"`
	Channel       string `json:"channel"`
	RecipientJSON string `json:"recipient_json"`
	EnvTag        string `json:"env_tag"`
}

// NewQueryExecuteTask creates a new Asynq Task for query execution
func NewQueryExecuteTask(payload QueryExecutePayload) (*asynq.Task, error) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeQueryExecute, bytes, asynq.Queue(QueueDefault), asynq.MaxRetry(3)), nil
}

// NewExportJobTask creates a new Asynq Task for export generation
func NewExportJobTask(payload ExportJobPayload) (*asynq.Task, error) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeExportJob, bytes, asynq.Queue(QueueHeavy), asynq.MaxRetry(2), asynq.Timeout(5*time.Minute)), nil
}

// NewAlertCheckTask creates a new Asynq Task for threshold alert evaluation
func NewAlertCheckTask(payload AlertCheckPayload) (*asynq.Task, error) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeAlertCheck, bytes, asynq.Queue(QueueCritical), asynq.MaxRetry(3)), nil
}

// NewDigestDispatchTask creates a new Asynq Task for scheduled digest delivery
func NewDigestDispatchTask(payload DigestDispatchPayload) (*asynq.Task, error) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeDigestDispatch, bytes, asynq.Queue(QueueDefault), asynq.MaxRetry(3), asynq.Timeout(5*time.Minute)), nil
}
