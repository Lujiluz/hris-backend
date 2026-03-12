package worker

import (
	"context"

	"hris-backend/internal/domain"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
)

type WorkerServer struct {
	server *asynq.Server
	mux    *asynq.ServeMux
}

func NewWorkerServer(redisAddr, redisPassword string, sender domain.MailSender) *WorkerServer {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     redisAddr,
			Password: redisPassword,
		},
		asynq.Config{
			Concurrency: 5,
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				log.Error().
					Err(err).
					Str("task_type", task.Type()).
					Msg("Worker task permanently failed")
			}),
		},
	)

	mux := asynq.NewServeMux()
	mux.Handle(domain.TaskSendOTPEmail, NewOTPEmailHandler(sender))

	return &WorkerServer{server: srv, mux: mux}
}

func (w *WorkerServer) Start() error {
	go func() {
		if err := w.server.Run(w.mux); err != nil {
			log.Fatal().Err(err).Msg("Worker server failed")
		}
	}()
	log.Info().Msg("Background worker started")
	return nil
}

func (w *WorkerServer) Shutdown() {
	w.server.Shutdown()
	log.Info().Msg("Background worker stopped")
}
