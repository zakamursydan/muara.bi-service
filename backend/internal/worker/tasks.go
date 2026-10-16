package worker

import (
	"github.com/hibiken/asynq"
	"muara.bi/core/internal/tasks"
)

// Queue definitions re-exported from shared tasks package
const (
	QueueCritical = tasks.QueueCritical
	QueueDefault  = tasks.QueueDefault
	QueueHeavy    = tasks.QueueHeavy

	TypeQueryExecute   = tasks.TypeQueryExecute
	TypeExportJob      = tasks.TypeExportJob
	TypeAlertCheck     = tasks.TypeAlertCheck
	TypeDigestDispatch = tasks.TypeDigestDispatch
)

type (
	QueryExecutePayload   = tasks.QueryExecutePayload
	ExportSheetSpec       = tasks.ExportSheetSpec
	ExportJobPayload      = tasks.ExportJobPayload
	AlertCheckPayload     = tasks.AlertCheckPayload
	DigestDispatchPayload = tasks.DigestDispatchPayload
)

func NewQueryExecuteTask(payload QueryExecutePayload) (*asynq.Task, error) {
	return tasks.NewQueryExecuteTask(payload)
}

func NewExportJobTask(payload ExportJobPayload) (*asynq.Task, error) {
	return tasks.NewExportJobTask(payload)
}

func NewAlertCheckTask(payload AlertCheckPayload) (*asynq.Task, error) {
	return tasks.NewAlertCheckTask(payload)
}

func NewDigestDispatchTask(payload DigestDispatchPayload) (*asynq.Task, error) {
	return tasks.NewDigestDispatchTask(payload)
}
