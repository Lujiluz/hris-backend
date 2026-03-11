package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrOfficeNotConfigured  = errors.New("office location not configured for this company")
	ErrMockLocationDetected = errors.New("mock/fake GPS location detected")
	ErrGPSAccuracyTooLow    = errors.New("GPS accuracy is too low, move to an open area and retry")
	ErrAlreadyClockedIn     = errors.New("already clocked in today")
	ErrNotClockedIn         = errors.New("no clock-in record found for today")
	ErrAlreadyClockedOut    = errors.New("already clocked out")
	ErrAlreadyOnBreak       = errors.New("already on break")
	ErrNotOnBreak           = errors.New("not currently on break")
	ErrInvalidBreakAction   = errors.New("invalid break action, must be 'start' or 'end'")
)

// MaxAllowedAccuracyMeters is the worst acceptable GPS accuracy.
const MaxAllowedAccuracyMeters = 50.0

// --- Entities ---

type AttendanceRecord struct {
	ID                  uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	EmployeeID          uuid.UUID  `gorm:"type:uuid;not null"`
	CompanyID           uuid.UUID  `gorm:"type:uuid;not null"`
	WorkDate            time.Time  `gorm:"type:date;not null"`
	Status              string     `gorm:"type:varchar(20);not null;default:'clocked_in'"`
	ClockInAt           time.Time  `gorm:"not null"`
	ClockOutAt          *time.Time
	ClockInLatitude     float64    `gorm:"not null"`
	ClockInLongitude    float64    `gorm:"not null"`
	SelfieClockInURL    string     `gorm:"type:text;not null"`
	Notes               *string    `gorm:"type:text"`
	ScheduledClockInAt  *time.Time
	ScheduledClockOutAt *time.Time
	WorkingMinutes      *int
	OvertimeMinutes     *int
	CreatedAt           time.Time  `gorm:"not null;default:now()"`
	UpdatedAt           time.Time  `gorm:"not null;default:now()"`
}

func (AttendanceRecord) TableName() string { return "attendance_records" }

type AttendanceBreak struct {
	ID                 uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	AttendanceRecordID uuid.UUID  `gorm:"type:uuid;not null"`
	BreakStartAt       time.Time  `gorm:"not null"`
	BreakEndAt         *time.Time
	CreatedAt          time.Time  `gorm:"not null;default:now()"`
}

func (AttendanceBreak) TableName() string { return "attendance_breaks" }

type EmployeeSchedule struct {
	ID           uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	EmployeeID   uuid.UUID `gorm:"type:uuid;not null"`
	CompanyID    uuid.UUID `gorm:"type:uuid;not null"`
	DayOfWeek    int       `gorm:"type:smallint;not null"`
	ClockInTime  string    `gorm:"type:time;not null"`
	ClockOutTime string    `gorm:"type:time;not null"`
	IsActive     bool      `gorm:"not null;default:true"`
}

func (EmployeeSchedule) TableName() string { return "employee_schedules" }

// --- Repository Interfaces ---

type AttendanceRepository interface {
	CreateClockIn(ctx context.Context, record *AttendanceRecord) error
	GetTodayRecord(ctx context.Context, employeeID uuid.UUID, date time.Time) (*AttendanceRecord, error)
	UpdateClockOut(ctx context.Context, record *AttendanceRecord) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
}

type AttendanceBreakRepository interface {
	StartBreak(ctx context.Context, b *AttendanceBreak) error
	EndLatestBreak(ctx context.Context, attendanceID uuid.UUID, endTime time.Time) error
	GetOpenBreak(ctx context.Context, attendanceID uuid.UUID) (*AttendanceBreak, error)
	SumBreakMinutes(ctx context.Context, attendanceID uuid.UUID) (int, error)
}

type EmployeeScheduleRepository interface {
	GetByDayOfWeek(ctx context.Context, employeeID uuid.UUID, day int) (*EmployeeSchedule, error)
}

// --- Usecase Interface ---

type AttendanceUsecase interface {
	ValidateLocation(ctx context.Context, req *ValidateLocationRequest) (*ValidateLocationResponse, error)
	ClockIn(ctx context.Context, req *ClockInRequest) (*ClockInResponse, error)
	ToggleBreak(ctx context.Context, req *BreakRequest) (*BreakResponse, error)
	GetClockOutPreview(ctx context.Context, employeeID string, companyID uuid.UUID) (*ClockOutPreview, error)
	ClockOut(ctx context.Context, employeeID string, companyID uuid.UUID) (*ClockOutResponse, error)
	GetTodayStatus(ctx context.Context, employeeID string) (*TodayStatusResponse, error)
	RegisterSelfie(ctx context.Context, employeeID string, req *RegisterSelfieRequest) error
	GetRegisteredSelfie(ctx context.Context, employeeID string) (*SelfieStatusResponse, error)
}

// --- DTOs ---

type ValidateLocationRequest struct {
	Latitude  *float64 `json:"latitude"  binding:"required"`
	Longitude *float64 `json:"longitude" binding:"required"`

	AccuracyMeters float64 `json:"accuracy_meters" binding:"required,gt=0"`

	IsMockLocation bool `json:"is_mock_location"`

	CompanyID string // Injected by handler from JWT claims, NOT from JSON body
}

type ValidateLocationResponse struct {
	IsInsideGeofence    bool    `json:"is_inside_geofence"`
	DistanceMeters      float64 `json:"distance_meters"`
	AllowedRadiusMeters float64 `json:"allowed_radius_meters"`
}

type ClockInRequest struct {
	SelfieURL  string   `json:"selfie_url"  binding:"required"`
	Latitude   *float64 `json:"latitude"    binding:"required"`
	Longitude  *float64 `json:"longitude"   binding:"required"`
	Notes      *string  `json:"notes"`
	EmployeeID string
	CompanyID  uuid.UUID
}

type ClockInResponse struct {
	ID        uuid.UUID `json:"id"`
	ClockInAt time.Time `json:"clock_in_at"`
	Status    string    `json:"status"`
}

type BreakRequest struct {
	Action     string `json:"action" binding:"required"`
	EmployeeID string
}

type BreakResponse struct {
	Action    string    `json:"action"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
}

type ClockOutPreview struct {
	WorkingMinutes      int        `json:"working_minutes"`
	OvertimeMinutes     int        `json:"overtime_minutes"`
	ScheduledClockOutAt *time.Time `json:"scheduled_clock_out_at"`
	CurrentTime         time.Time  `json:"current_time"`
}

type ClockOutResponse struct {
	ClockOutAt     time.Time `json:"clock_out_at"`
	WorkingMinutes int       `json:"working_minutes"`
	OvertimeMinutes int      `json:"overtime_minutes"`
	Status         string    `json:"status"`
}

type TodayStatusResponse struct {
	Status            string     `json:"status"`
	AttendanceID      *uuid.UUID `json:"attendance_id,omitempty"`
	ClockInAt         *time.Time `json:"clock_in_at,omitempty"`
	ClockOutAt        *time.Time `json:"clock_out_at,omitempty"`
	IsOnBreak         bool       `json:"is_on_break,omitempty"`
	OpenBreakStartAt  *time.Time `json:"open_break_start_at,omitempty"`
	Notes             *string    `json:"notes,omitempty"`
}

type RegisterSelfieRequest struct {
	SelfieURL string `json:"selfie_url" binding:"required,url"`
}

type SelfieStatusResponse struct {
	SelfieURL    string    `json:"selfie_url"`
	RegisteredAt time.Time `json:"registered_at"`
}
