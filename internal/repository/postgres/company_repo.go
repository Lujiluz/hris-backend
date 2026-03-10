package postgres

import (
	"context"
	"hris-backend/internal/domain"

	"gorm.io/gorm"
)

type companyRepo struct {
	db *gorm.DB
}

func NewCompanyRepository(db *gorm.DB) domain.CompanyRepository {
	return &companyRepo{db: db}
}

func (r *companyRepo) GetByCode(ctx context.Context, code string) (*domain.Company, error) {
	var company domain.Company
	err := r.db.WithContext(ctx).Where("company_code = ?", code).First(&company).Error
	if err != nil {
		return nil, err
	}

	return &company, nil
}

func (r *companyRepo) GetByID(ctx context.Context, id string) (*domain.Company, error) {
	var company domain.Company
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&company).Error; err != nil {
		return nil, err
	}
	return &company, nil
}
