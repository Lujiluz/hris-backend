package postgres

import (
	"context"
	"errors"
	"hris-backend/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type employeeScheduleRepo struct {
	db *gorm.DB
}

func NewEmployeeScheduleRepository(db *gorm.DB) domain.EmployeeScheduleRepository {
	return &employeeScheduleRepo{db: db}
}

func (r *employeeScheduleRepo) GetByDayOfWeek(ctx context.Context, employeeID uuid.UUID, day int) (*domain.EmployeeSchedule, error) {
	var schedule domain.EmployeeSchedule
	err := r.db.WithContext(ctx).
		Where("employee_id = ? AND day_of_week = ? AND is_active = true", employeeID, day).
		First(&schedule).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &schedule, nil
}
