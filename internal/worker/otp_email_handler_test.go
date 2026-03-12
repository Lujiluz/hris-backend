package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"hris-backend/internal/domain"
	"hris-backend/internal/worker"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockMailSender implements domain.MailSender
type MockMailSender struct {
	mock.Mock
}

func (m *MockMailSender) SendOTP(toEmail, otpCode string) error {
	args := m.Called(toEmail, otpCode)
	return args.Error(0)
}

func newOTPTask(t *testing.T, payload domain.OTPEmailPayload) *asynq.Task {
	t.Helper()
	data, _ := json.Marshal(payload)
	return asynq.NewTask(domain.TaskSendOTPEmail, data)
}

func TestOTPEmailHandler_ProcessTask(t *testing.T) {
	ctx := context.Background()

	t.Run("Success - email sent", func(t *testing.T) {
		mockSender := new(MockMailSender)
		mockSender.On("SendOTP", "user@company.com", "123456").Return(nil)

		handler := worker.NewOTPEmailHandler(mockSender)
		task := newOTPTask(t, domain.OTPEmailPayload{Email: "user@company.com", OTPCode: "123456"})

		err := handler.ProcessTask(ctx, task)

		assert.NoError(t, err)
		mockSender.AssertExpectations(t)
	})

	t.Run("Error - invalid JSON payload (skip retry)", func(t *testing.T) {
		mockSender := new(MockMailSender)
		handler := worker.NewOTPEmailHandler(mockSender)
		task := asynq.NewTask(domain.TaskSendOTPEmail, []byte("not valid json{"))

		err := handler.ProcessTask(ctx, task)

		assert.Error(t, err)
		assert.ErrorIs(t, err, asynq.SkipRetry)
		mockSender.AssertNotCalled(t, "SendOTP")
	})

	t.Run("Error - empty email in payload (skip retry)", func(t *testing.T) {
		mockSender := new(MockMailSender)
		handler := worker.NewOTPEmailHandler(mockSender)
		task := newOTPTask(t, domain.OTPEmailPayload{Email: "", OTPCode: "123456"})

		err := handler.ProcessTask(ctx, task)

		assert.Error(t, err)
		assert.ErrorIs(t, err, asynq.SkipRetry)
		mockSender.AssertNotCalled(t, "SendOTP")
	})

	t.Run("Error - SMTP failure is retryable", func(t *testing.T) {
		mockSender := new(MockMailSender)
		smtpErr := errors.New("connection refused")
		mockSender.On("SendOTP", "user@company.com", "123456").Return(smtpErr)

		handler := worker.NewOTPEmailHandler(mockSender)
		task := newOTPTask(t, domain.OTPEmailPayload{Email: "user@company.com", OTPCode: "123456"})

		err := handler.ProcessTask(ctx, task)

		assert.ErrorIs(t, err, smtpErr)
		assert.NotErrorIs(t, err, asynq.SkipRetry)
		mockSender.AssertExpectations(t)
	})
}
