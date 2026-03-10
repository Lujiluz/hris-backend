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
	// 1. Cek karyawan ada atau nggak
	_, err := uc.empRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return errors.New("email not registered")
	}

	// 2. Generate 6 digit OTP acak
	rand.Seed(time.Now().UnixNano())
	otpCode := fmt.Sprintf("%06d", rand.Intn(1000000))

	// 3. Simpan ke Redis  (TTL 5 menit)
	err = uc.otpRepo.SetOTP(ctx, req.Email, otpCode, 5*time.Minute)
	if err != nil {
		return errors.New("failed to generate OTP")
	}

	// 4. Kirim Email
	// Catatan: Di production, mending ini dilempar ke background job (Asynq) biar API gak nungguin email ke-send
	go mail.SendOTP(req.Email, otpCode)

	return nil
}

func (uc *authUsecase) VerifyOTP(ctx context.Context, req *domain.VerifyOTPRequest) (string, error) {
	// 1. Ambil OTP dari Redis
	savedOTP, err := uc.otpRepo.GetOTP(ctx, req.Email)
	if err != nil {
		return "", errors.New("OTP expired or not found")
	}

	// 2. Cocokkan OTP
	if savedOTP != req.OTP {
		return "", errors.New("invalid OTP")
	}

	// 3. OTP Valid -> Ambil data Employee buat masukin ke Token
	emp, err := uc.empRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return "", errors.New("employee not found")
	}

	// 4. Hapus OTP dari Redis biar gak bisa dipake 2x
	uc.otpRepo.DeleteOTP(ctx, req.Email)

	// 5. Generate JWT Token
	token, err := jwt.GenerateToken(emp.EmployeeID, emp.Role)
	if err != nil {
		return "", errors.New("failed to generate token")
	}

	return token, nil
}

func (uc *authUsecase) Login(ctx context.Context, req *domain.LoginRequest) (string, error) {
	// 1. Cari user pakai Employee ID
	emp, err := uc.empRepo.GetByEmployeeID(ctx, req.EmployeeID)
	if err != nil {
		return "", errors.New("invalid employee id or password")
	}

	// 2. Komparasi Password dari Input vs DB (yang di-hash)
	err = bcrypt.CompareHashAndPassword([]byte(emp.Password), []byte(req.Password))
	if err != nil {
		return "", errors.New("invalid employee id or password")
	}

	// 3. Password cocok -> Generate JWT Token
	token, err := jwt.GenerateToken(emp.EmployeeID, emp.Role)
	if err != nil {
		return "", errors.New("failed to generate token")
	}

	return token, nil
}
