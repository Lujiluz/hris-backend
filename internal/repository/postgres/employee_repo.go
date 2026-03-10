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

func (r *employeeRepo) GetByEmail(ctx context.Context, email string) (*domain.Employee, error) {
	var employee domain.Employee

	err := r.db.WithContext(ctx).Where("email = ?", email).First(&employee).Error
	if err != nil {
		return nil, err
	}

	return &employee, nil
}

func (r *employeeRepo) GetByEmployeeID(ctx context.Context, employeeID string) (*domain.Employee, error) {
	var employee domain.Employee
	err := r.db.WithContext(ctx).Where("employee_id = ?", employeeID).First(&employee).Error
	if err != nil {
		return nil, err
	}
	return &employee, nil
}
