package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"hris-backend/internal/domain"
	"hris-backend/internal/usecase"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockSequenceRepo struct {
	mock.Mock
}

func (m *MockSequenceRepo) IncrementAndGetCounter(ctx context.Context, companyID string, year int) (int, error) {
	args := m.Called(ctx, companyID, year)
	return args.Int(0), args.Error(1)
}

type MockEmployeeRepo struct {
	mock.Mock
}

func (m *MockEmployeeRepo) Create(ctx context.Context, emp *domain.Employee) error {
	args := m.Called(ctx, emp)
	return args.Error(0)
}
func (m *MockEmployeeRepo) GetByEmail(ctx context.Context, email string) (*domain.Employee, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Employee), args.Error(1)
}
func (m *MockEmployeeRepo) GetByEmployeeID(ctx context.Context, id string) (*domain.Employee, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Employee), args.Error(1)
}
func (m *MockEmployeeRepo) GetByPhoneNumber(ctx context.Context, phone string) (*domain.Employee, error) {
	args := m.Called(ctx, phone)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Employee), args.Error(1)
}
func (m *MockEmployeeRepo) RegisterSelfie(ctx context.Context, employeeID string, selfieURL string) error {
	args := m.Called(ctx, employeeID, selfieURL)
	return args.Error(0)
}
func (m *MockEmployeeRepo) GetProfileByEmployeeID(ctx context.Context, employeeID string) (*domain.Employee, error) {
	args := m.Called(ctx, employeeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Employee), args.Error(1)
}

type MockCompanyRepo struct {
	mock.Mock
}

func (m *MockCompanyRepo) GetByCode(ctx context.Context, code string) (*domain.Company, error) {
	args := m.Called(ctx, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Company), args.Error(1)
}
func (m *MockCompanyRepo) GetByID(ctx context.Context, id string) (*domain.Company, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Company), args.Error(1)
}

func TestGenerateEmployeeID(t *testing.T) {
	currentYear := time.Now().Year()
	companyID := "550e8400-e29b-41d4-a716-446655440000"
	companyCode := "ARYA"

	t.Run("Success - Counter 1", func(t *testing.T) {
		mockRepo := new(MockSequenceRepo)
		// Tell mock to: if called, return 1 without error
		mockRepo.On("IncrementAndGetCounter", mock.Anything, companyID, currentYear).Return(1, nil)

		uc := usecase.NewEmployeeUsecase(mockRepo, nil, nil)
		employeeID, err := uc.GenerateEmployeeID(context.Background(), companyID, companyCode)

		assert.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("ARYA-%d-0001", currentYear), employeeID)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Success - Padding with Counter 150", func(t *testing.T) {
		mockRepo := new(MockSequenceRepo)
		mockRepo.On("IncrementAndGetCounter", mock.Anything, companyID, currentYear).Return(150, nil)

		uc := usecase.NewEmployeeUsecase(mockRepo, nil, nil)
		employeeID, err := uc.GenerateEmployeeID(context.Background(), companyID, companyCode)

		assert.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("ARYA-%d-0150", currentYear), employeeID)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Error from Repository", func(t *testing.T) {
		mockRepo := new(MockSequenceRepo)
		dbError := errors.New("database connection lost")
		mockRepo.On("IncrementAndGetCounter", mock.Anything, companyID, currentYear).Return(0, dbError)

		uc := usecase.NewEmployeeUsecase(mockRepo, nil, nil)
		employeeID, err := uc.GenerateEmployeeID(context.Background(), companyID, companyCode)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get sequence counter")
		assert.Empty(t, employeeID)
		mockRepo.AssertExpectations(t)
	})
}

func TestIsDisposableEmail(t *testing.T) {
	tests := []struct {
		email    string
		expected bool
	}{
		{"user@yopmail.com", true},
		{"user@mailinator.com", true},
		{"user@tempmail.com", true},
		{"user@10minutemail.com", true},
		{"user@gmail.com", false},
		{"user@yahoo.com", false},
		{"user@company.co.id", false},
		{"notanemail", false},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			assert.Equal(t, tt.expected, domain.IsDisposableEmail(tt.email))
		})
	}
}

func TestRegister(t *testing.T) {
	validCompany := &domain.Company{
		ID:          "company-uuid",
		CompanyCode: "ARYA",
	}

	validReq := &domain.RegisterRequest{
		CompanyCode:   "ARYA",
		Email:         "user@gmail.com",
		PhoneNumber:   "081234567890",
		Password:      "password123",
		IsTncAccepted: true,
	}

	t.Run("Error - Disposable email domain", func(t *testing.T) {
		uc := usecase.NewEmployeeUsecase(nil, nil, nil)
		req := *validReq
		req.Email = "user@yopmail.com"
		err := uc.Register(context.Background(), &req)
		assert.ErrorIs(t, err, domain.ErrInvalidEmailDomain)
	})

	t.Run("Error - Invalid company code", func(t *testing.T) {
		mockCompany := new(MockCompanyRepo)
		mockCompany.On("GetByCode", mock.Anything, "INVALID").Return(nil, errors.New("not found"))

		uc := usecase.NewEmployeeUsecase(nil, nil, mockCompany)
		req := *validReq
		req.CompanyCode = "INVALID"
		err := uc.Register(context.Background(), &req)
		assert.ErrorIs(t, err, domain.ErrInvalidCompanyCode)
		mockCompany.AssertExpectations(t)
	})

	t.Run("Success - Valid registration", func(t *testing.T) {
		currentYear := time.Now().Year()
		mockSeq := new(MockSequenceRepo)
		mockEmp := new(MockEmployeeRepo)
		mockCompany := new(MockCompanyRepo)

		mockCompany.On("GetByCode", mock.Anything, "ARYA").Return(validCompany, nil)
		mockSeq.On("IncrementAndGetCounter", mock.Anything, "company-uuid", currentYear).Return(1, nil)
		mockEmp.On("Create", mock.Anything, mock.AnythingOfType("*domain.Employee")).Return(nil)

		uc := usecase.NewEmployeeUsecase(mockSeq, mockEmp, mockCompany)
		err := uc.Register(context.Background(), validReq)
		assert.NoError(t, err)
		mockCompany.AssertExpectations(t)
		mockSeq.AssertExpectations(t)
		mockEmp.AssertExpectations(t)
	})
}
