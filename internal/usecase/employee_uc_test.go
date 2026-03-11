package usecase_test

import (
	"context"
	"errors"
	"fmt"
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
