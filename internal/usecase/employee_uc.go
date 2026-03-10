package usecase

import (
	"context"
	"errors"
	"fmt"
	"hris-backend/internal/domain"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type employeeUsecase struct {
	sequenceRepo domain.EmployeeSequenceRepository
	employeeRepo domain.EmployeeRepository
	companyRepo  domain.CompanyRepository
}

func NewEmployeeUsecase(seqRepo domain.EmployeeSequenceRepository, empRepo domain.EmployeeRepository, compRepo domain.CompanyRepository) domain.EmployeeUsecase {
	return &employeeUsecase{
		sequenceRepo: seqRepo,
		employeeRepo: empRepo,
		companyRepo:  compRepo,
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

// register use case
func (uc *employeeUsecase) Register(ctx context.Context, req *domain.RegisterRequest) error {
	// check if the company code is valid
	company, err := uc.companyRepo.GetByCode(ctx, req.CompanyCode)
	if err != nil {
		return errors.New("invalid company code")
	}

	// generate employee ID
	employeeID, err := uc.GenerateEmployeeID(ctx, company.ID, company.CompanyCode)
	if err != nil {
		return err
	}

	// hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Employee's entity prep
	newEmployee := &domain.Employee{
		CompanyID:     company.ID,
		EmployeeID:    employeeID,
		Email:         req.Email,
		Password:      string(hashedPassword),
		PhoneNumber:   req.PhoneNumber,
		IsTncAccepted: req.IsTncAccepted,
		Role:          "staff",
	}

	// database write
	return uc.employeeRepo.Create(ctx, newEmployee)
}
