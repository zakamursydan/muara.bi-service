package worker

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/hibiken/asynq"
	"muara.bi/core/internal/cache"
	"muara.bi/core/internal/config"
	"muara.bi/core/internal/repository/pgsql"
	"muara.bi/core/internal/service"
	"muara.bi/core/pkg/logging"
)

// Server coordinates Asynq task processing
type Server struct {
	srv    *asynq.Server
	mux    *asynq.ServeMux
	logger *logging.Logger
}

// NewWorkerServer creates a new worker server with weighted queues
func NewWorkerServer(
	cfg *config.Config,
	db *sql.DB,
	dashboardRepo pgsql.DashboardRepositoryProvider,
	widgetRepo pgsql.WidgetRepositoryProvider,
	preparer *service.SessionPreparer,
	access service.AccessServiceProvider,
	queryCache *cache.QueryCache,
	logger *logging.Logger,
) *Server {
	redisOpt := asynq.RedisClientOpt{
		Addr:     fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}

	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				QueueCritical: 6,
				QueueDefault:  3,
				QueueHeavy:    1,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				logger.Error(fmt.Sprintf("asynq task failed: type=%s payload=%s err=%v", task.Type(), string(task.Payload()), err))
			}),
		},
	)

	mux := asynq.NewServeMux()

	queryWorker := NewQueryWorker(preparer, access, queryCache, logger)
	exportWorker := NewExportWorker(db, preparer, access, logger)
	alertWorker := NewAlertWorker(db, preparer, logger)
	digestWorker := NewDigestWorker(db, dashboardRepo, widgetRepo, preparer, logger)

	mux.HandleFunc(TypeQueryExecute, queryWorker.ProcessTask)
	mux.HandleFunc(TypeExportJob, exportWorker.ProcessTask)
	mux.HandleFunc(TypeAlertCheck, alertWorker.ProcessTask)
	mux.HandleFunc(TypeDigestDispatch, digestWorker.ProcessTask)

	return &Server{
		srv:    srv,
		mux:    mux,
		logger: logger,
	}
}

// Run starts the Asynq worker server
func (s *Server) Run() error {
	s.logger.Info("starting asynq worker pool (queues: critical=6, default=3, heavy=1)")
	return s.srv.Run(s.mux)
}

// Shutdown gracefully stops the Asynq worker server
func (s *Server) Shutdown() {
	s.logger.Info("shutting down asynq worker pool...")
	s.srv.Shutdown()
}
