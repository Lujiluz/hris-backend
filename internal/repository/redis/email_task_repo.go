package redis

import (
	"context"
	"encoding/json"
	"time"

	"hris-backend/internal/domain"

	"github.com/hibiken/asynq"
)

type emailTaskEnqueuer struct {
	client *asynq.Client
}

func NewEmailTaskEnqueuer(client *asynq.Client) domain.EmailTaskEnqueuer {
	return &emailTaskEnqueuer{client: client}
}

func (e *emailTaskEnqueuer) EnqueueOTPEmail(ctx context.Context, email, otpCode string) error {
	payload, err := json.Marshal(domain.OTPEmailPayload{
		Email:   email,
		OTPCode: otpCode,
	})
	if err != nil {
		return err
	}

	task := asynq.NewTask(
		domain.TaskSendOTPEmail,
		payload,
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Second),
		asynq.Retention(1*time.Hour),
	)

	_, err = e.client.EnqueueContext(ctx, task)
	return err
}
