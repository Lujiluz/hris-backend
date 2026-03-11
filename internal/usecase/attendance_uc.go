package usecase

import (
	"context"
	"hris-backend/internal/domain"
	"hris-backend/pkg/geo"
	"time"

	"github.com/google/uuid"
)

type attendanceUsecase struct {
	empRepo        domain.EmployeeRepository
	companyRepo    domain.CompanyRepository
	attendanceRepo domain.AttendanceRepository
	breakRepo      domain.AttendanceBreakRepository
	scheduleRepo   domain.EmployeeScheduleRepository
}

func NewAttendanceUsecase(
	empRepo domain.EmployeeRepository,
	companyRepo domain.CompanyRepository,
	attendanceRepo domain.AttendanceRepository,
	breakRepo domain.AttendanceBreakRepository,
	scheduleRepo domain.EmployeeScheduleRepository,
) domain.AttendanceUsecase {
	return &attendanceUsecase{
		empRepo:        empRepo,
		companyRepo:    companyRepo,
		attendanceRepo: attendanceRepo,
		breakRepo:      breakRepo,
		scheduleRepo:   scheduleRepo,
	}
}

// resolveEmployeeUUID looks up the employee's internal UUID from the public EmployeeID code.
func (uc *attendanceUsecase) resolveEmployeeUUID(ctx context.Context, employeeCode string) (uuid.UUID, error) {
	emp, err := uc.empRepo.GetByEmployeeID(ctx, employeeCode)
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(emp.ID)
}

func (uc *attendanceUsecase) ValidateLocation(ctx context.Context, req *domain.ValidateLocationRequest) (*domain.ValidateLocationResponse, error) {
	if req.IsMockLocation {
		return nil, domain.ErrMockLocationDetected
	}

	if req.AccuracyMeters > domain.MaxAllowedAccuracyMeters {
		return nil, domain.ErrGPSAccuracyTooLow
	}

	company, err := uc.companyRepo.GetByID(ctx, req.CompanyID)
	if err != nil {
		return nil, err
	}

	if company.OfficeLatitude == nil || company.OfficeLongitude == nil || company.OfficeRadiusMeters == nil {
		return nil, domain.ErrOfficeNotConfigured
	}

	distance := geo.Haversine(*req.Latitude, *req.Longitude, *company.OfficeLatitude, *company.OfficeLongitude)

	return &domain.ValidateLocationResponse{
		IsInsideGeofence:    distance <= *company.OfficeRadiusMeters,
		DistanceMeters:      distance,
		AllowedRadiusMeters: *company.OfficeRadiusMeters,
	}, nil
}

func (uc *attendanceUsecase) ClockIn(ctx context.Context, req *domain.ClockInRequest) (*domain.ClockInResponse, error) {
	empUUID, err := uc.resolveEmployeeUUID(ctx, req.EmployeeID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	existing, err := uc.attendanceRepo.GetTodayRecord(ctx, empUUID, today)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, domain.ErrAlreadyClockedIn
	}

	// Snapshot schedule for today
	schedule, err := uc.scheduleRepo.GetByDayOfWeek(ctx, empUUID, int(now.Weekday()))
	if err != nil {
		return nil, err
	}

	record := &domain.AttendanceRecord{
		ID:               uuid.New(),
		EmployeeID:       empUUID,
		CompanyID:        req.CompanyID,
		WorkDate:         today,
		Status:           "clocked_in",
		ClockInAt:        now,
		ClockInLatitude:  *req.Latitude,
		ClockInLongitude: *req.Longitude,
		SelfieClockInURL: req.SelfieURL,
		Notes:            req.Notes,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if schedule != nil {
		parseHM := func(s string) (int, int) {
			t, err := time.Parse("15:04:05", s)
			if err != nil {
				return 0, 0
			}
			return t.Hour(), t.Minute()
		}
		inH, inM := parseHM(schedule.ClockInTime)
		outH, outM := parseHM(schedule.ClockOutTime)
		scheduledIn := time.Date(now.Year(), now.Month(), now.Day(), inH, inM, 0, 0, now.Location())
		scheduledOut := time.Date(now.Year(), now.Month(), now.Day(), outH, outM, 0, 0, now.Location())
		record.ScheduledClockInAt = &scheduledIn
		record.ScheduledClockOutAt = &scheduledOut
	}

	if err := uc.attendanceRepo.CreateClockIn(ctx, record); err != nil {
		return nil, err
	}

	return &domain.ClockInResponse{
		ID:        record.ID,
		ClockInAt: record.ClockInAt,
		Status:    record.Status,
	}, nil
}

func (uc *attendanceUsecase) ToggleBreak(ctx context.Context, req *domain.BreakRequest) (*domain.BreakResponse, error) {
	if req.Action != "start" && req.Action != "end" {
		return nil, domain.ErrInvalidBreakAction
	}

	empUUID, err := uc.resolveEmployeeUUID(ctx, req.EmployeeID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	record, err := uc.attendanceRepo.GetTodayRecord(ctx, empUUID, today)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, domain.ErrNotClockedIn
	}
	if record.Status == "clocked_out" {
		return nil, domain.ErrAlreadyClockedOut
	}

	if req.Action == "start" {
		if record.Status == "on_break" {
			return nil, domain.ErrAlreadyOnBreak
		}
		b := &domain.AttendanceBreak{
			ID:                 uuid.New(),
			AttendanceRecordID: record.ID,
			BreakStartAt:       now,
			CreatedAt:          now,
		}
		if err := uc.breakRepo.StartBreak(ctx, b); err != nil {
			return nil, err
		}
		if err := uc.attendanceRepo.UpdateStatus(ctx, record.ID, "on_break"); err != nil {
			return nil, err
		}
		return &domain.BreakResponse{Action: "start", Timestamp: now, Status: "on_break"}, nil
	}

	// action == "end"
	openBreak, err := uc.breakRepo.GetOpenBreak(ctx, record.ID)
	if err != nil {
		return nil, err
	}
	if openBreak == nil {
		return nil, domain.ErrNotOnBreak
	}
	if err := uc.breakRepo.EndLatestBreak(ctx, record.ID, now); err != nil {
		return nil, err
	}
	if err := uc.attendanceRepo.UpdateStatus(ctx, record.ID, "clocked_in"); err != nil {
		return nil, err
	}
	return &domain.BreakResponse{Action: "end", Timestamp: now, Status: "clocked_in"}, nil
}

func (uc *attendanceUsecase) GetClockOutPreview(ctx context.Context, employeeID string, companyID uuid.UUID) (*domain.ClockOutPreview, error) {
	empUUID, err := uc.resolveEmployeeUUID(ctx, employeeID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	record, err := uc.attendanceRepo.GetTodayRecord(ctx, empUUID, today)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, domain.ErrNotClockedIn
	}
	if record.Status == "clocked_out" {
		return nil, domain.ErrAlreadyClockedOut
	}

	breakMinutes, err := uc.breakRepo.SumBreakMinutes(ctx, record.ID)
	if err != nil {
		return nil, err
	}

	// If on break, include open break duration in the sum
	if record.Status == "on_break" {
		openBreak, err := uc.breakRepo.GetOpenBreak(ctx, record.ID)
		if err != nil {
			return nil, err
		}
		if openBreak != nil {
			breakMinutes += int(now.Sub(openBreak.BreakStartAt).Minutes())
		}
	}

	totalMinutes := int(now.Sub(record.ClockInAt).Minutes()) - breakMinutes
	if totalMinutes < 0 {
		totalMinutes = 0
	}

	overtimeMinutes := 0
	if record.ScheduledClockOutAt != nil && now.After(*record.ScheduledClockOutAt) {
		overtimeMinutes = int(now.Sub(*record.ScheduledClockOutAt).Minutes())
	}

	return &domain.ClockOutPreview{
		WorkingMinutes:      totalMinutes,
		OvertimeMinutes:     overtimeMinutes,
		ScheduledClockOutAt: record.ScheduledClockOutAt,
		CurrentTime:         now,
	}, nil
}

func (uc *attendanceUsecase) ClockOut(ctx context.Context, employeeID string, companyID uuid.UUID) (*domain.ClockOutResponse, error) {
	empUUID, err := uc.resolveEmployeeUUID(ctx, employeeID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	record, err := uc.attendanceRepo.GetTodayRecord(ctx, empUUID, today)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, domain.ErrNotClockedIn
	}
	if record.Status == "clocked_out" {
		return nil, domain.ErrAlreadyClockedOut
	}

	// Auto-end open break if on break
	if record.Status == "on_break" {
		if err := uc.breakRepo.EndLatestBreak(ctx, record.ID, now); err != nil {
			return nil, err
		}
	}

	breakMinutes, err := uc.breakRepo.SumBreakMinutes(ctx, record.ID)
	if err != nil {
		return nil, err
	}

	workingMinutes := int(now.Sub(record.ClockInAt).Minutes()) - breakMinutes
	if workingMinutes < 0 {
		workingMinutes = 0
	}

	overtimeMinutes := 0
	if record.ScheduledClockOutAt != nil && now.After(*record.ScheduledClockOutAt) {
		overtimeMinutes = int(now.Sub(*record.ScheduledClockOutAt).Minutes())
	}

	record.ClockOutAt = &now
	record.Status = "clocked_out"
	record.WorkingMinutes = &workingMinutes
	record.OvertimeMinutes = &overtimeMinutes

	if err := uc.attendanceRepo.UpdateClockOut(ctx, record); err != nil {
		return nil, err
	}

	return &domain.ClockOutResponse{
		ClockOutAt:      now,
		WorkingMinutes:  workingMinutes,
		OvertimeMinutes: overtimeMinutes,
		Status:          "clocked_out",
	}, nil
}

func (uc *attendanceUsecase) RegisterSelfie(ctx context.Context, employeeID string, req *domain.RegisterSelfieRequest) error {
	emp, err := uc.empRepo.GetByEmployeeID(ctx, employeeID)
	if err != nil {
		return err
	}
	if emp.SelfieURL != nil && *emp.SelfieURL != "" {
		return domain.ErrSelfieAlreadyRegistered
	}
	return uc.empRepo.RegisterSelfie(ctx, employeeID, req.SelfieURL)
}

func (uc *attendanceUsecase) GetRegisteredSelfie(ctx context.Context, employeeID string) (*domain.SelfieStatusResponse, error) {
	emp, err := uc.empRepo.GetByEmployeeID(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	if emp.SelfieURL == nil || *emp.SelfieURL == "" {
		return nil, domain.ErrSelfieNotRegistered
	}
	return &domain.SelfieStatusResponse{
		SelfieURL:    *emp.SelfieURL,
		RegisteredAt: *emp.SelfieRegisteredAt,
	}, nil
}

func (uc *attendanceUsecase) GetTodayStatus(ctx context.Context, employeeID string) (*domain.TodayStatusResponse, error) {
	empUUID, err := uc.resolveEmployeeUUID(ctx, employeeID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	record, err := uc.attendanceRepo.GetTodayRecord(ctx, empUUID, today)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return &domain.TodayStatusResponse{Status: "idle"}, nil
	}

	resp := &domain.TodayStatusResponse{
		Status:       record.Status,
		AttendanceID: &record.ID,
		ClockInAt:    &record.ClockInAt,
		ClockOutAt:   record.ClockOutAt,
		Notes:        record.Notes,
	}

	if record.Status == "on_break" {
		resp.IsOnBreak = true
		openBreak, err := uc.breakRepo.GetOpenBreak(ctx, record.ID)
		if err != nil {
			return nil, err
		}
		if openBreak != nil {
			resp.OpenBreakStartAt = &openBreak.BreakStartAt
		}
	}

	return resp, nil
}
