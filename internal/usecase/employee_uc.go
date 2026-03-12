package usecase

import (
	"context"
	"errors"
	"fmt"
	"hris-backend/internal/domain"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
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
	// validate email domain against disposable/testing providers
	if domain.IsDisposableEmail(req.Email) {
		return domain.ErrInvalidEmailDomain
	}

	// check if the company code is valid
	company, err := uc.companyRepo.GetByCode(ctx, req.CompanyCode)
	if err != nil {
		return domain.ErrInvalidCompanyCode
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
	if err := uc.employeeRepo.Create(ctx, newEmployee); err != nil {
		return mapCreateEmployeeError(err)
	}
	return nil
}

// mapCreateEmployeeError translates PostgreSQL unique constraint violations
// into domain sentinel errors for field-specific error responses.
func mapCreateEmployeeError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		constraint := strings.ToLower(pgErr.ConstraintName)
		switch {
		case strings.Contains(constraint, "email"):
			return domain.ErrEmailAlreadyRegistered
		case strings.Contains(constraint, "phone"):
			return domain.ErrPhoneAlreadyRegistered
		}
	}
	return err
}
