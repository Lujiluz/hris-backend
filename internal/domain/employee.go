package domain

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Employee struct {
	ID            string         `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	CompanyID     string         `gorm:"type:uuid;not null" json:"company_id"`
	EmployeeID    string         `gorm:"type:varchar(100);uniqueIndex;not null" json:"employee_id"`
	Email         string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	PhoneNumber   string         `gorm:"type:varchar(50);uniqueIndex;not null" json:"phone_number"`
	Password      string         `gorm:"type:varchar(255);not null" json:"-"`
	IsTncAccepted bool           `gorm:"default:false" json:"is_tnc_accepted"`
	Role          string         `gorm:"type:varchar(50);default:'staff'" json:"role"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

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
	Email string `json:"email" binding:"required,email"`
}

type VerifyOTPRequest struct {
	Email string `json:"email" binding:"required,email"`
	OTP   string `json:"otp" binding:"required,len=6"`
}

type LoginRequest struct {
	EmployeeID  string `json:"employee_id"`
	Email       string `json:"email"`
	PhoneNumber string `json:"phone_number"`
	Password    string `json:"password" binding:"required"`
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

// Employee interfaces
type EmployeeRepository interface {
	Create(ctx context.Context, employee *Employee) error
	GetByEmail(ctx context.Context, email string) (*Employee, error)
	GetByEmployeeID(ctx context.Context, employeeID string) (*Employee, error)
	GetByPhoneNumber(ctx context.Context, phoneNumber string) (*Employee, error)
}

type CompanyRepository interface {
	GetByCode(ctx context.Context, code string) (*Company, error)
	GetByID(ctx context.Context, id string) (*Company, error)
}
