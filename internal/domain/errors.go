package domain

import (
	"errors"
	"strings"
)

// ErrorResponse is the standard error response format with field-specific errors.
type ErrorResponse struct {
	Message string            `json:"message"`
	Errors  map[string]string `json:"errors"`
}

// NewFieldError creates an ErrorResponse with a single field error.
func NewFieldError(message, field, detail string) ErrorResponse {
	return ErrorResponse{
		Message: message,
		Errors:  map[string]string{field: detail},
	}
}

// Sentinel errors for login
var (
	ErrIdentifierRequired = errors.New("provide employee_id, email, or phone_number")
	ErrAccountNotFound    = errors.New("account not found")
	ErrWrongPassword      = errors.New("wrong password")
)

// Sentinel errors for register
var (
	ErrInvalidEmailDomain      = errors.New("invalid email domain")
	ErrInvalidCompanyCode      = errors.New("company code not registered")
	ErrEmailAlreadyRegistered  = errors.New("email already registered")
	ErrPhoneAlreadyRegistered  = errors.New("phone number already registered")
)

// disposableEmailDomains is a blocklist of known disposable/testing email domains.
var disposableEmailDomains = []string{
	"yopmail.com",
	"mailinator.com",
	"tempmail.com",
	"guerrillamail.com",
	"throwaway.email",
	"fakeinbox.com",
	"sharklasers.com",
	"guerrillamailblock.com",
	"grr.la",
	"dispostable.com",
	"maildrop.cc",
	"trashmail.com",
	"trashmail.me",
	"spamgourmet.com",
	"10minutemail.com",
	"temp-mail.org",
}

// IsDisposableEmail returns true if the email uses a known disposable/testing domain.
func IsDisposableEmail(email string) bool {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return false
	}
	domain := strings.ToLower(parts[1])
	for _, blocked := range disposableEmailDomains {
		if domain == blocked {
			return true
		}
	}
	return false
}
