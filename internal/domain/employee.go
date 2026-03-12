package domain

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type Employee struct {
	ID                  string         `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	CompanyID           string         `gorm:"type:uuid;not null" json:"company_id"`
	EmployeeID          string         `gorm:"type:varchar(100);uniqueIndex;not null" json:"employee_id"`
	Email               string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	PhoneNumber         string         `gorm:"type:varchar(50);uniqueIndex;not null" json:"phone_number"`
	Password            string         `gorm:"type:varchar(255);not null" json:"-"`
	IsTncAccepted       bool           `gorm:"default:false" json:"is_tnc_accepted"`
	Role                string         `gorm:"type:varchar(50);default:'staff'" json:"role" enums:"staff,admin" example:"staff"`
	SelfieURL           *string        `gorm:"type:text" json:"selfie_url,omitempty"`
	SelfieRegisteredAt  *time.Time     `json:"selfie_registered_at,omitempty"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`

	// Relation
	Company Company `gorm:"foreignKey:CompanyID" json:"company,omitzero"`
}

type EmployeeSequenceRepository interface {
	// guarantee atomic operation: insert new (1) or increment (+1)
	IncrementAndGetCounter(ctx context.Context, companyID string, year int) (int, error)
}

type EmployeeUsecase interface {
	GenerateEmployeeID(ctx context.Context, companyID string, companyCode string) (string, error)
	Register(ctx context.Context, req *RegisterRequest) error
}

type RegisterRequest struct {
	CompanyCode   string `json:"company_code" binding:"required"`
	Email         string `json:"email" binding:"required"`
	PhoneNumber   string `json:"phone_number" binding:"required"`
	Password      string `json:"password" binding:"required,min=6"`
	IsTncAccepted bool   `json:"is_tnc_accepted" binding:"required"`
}

type RequestOTPRequest struct {
	Email string `json:"email" binding:"required,email" example:"john@company.com"`
}

type VerifyOTPRequest struct {
	Email string `json:"email" binding:"required,email" example:"john@company.com"`
	OTP   string `json:"otp" binding:"required,len=6" example:"123456"`
}

// LoginRequest accepts one of employee_id, email, or phone_number as the login identifier.
type LoginRequest struct {
	EmployeeID  string `json:"employee_id" example:"GOTO-2026-0001"`
	Email       string `json:"email" example:"john@company.com"`
	PhoneNumber string `json:"phone_number" example:"08123456789"`
	Password    string `json:"password" binding:"required" example:"secret123"`
}

type AuthUsecase interface {
	RequestOTP(ctx context.Context, req *RequestOTPRequest) error
	VerifyOTP(ctx context.Context, req *VerifyOTPRequest) (string, error)
	Login(ctx context.Context, req *LoginRequest) (string, error)
}

type OTPRepository interface {
	SetOTP(ctx context.Context, email string, otp string, expiration time.Duration) error
	GetOTP(ctx context.Context, email string) (string, error)
	DeleteOTP(ctx context.Context, email string) error
}

var (
	ErrSelfieAlreadyRegistered = errors.New("selfie already registered for this employee")
	ErrSelfieNotRegistered     = errors.New("no selfie registered for this employee")
)

// Employee interfaces
type EmployeeRepository interface {
	Create(ctx context.Context, employee *Employee) error
	GetByEmail(ctx context.Context, email string) (*Employee, error)
	GetByEmployeeID(ctx context.Context, employeeID string) (*Employee, error)
	GetByPhoneNumber(ctx context.Context, phoneNumber string) (*Employee, error)
	RegisterSelfie(ctx context.Context, employeeID string, selfieURL string) error
}

type CompanyRepository interface {
	GetByCode(ctx context.Context, code string) (*Company, error)
	GetByID(ctx context.Context, id string) (*Company, error)
}
