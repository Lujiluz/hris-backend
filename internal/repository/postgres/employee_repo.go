package postgres

import (
	"context"
	"hris-backend/internal/domain"

	"gorm.io/gorm"
)

type employeeRepo struct {
	db *gorm.DB
}

func NewEmployeeRepository(db *gorm.DB) domain.EmployeeRepository {
	return &employeeRepo{db: db}
}

func (r *employeeRepo) Create(ctx context.Context, employee *domain.Employee) error {
	return r.db.WithContext(ctx).Create(employee).Error
}
