package postgres

import (
	"context"
	"errors"
	"hris-backend/internal/domain"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type attendanceRepo struct {
	db *gorm.DB
}

func NewAttendanceRepository(db *gorm.DB) domain.AttendanceRepository {
	return &attendanceRepo{db: db}
}

func (r *attendanceRepo) CreateClockIn(ctx context.Context, record *domain.AttendanceRecord) error {
	return r.db.WithContext(ctx).Create(record).Error
}

func (r *attendanceRepo) GetTodayRecord(ctx context.Context, employeeID uuid.UUID, date time.Time) (*domain.AttendanceRecord, error) {
	var record domain.AttendanceRecord
	err := r.db.WithContext(ctx).
		Where("employee_id = ? AND work_date = ?", employeeID, date.Format("2006-01-02")).
		First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (r *attendanceRepo) UpdateClockOut(ctx context.Context, record *domain.AttendanceRecord) error {
	return r.db.WithContext(ctx).
		Model(record).
		Updates(map[string]interface{}{
			"clock_out_at":     record.ClockOutAt,
			"status":           record.Status,
			"working_minutes":  record.WorkingMinutes,
			"overtime_minutes": record.OvertimeMinutes,
			"updated_at":       time.Now(),
		}).Error
}

func (r *attendanceRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	return r.db.WithContext(ctx).
		Model(&domain.AttendanceRecord{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		}).Error
}
