package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"hris-backend/internal/domain"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
)

type OTPEmailHandler struct {
	sender domain.MailSender
}

func NewOTPEmailHandler(sender domain.MailSender) *OTPEmailHandler {
	return &OTPEmailHandler{sender: sender}
}

func (h *OTPEmailHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload domain.OTPEmailPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal OTP email payload: %w", asynq.SkipRetry)
	}

	if payload.Email == "" {
		return fmt.Errorf("OTP email payload has empty email: %w", asynq.SkipRetry)
	}

	log.Info().Str("email", payload.Email).Msg("Sending OTP email")

	if err := h.sender.SendOTP(payload.Email, payload.OTPCode); err != nil {
		log.Error().Err(err).Str("email", payload.Email).Msg("Failed to send OTP email, will retry")
		return err
	}

	log.Info().Str("email", payload.Email).Msg("OTP email sent successfully")
	return nil
}
