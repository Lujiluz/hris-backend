package usecase

import (
	"context"
	"fmt"
	"hris-backend/internal/domain"
	"time"
)

type employeeUsecase struct {
	sequenceRepo domain.EmployeeSequenceRepository
}

func NewEmployeeUsecase(seqRepo domain.EmployeeSequenceRepository) domain.EmployeeUsecase {
	return &employeeUsecase{
		sequenceRepo: seqRepo,
	}
}

func (uc *employeeUsecase) GenerateEmployeeID(ctx context.Context, companyID string, companyCode string) (string, error) {
	currentYear := time.Now().Year()

	counter, err := uc.sequenceRepo.IncrementAndGetCounter(ctx, companyID, currentYear)
	if err != nil {
		return "", fmt.Errorf("failed to get sequence counter: %w", err)
	}

	employeeID := fmt.Sprintf("%s-%d-%04d", companyCode, currentYear, counter)

	return employeeID, nil
}
