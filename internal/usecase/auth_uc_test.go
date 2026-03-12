package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"hris-backend/internal/domain"
	"hris-backend/internal/usecase"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

// MockOTPRepo implements domain.OTPRepository
type MockOTPRepo struct {
	mock.Mock
}

func (m *MockOTPRepo) SetOTP(ctx context.Context, email, otp string, expiration time.Duration) error {
	args := m.Called(ctx, email, otp, expiration)
	return args.Error(0)
}

func (m *MockOTPRepo) GetOTP(ctx context.Context, email string) (string, error) {
	args := m.Called(ctx, email)
	return args.String(0), args.Error(1)
}

func (m *MockOTPRepo) DeleteOTP(ctx context.Context, email string) error {
	args := m.Called(ctx, email)
	return args.Error(0)
}

// MockEmailTaskEnqueuer implements domain.EmailTaskEnqueuer
type MockEmailTaskEnqueuer struct {
	mock.Mock
}

func (m *MockEmailTaskEnqueuer) EnqueueOTPEmail(ctx context.Context, email, otpCode string) error {
	args := m.Called(ctx, email, otpCode)
	return args.Error(0)
}

// ---- RequestOTP Tests ----

func TestRequestOTP(t *testing.T) {
	ctx := context.Background()
	email := "user@company.com"

	t.Run("Success - OTP enqueued", func(t *testing.T) {
		mockEmp := new(MockEmployeeRepo)
		mockOTP := new(MockOTPRepo)
		mockEnq := new(MockEmailTaskEnqueuer)

		mockEmp.On("GetByEmail", ctx, email).Return(&domain.Employee{Email: email}, nil)
		mockOTP.On("SetOTP", ctx, email, mock.AnythingOfType("string"), 5*time.Minute).Return(nil)
		mockEnq.On("EnqueueOTPEmail", ctx, email, mock.AnythingOfType("string")).Return(nil)

		uc := usecase.NewAuthUsecase(mockEmp, mockOTP, mockEnq)
		err := uc.RequestOTP(ctx, &domain.RequestOTPRequest{Email: email})

		assert.NoError(t, err)
		mockEmp.AssertExpectations(t)
		mockOTP.AssertExpectations(t)
		mockEnq.AssertExpectations(t)
	})

	t.Run("Error - email not registered", func(t *testing.T) {
		mockEmp := new(MockEmployeeRepo)
		mockOTP := new(MockOTPRepo)
		mockEnq := new(MockEmailTaskEnqueuer)

		mockEmp.On("GetByEmail", ctx, email).Return(nil, errors.New("not found"))

		uc := usecase.NewAuthUsecase(mockEmp, mockOTP, mockEnq)
		err := uc.RequestOTP(ctx, &domain.RequestOTPRequest{Email: email})

		assert.EqualError(t, err, "email not registered")
		mockOTP.AssertNotCalled(t, "SetOTP")
		mockEnq.AssertNotCalled(t, "EnqueueOTPEmail")
	})

	t.Run("Error - Redis SetOTP fails", func(t *testing.T) {
		mockEmp := new(MockEmployeeRepo)
		mockOTP := new(MockOTPRepo)
		mockEnq := new(MockEmailTaskEnqueuer)

		mockEmp.On("GetByEmail", ctx, email).Return(&domain.Employee{Email: email}, nil)
		mockOTP.On("SetOTP", ctx, email, mock.AnythingOfType("string"), 5*time.Minute).Return(errors.New("redis error"))

		uc := usecase.NewAuthUsecase(mockEmp, mockOTP, mockEnq)
		err := uc.RequestOTP(ctx, &domain.RequestOTPRequest{Email: email})

		assert.EqualError(t, err, "failed to generate OTP")
		mockEnq.AssertNotCalled(t, "EnqueueOTPEmail")
	})

	t.Run("Error - enqueue fails", func(t *testing.T) {
		mockEmp := new(MockEmployeeRepo)
		mockOTP := new(MockOTPRepo)
		mockEnq := new(MockEmailTaskEnqueuer)

		mockEmp.On("GetByEmail", ctx, email).Return(&domain.Employee{Email: email}, nil)
		mockOTP.On("SetOTP", ctx, email, mock.AnythingOfType("string"), 5*time.Minute).Return(nil)
		mockEnq.On("EnqueueOTPEmail", ctx, email, mock.AnythingOfType("string")).Return(errors.New("redis connection lost"))

		uc := usecase.NewAuthUsecase(mockEmp, mockOTP, mockEnq)
		err := uc.RequestOTP(ctx, &domain.RequestOTPRequest{Email: email})

		assert.EqualError(t, err, "failed to queue OTP email")
	})
}

// ---- VerifyOTP Tests ----

func TestVerifyOTP(t *testing.T) {
	ctx := context.Background()
	email := "user@company.com"
	correctOTP := "123456"

	validEmployee := &domain.Employee{
		EmployeeID: "ARYA-2026-0001",
		Email:      email,
		Role:       "staff",
		CompanyID:  "company-uuid",
	}

	t.Run("Success - valid OTP returns token", func(t *testing.T) {
		mockEmp := new(MockEmployeeRepo)
		mockOTP := new(MockOTPRepo)
		mockEnq := new(MockEmailTaskEnqueuer)

		mockOTP.On("GetOTP", ctx, email).Return(correctOTP, nil)
		mockEmp.On("GetByEmail", ctx, email).Return(validEmployee, nil)
		mockOTP.On("DeleteOTP", ctx, email).Return(nil)

		uc := usecase.NewAuthUsecase(mockEmp, mockOTP, mockEnq)
		token, err := uc.VerifyOTP(ctx, &domain.VerifyOTPRequest{Email: email, OTP: correctOTP})

		assert.NoError(t, err)
		assert.NotEmpty(t, token)
		mockOTP.AssertExpectations(t)
		mockEmp.AssertExpectations(t)
	})

	t.Run("Error - OTP expired or not found", func(t *testing.T) {
		mockEmp := new(MockEmployeeRepo)
		mockOTP := new(MockOTPRepo)
		mockEnq := new(MockEmailTaskEnqueuer)

		mockOTP.On("GetOTP", ctx, email).Return("", errors.New("redis: nil"))

		uc := usecase.NewAuthUsecase(mockEmp, mockOTP, mockEnq)
		token, err := uc.VerifyOTP(ctx, &domain.VerifyOTPRequest{Email: email, OTP: correctOTP})

		assert.EqualError(t, err, "OTP expired or not found")
		assert.Empty(t, token)
		mockEmp.AssertNotCalled(t, "GetByEmail")
	})

	t.Run("Error - OTP mismatch", func(t *testing.T) {
		mockEmp := new(MockEmployeeRepo)
		mockOTP := new(MockOTPRepo)
		mockEnq := new(MockEmailTaskEnqueuer)

		mockOTP.On("GetOTP", ctx, email).Return("999999", nil)

		uc := usecase.NewAuthUsecase(mockEmp, mockOTP, mockEnq)
		token, err := uc.VerifyOTP(ctx, &domain.VerifyOTPRequest{Email: email, OTP: correctOTP})

		assert.EqualError(t, err, "invalid OTP")
		assert.Empty(t, token)
		mockEmp.AssertNotCalled(t, "GetByEmail")
	})

	t.Run("Error - employee not found after OTP match", func(t *testing.T) {
		mockEmp := new(MockEmployeeRepo)
		mockOTP := new(MockOTPRepo)
		mockEnq := new(MockEmailTaskEnqueuer)

		mockOTP.On("GetOTP", ctx, email).Return(correctOTP, nil)
		mockEmp.On("GetByEmail", ctx, email).Return(nil, errors.New("not found"))

		uc := usecase.NewAuthUsecase(mockEmp, mockOTP, mockEnq)
		token, err := uc.VerifyOTP(ctx, &domain.VerifyOTPRequest{Email: email, OTP: correctOTP})

		assert.EqualError(t, err, "employee not found")
		assert.Empty(t, token)
	})
}

// ---- Login Tests ----

func TestLogin(t *testing.T) {
	ctx := context.Background()

	rawPassword := "password123"
	hashed, _ := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.DefaultCost)
	hashedPassword := string(hashed)

	validEmployee := &domain.Employee{
		EmployeeID: "ARYA-2026-0001",
		Email:      "user@company.com",
		PhoneNumber: "081234567890",
		Role:       "staff",
		CompanyID:  "company-uuid",
		Password:   hashedPassword,
	}

	t.Run("Success - login via EmployeeID", func(t *testing.T) {
		mockEmp := new(MockEmployeeRepo)
		mockOTP := new(MockOTPRepo)
		mockEnq := new(MockEmailTaskEnqueuer)

		mockEmp.On("GetByEmployeeID", ctx, "ARYA-2026-0001").Return(validEmployee, nil)

		uc := usecase.NewAuthUsecase(mockEmp, mockOTP, mockEnq)
		token, err := uc.Login(ctx, &domain.LoginRequest{EmployeeID: "ARYA-2026-0001", Password: rawPassword})

		assert.NoError(t, err)
		assert.NotEmpty(t, token)
	})

	t.Run("Success - login via Email", func(t *testing.T) {
		mockEmp := new(MockEmployeeRepo)
		mockOTP := new(MockOTPRepo)
		mockEnq := new(MockEmailTaskEnqueuer)

		mockEmp.On("GetByEmail", ctx, "user@company.com").Return(validEmployee, nil)

		uc := usecase.NewAuthUsecase(mockEmp, mockOTP, mockEnq)
		token, err := uc.Login(ctx, &domain.LoginRequest{Email: "user@company.com", Password: rawPassword})

		assert.NoError(t, err)
		assert.NotEmpty(t, token)
	})

	t.Run("Success - login via PhoneNumber", func(t *testing.T) {
		mockEmp := new(MockEmployeeRepo)
		mockOTP := new(MockOTPRepo)
		mockEnq := new(MockEmailTaskEnqueuer)

		mockEmp.On("GetByPhoneNumber", ctx, "081234567890").Return(validEmployee, nil)

		uc := usecase.NewAuthUsecase(mockEmp, mockOTP, mockEnq)
		token, err := uc.Login(ctx, &domain.LoginRequest{PhoneNumber: "081234567890", Password: rawPassword})

		assert.NoError(t, err)
		assert.NotEmpty(t, token)
	})

	t.Run("Error - no identifier provided", func(t *testing.T) {
		mockEmp := new(MockEmployeeRepo)
		mockOTP := new(MockOTPRepo)
		mockEnq := new(MockEmailTaskEnqueuer)

		uc := usecase.NewAuthUsecase(mockEmp, mockOTP, mockEnq)
		token, err := uc.Login(ctx, &domain.LoginRequest{Password: "password123"})

		assert.ErrorIs(t, err, domain.ErrIdentifierRequired)
		assert.Empty(t, token)
	})

	t.Run("Error - account not found", func(t *testing.T) {
		mockEmp := new(MockEmployeeRepo)
		mockOTP := new(MockOTPRepo)
		mockEnq := new(MockEmailTaskEnqueuer)

		mockEmp.On("GetByEmail", ctx, "unknown@company.com").Return(nil, errors.New("not found"))

		uc := usecase.NewAuthUsecase(mockEmp, mockOTP, mockEnq)
		token, err := uc.Login(ctx, &domain.LoginRequest{Email: "unknown@company.com", Password: "password123"})

		assert.ErrorIs(t, err, domain.ErrAccountNotFound)
		assert.Empty(t, token)
	})

	t.Run("Error - wrong password", func(t *testing.T) {
		mockEmp := new(MockEmployeeRepo)
		mockOTP := new(MockOTPRepo)
		mockEnq := new(MockEmailTaskEnqueuer)

		mockEmp.On("GetByEmail", ctx, "user@company.com").Return(validEmployee, nil)

		uc := usecase.NewAuthUsecase(mockEmp, mockOTP, mockEnq)
		token, err := uc.Login(ctx, &domain.LoginRequest{Email: "user@company.com", Password: "wrongpassword123"})

		assert.ErrorIs(t, err, domain.ErrWrongPassword)
		assert.Empty(t, token)
	})
}
