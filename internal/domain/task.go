package domain

import "context"

const TaskSendOTPEmail = "email:send_otp"

type OTPEmailPayload struct {
	Email   string `json:"email"`
	OTPCode string `json:"otp_code"`
}

// EmailTaskEnqueuer abstracts enqueuing email tasks.
type EmailTaskEnqueuer interface {
	EnqueueOTPEmail(ctx context.Context, email, otpCode string) error
}

// MailSender abstracts sending OTP emails (wraps pkg/mail for testability).
type MailSender interface {
	SendOTP(toEmail, otpCode string) error
}
