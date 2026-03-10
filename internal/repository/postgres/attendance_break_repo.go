package postgres

import (
	"context"
	"errors"
	"hris-backend/internal/domain"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type attendanceBreakRepo struct {
	db *gorm.DB
}

func NewAttendanceBreakRepository(db *gorm.DB) domain.AttendanceBreakRepository {
	return &attendanceBreakRepo{db: db}
}

func (r *attendanceBreakRepo) StartBreak(ctx context.Context, b *domain.AttendanceBreak) error {
	return r.db.WithContext(ctx).Create(b).Error
}

func (r *attendanceBreakRepo) EndLatestBreak(ctx context.Context, attendanceID uuid.UUID, endTime time.Time) error {
	return r.db.WithContext(ctx).
		Model(&domain.AttendanceBreak{}).
		Where("attendance_record_id = ? AND break_end_at IS NULL", attendanceID).
		Update("break_end_at", endTime).Error
}

func (r *attendanceBreakRepo) GetOpenBreak(ctx context.Context, attendanceID uuid.UUID) (*domain.AttendanceBreak, error) {
	var b domain.AttendanceBreak
	err := r.db.WithContext(ctx).
		Where("attendance_record_id = ? AND break_end_at IS NULL", attendanceID).
		First(&b).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &b, nil
}

func (r *attendanceBreakRepo) SumBreakMinutes(ctx context.Context, attendanceID uuid.UUID) (int, error) {
	var totalSeconds float64
	err := r.db.WithContext(ctx).
		Model(&domain.AttendanceBreak{}).
		Where("attendance_record_id = ? AND break_end_at IS NOT NULL", attendanceID).
		Select("COALESCE(SUM(EXTRACT(EPOCH FROM (break_end_at - break_start_at))), 0)").
		Scan(&totalSeconds).Error
	if err != nil {
		return 0, err
	}
	return int(totalSeconds / 60), nil
}
