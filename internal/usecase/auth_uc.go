package usecase

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"hris-backend/internal/domain"
	"hris-backend/pkg/jwt"
	"hris-backend/pkg/mail"

	"golang.org/x/crypto/bcrypt"
)

type authUsecase struct {
	empRepo domain.EmployeeRepository
	otpRepo domain.OTPRepository
}

func NewAuthUsecase(empRepo domain.EmployeeRepository, otpRepo domain.OTPRepository) domain.AuthUsecase {
	return &authUsecase{empRepo: empRepo, otpRepo: otpRepo}
}

func (uc *authUsecase) RequestOTP(ctx context.Context, req *domain.RequestOTPRequest) error {
	_, err := uc.empRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return errors.New("email not registered")
	}

	rand.Seed(time.Now().UnixNano())
	otpCode := fmt.Sprintf("%06d", rand.Intn(1000000))

	err = uc.otpRepo.SetOTP(ctx, req.Email, otpCode, 5*time.Minute)
	if err != nil {
		return errors.New("failed to generate OTP")
	}

	// TODO: use a worker to handle sending OTP
	go mail.SendOTP(req.Email, otpCode)

	return nil
}

func (uc *authUsecase) VerifyOTP(ctx context.Context, req *domain.VerifyOTPRequest) (string, error) {
	savedOTP, err := uc.otpRepo.GetOTP(ctx, req.Email)
	if err != nil {
		return "", errors.New("OTP expired or not found")
	}

	if savedOTP != req.OTP {
		return "", errors.New("invalid OTP")
	}

	emp, err := uc.empRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return "", errors.New("employee not found")
	}

	uc.otpRepo.DeleteOTP(ctx, req.Email)

	token, err := jwt.GenerateToken(emp.EmployeeID, emp.Role, emp.CompanyID)
	if err != nil {
		return "", errors.New("failed to generate token")
	}

	return token, nil
}

func (uc *authUsecase) Login(ctx context.Context, req *domain.LoginRequest) (string, error) {
	var emp *domain.Employee
	var err error

	switch {
	case req.EmployeeID != "":
		emp, err = uc.empRepo.GetByEmployeeID(ctx, req.EmployeeID)
	case req.Email != "":
		emp, err = uc.empRepo.GetByEmail(ctx, req.Email)
	case req.PhoneNumber != "":
		emp, err = uc.empRepo.GetByPhoneNumber(ctx, req.PhoneNumber)
	default:
		return "", domain.ErrIdentifierRequired
	}

	if err != nil {
		return "", domain.ErrAccountNotFound
	}

	if bcrypt.CompareHashAndPassword([]byte(emp.Password), []byte(req.Password)) != nil {
		return "", domain.ErrWrongPassword
	}

	token, err := jwt.GenerateToken(emp.EmployeeID, emp.Role, emp.CompanyID)
	if err != nil {
		return "", errors.New("failed to generate token")
	}

	return token, nil
}
