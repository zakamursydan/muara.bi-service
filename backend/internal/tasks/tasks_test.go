package tasks

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hibiken/asynq"
)

// Priority only works if each task type actually lands on the queue the worker
// weights. This enqueues against a live Redis and reads the queue back through
// Asynq's own inspector, so a typo in a queue name or option fails here instead
// of silently flattening priorities in production.
//
//	MUARA_E2E_REDIS=127.0.0.1:6379 go test ./internal/tasks -v
func newTestInspector(t *testing.T) *asynq.Inspector {
	t.Helper()

	addr := os.Getenv("MUARA_E2E_REDIS")
	if addr == "" {
		t.Skip("set MUARA_E2E_REDIS=<host:port> to run against a live redis")
	}

	opt := asynq.RedisClientOpt{Addr: addr, DB: 14}
	inspector := asynq.NewInspector(opt)
	t.Cleanup(func() {
		for _, q := range []string{QueueCritical, QueueDefault, QueueHeavy} {
			if _, err := inspector.DeleteAllPendingTasks(q); err != nil {
				t.Errorf("cleanup queue %s: %v", q, err)
			}
		}
		_ = inspector.Close()
	})
	return inspector
}

func TestTaskQueueAssignments(t *testing.T) {
	inspector := newTestInspector(t)
	ctx := context.Background()

	client := asynq.NewClient(asynq.RedisClientOpt{Addr: os.Getenv("MUARA_E2E_REDIS"), DB: 14})
	defer client.Close()

	cases := []struct {
		name     string
		build    func() (*asynq.Task, error)
		want     string
		wantTo   time.Duration
		maxRetry int
	}{
		{
			name: "alert checks are interactive and go critical",
			build: func() (*asynq.Task, error) {
				return NewAlertCheckTask(AlertCheckPayload{AlertID: "a-1", Title: "revenue drop"})
			},
			// No explicit timeout here, so this pins Asynq's own default.
			want: QueueCritical, wantTo: 30 * time.Minute, maxRetry: 3,
		},
		{
			name: "queries are default priority",
			build: func() (*asynq.Task, error) {
				return NewQueryExecuteTask(QueryExecutePayload{WorkspaceID: "ws-1", RawSQL: "SELECT 1"})
			},
			// Slot deadline plus the engine statement timeout: a stuck query must not
			// occupy a worker for the 30 minutes Asynq would otherwise allow.
			want:   QueueDefault,
			wantTo: 90 * time.Second, maxRetry: 3,
		},
		{
			name: "exports are heavy and may run long",
			build: func() (*asynq.Task, error) {
				return NewExportJobTask(ExportJobPayload{ExportJobID: "e-1", Format: "csv"})
			},
			want: QueueHeavy, wantTo: 5 * time.Minute, maxRetry: 2,
		},
		{
			name: "digests are scheduled, not interactive",
			build: func() (*asynq.Task, error) {
				return NewDigestDispatchTask(DigestDispatchPayload{AlertID: "a-2", Channel: "email"})
			},
			want: QueueDefault, wantTo: 5 * time.Minute, maxRetry: 3,
		},
	}

	for _, tc := range cases {
		task, err := tc.build()
		if err != nil {
			t.Fatalf("%s: build task: %v", tc.name, err)
		}

		info, err := client.EnqueueContext(ctx, task)
		if err != nil {
			t.Fatalf("%s: enqueue: %v", tc.name, err)
		}
		if info.Queue != tc.want {
			t.Errorf("%s: queue = %s, want %s", tc.name, info.Queue, tc.want)
		}
		if info.MaxRetry != tc.maxRetry {
			t.Errorf("%s: max retry = %d, want %d", tc.name, info.MaxRetry, tc.maxRetry)
		}

		// Read it back from Redis rather than trusting the enqueue response.
		found := false
		listed, err := inspector.ListPendingTasks(tc.want)
		if err != nil {
			t.Fatalf("%s: inspect %s: %v", tc.name, tc.want, err)
		}
		for _, ti := range listed {
			if ti.Type == task.Type() {
				found = true
				if ti.Timeout != tc.wantTo {
					t.Errorf("%s: timeout = %s, want %s", tc.name, ti.Timeout, tc.wantTo)
				}
			}
		}
		if !found {
			t.Errorf("%s: task %s never appeared in queue %s", tc.name, task.Type(), tc.want)
		}
	}
}

// The worker runs with these weights, so ordering them is what makes an alert
// interrupt an export rather than queue behind it.
func TestWorkerQueueWeights(t *testing.T) {
	critical, scheduled, heavy := QueueWeights[QueueCritical], QueueWeights[QueueDefault], QueueWeights[QueueHeavy]
	if critical <= scheduled || scheduled <= heavy {
		t.Errorf("queue weights are not ordered: %v", QueueWeights)
	}
	if len(QueueWeights) != 3 {
		t.Errorf("queue weights list %v, want exactly the three declared queues", QueueWeights)
	}
}
