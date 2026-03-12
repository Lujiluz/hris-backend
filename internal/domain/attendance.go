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
	ErrInvalidBreakAction      = errors.New("invalid break action, must be 'start' or 'end'")
	ErrClientTimestampInFuture = errors.New("client_timestamp is in the future")
	ErrClientTimestampTooOld   = errors.New("client_timestamp exceeds max offline duration")
	ErrInvalidClientTimestamp  = errors.New("invalid client_timestamp format, expected RFC3339")
)

// MaxAllowedAccuracyMeters is the worst acceptable GPS accuracy.
const MaxAllowedAccuracyMeters = 50.0

// MaxOfflineDuration is the maximum age of a client_timestamp accepted for offline clock-out.
const MaxOfflineDuration = 24 * time.Hour

// Attendance status constants
const (
	AttendanceStatusIdle      = "idle"       // No attendance record for today
	AttendanceStatusClockedIn = "clocked_in" // Employee is actively working
	AttendanceStatusOnBreak   = "on_break"   // Employee is on break
	AttendanceStatusClockedOut = "clocked_out" // Employee has clocked out
)

// Break action constants
const (
	BreakActionStart = "start"
	BreakActionEnd   = "end"
)

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
	IsOfflineSubmission bool       `json:"is_offline_submission" gorm:"column:is_offline_submission;not null;default:false"`
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
	ClockOut(ctx context.Context, employeeID string, companyID uuid.UUID, req ClockOutRequest) (*ClockOutResponse, error)
	GetTodayStatus(ctx context.Context, employeeID string) (*TodayStatusResponse, error)
	RegisterSelfie(ctx context.Context, employeeID string, req *RegisterSelfieRequest) error
	GetRegisteredSelfie(ctx context.Context, employeeID string) (*SelfieStatusResponse, error)
}

// --- DTOs ---

type ValidateLocationRequest struct {
	Latitude  *float64 `json:"latitude"  binding:"required" example:"-6.2088"`
	Longitude *float64 `json:"longitude" binding:"required" example:"106.8456"`

	AccuracyMeters float64 `json:"accuracy_meters" binding:"required,gt=0" example:"10.5"`

	IsMockLocation bool `json:"is_mock_location" example:"false"`

	CompanyID string `json:"-" swaggerignore:"true"` // Injected by handler from JWT claims
}

type ValidateLocationResponse struct {
	IsInsideGeofence    bool    `json:"is_inside_geofence" example:"true"`
	DistanceMeters      float64 `json:"distance_meters" example:"45.2"`
	AllowedRadiusMeters float64 `json:"allowed_radius_meters" example:"100"`
}

type ClockInRequest struct {
	SelfieURL  string   `json:"selfie_url"  binding:"required" example:"https://res.cloudinary.com/xxx/image/upload/selfie.jpg"`
	Latitude   *float64 `json:"latitude"    binding:"required" example:"-6.2088"`
	Longitude  *float64 `json:"longitude"   binding:"required" example:"106.8456"`
	Notes      *string  `json:"notes" example:"Working from office today"`
	EmployeeID string   `json:"-" swaggerignore:"true"`
	CompanyID  uuid.UUID `json:"-" swaggerignore:"true"`
}

type ClockInResponse struct {
	ID        uuid.UUID `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	ClockInAt time.Time `json:"clock_in_at" example:"2026-03-12T08:00:00Z"`
	Status    string    `json:"status" example:"clocked_in" enums:"clocked_in"`
}

type BreakRequest struct {
	Action     string `json:"action" binding:"required" example:"start" enums:"start,end"`
	EmployeeID string `json:"-" swaggerignore:"true"`
}

type BreakResponse struct {
	Action    string    `json:"action" example:"start" enums:"start,end"`
	Timestamp time.Time `json:"timestamp" example:"2026-03-12T12:00:00Z"`
	Status    string    `json:"status" example:"on_break" enums:"clocked_in,on_break"`
}

type ClockOutRequest struct {
	ClientTimestamp *string `json:"client_timestamp"`
	Notes           *string `json:"notes"`
}

type ClockOutPreview struct {
	WorkingMinutes      int        `json:"working_minutes" example:"480"`
	OvertimeMinutes     int        `json:"overtime_minutes" example:"30"`
	ScheduledClockOutAt *time.Time `json:"scheduled_clock_out_at" example:"2026-03-12T17:00:00Z"`
	CurrentTime         time.Time  `json:"current_time" example:"2026-03-12T17:30:00Z"`
}

type ClockOutResponse struct {
	ClockOutAt          time.Time `json:"clock_out_at" example:"2026-03-12T17:30:00Z"`
	WorkingMinutes      int       `json:"working_minutes" example:"480"`
	OvertimeMinutes     int       `json:"overtime_minutes" example:"30"`
	Status              string    `json:"status" example:"clocked_out" enums:"clocked_out"`
	IsOfflineSubmission bool      `json:"is_offline_submission" example:"false"`
}

type TodayStatusResponse struct {
	// Status represents the current attendance state of the employee for today.
	Status           string     `json:"status" example:"clocked_in" enums:"idle,clocked_in,on_break,clocked_out"`
	AttendanceID     *uuid.UUID `json:"attendance_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	ClockInAt        *time.Time `json:"clock_in_at,omitempty" example:"2026-03-12T08:00:00Z"`
	ClockOutAt       *time.Time `json:"clock_out_at,omitempty" example:"2026-03-12T17:00:00Z"`
	IsOnBreak        bool       `json:"is_on_break,omitempty" example:"false"`
	OpenBreakStartAt *time.Time `json:"open_break_start_at,omitempty" example:"2026-03-12T12:00:00Z"`
	Notes            *string    `json:"notes,omitempty" example:"Working from office today"`
}

type RegisterSelfieRequest struct {
	SelfieURL string `json:"selfie_url" binding:"required,url" example:"https://res.cloudinary.com/xxx/image/upload/selfie.jpg"`
}

type SelfieStatusResponse struct {
	SelfieURL    string    `json:"selfie_url" example:"https://res.cloudinary.com/xxx/image/upload/selfie.jpg"`
	RegisteredAt time.Time `json:"registered_at" example:"2026-03-12T08:00:00Z"`
}
