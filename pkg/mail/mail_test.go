package mail_test

import (
	"os"
	"testing"

	"hris-backend/pkg/mail"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
)

func TestSendOTP_RealEmail(t *testing.T) {
	err := godotenv.Load("../../.env")
	if err != nil {
		t.Fatalf("Failed to load .env file: %v", err)
	}

	if os.Getenv("SMTP_USER") == "" || os.Getenv("SMTP_PASS") == "" {
		t.Skip("SMTP credentials not found in .env, skipping test")
	}

	targetEmail := "test_integration_01@yopmail.com"
	testOTP := "999888"

	err = mail.SendOTP(targetEmail, testOTP)

	assert.NoError(t, err, "Email harusnya berhasil terkirim tanpa error ke server SMTP")
}
