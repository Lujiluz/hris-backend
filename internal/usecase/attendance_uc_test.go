package usecase_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"hris-backend/internal/domain"
	"hris-backend/internal/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mock repos for attendance ---
// MockEmployeeRepo and MockCompanyRepo are already defined in employee_uc_test.go
// in the same package; do not redeclare them here.

type MockAttendanceRepo struct{ mock.Mock }

func (m *MockAttendanceRepo) CreateClockIn(ctx context.Context, r *domain.AttendanceRecord) error {
	return m.Called(ctx, r).Error(0)
}
func (m *MockAttendanceRepo) GetTodayRecord(ctx context.Context, employeeID uuid.UUID, date time.Time) (*domain.AttendanceRecord, error) {
	args := m.Called(ctx, employeeID, date)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AttendanceRecord), args.Error(1)
}
func (m *MockAttendanceRepo) UpdateClockOut(ctx context.Context, record *domain.AttendanceRecord) error {
	return m.Called(ctx, record).Error(0)
}
func (m *MockAttendanceRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	return m.Called(ctx, id, status).Error(0)
}

type MockBreakRepo struct{ mock.Mock }

func (m *MockBreakRepo) StartBreak(ctx context.Context, b *domain.AttendanceBreak) error {
	return m.Called(ctx, b).Error(0)
}
func (m *MockBreakRepo) EndLatestBreak(ctx context.Context, attendanceID uuid.UUID, endTime time.Time) error {
	return m.Called(ctx, attendanceID, endTime).Error(0)
}
func (m *MockBreakRepo) GetOpenBreak(ctx context.Context, attendanceID uuid.UUID) (*domain.AttendanceBreak, error) {
	args := m.Called(ctx, attendanceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AttendanceBreak), args.Error(1)
}
func (m *MockBreakRepo) SumBreakMinutes(ctx context.Context, attendanceID uuid.UUID) (int, error) {
	args := m.Called(ctx, attendanceID)
	return args.Int(0), args.Error(1)
}

type MockScheduleRepo struct{ mock.Mock }

func (m *MockScheduleRepo) GetByDayOfWeek(ctx context.Context, employeeID uuid.UUID, day int) (*domain.EmployeeSchedule, error) {
	args := m.Called(ctx, employeeID, day)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EmployeeSchedule), args.Error(1)
}

// --- Helpers ---

func makeClockOutUsecase(empRepo *MockEmployeeRepo, attRepo *MockAttendanceRepo, brRepo *MockBreakRepo) domain.AttendanceUsecase {
	return usecase.NewAttendanceUsecase(empRepo, new(MockCompanyRepo), attRepo, brRepo, new(MockScheduleRepo))
}

const testEmployeeCode = "GOTO-2026-0001"

func setupClockOutMocks(empRepo *MockEmployeeRepo, attRepo *MockAttendanceRepo, brRepo *MockBreakRepo, clockInAt time.Time) *domain.AttendanceRecord {
	empUUID := uuid.New()
	emp := &domain.Employee{ID: empUUID.String(), EmployeeID: testEmployeeCode}
	record := &domain.AttendanceRecord{
		ID:         uuid.New(),
		EmployeeID: empUUID,
		Status:     domain.AttendanceStatusClockedIn,
		ClockInAt:  clockInAt,
	}

	empRepo.On("GetByEmployeeID", mock.Anything, testEmployeeCode).Return(emp, nil)
	// Use mock.AnythingOfType for the date argument so tests are timezone-safe.
	attRepo.On("GetTodayRecord", mock.Anything, empUUID, mock.AnythingOfType("time.Time")).Return(record, nil)
	brRepo.On("SumBreakMinutes", mock.Anything, record.ID).Return(0, nil)
	attRepo.On("UpdateClockOut", mock.Anything, mock.AnythingOfType("*domain.AttendanceRecord")).Return(nil)
	return record
}

// --- Tests ---

func TestClockOut_NoClientTimestamp_UsesServerTime(t *testing.T) {
	empRepo := new(MockEmployeeRepo)
	attRepo := new(MockAttendanceRepo)
	brRepo := new(MockBreakRepo)

	setupClockOutMocks(empRepo, attRepo, brRepo, time.Now().Add(-4*time.Hour))

	uc := makeClockOutUsecase(empRepo, attRepo, brRepo)
	resp, err := uc.ClockOut(context.Background(), testEmployeeCode, uuid.New(), domain.ClockOutRequest{})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.IsOfflineSubmission)
}

func TestClockOut_ValidClientTimestamp_UsesClientTime(t *testing.T) {
	empRepo := new(MockEmployeeRepo)
	attRepo := new(MockAttendanceRepo)
	brRepo := new(MockBreakRepo)

	clientTime := time.Now().Add(-2 * time.Hour)
	clientTS := clientTime.Format(time.RFC3339)
	setupClockOutMocks(empRepo, attRepo, brRepo, time.Now().Add(-4*time.Hour))

	uc := makeClockOutUsecase(empRepo, attRepo, brRepo)
	resp, err := uc.ClockOut(context.Background(), testEmployeeCode, uuid.New(), domain.ClockOutRequest{
		ClientTimestamp: &clientTS,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.IsOfflineSubmission)
	// ClockOutAt must match the client timestamp (within 1s for RFC3339 second truncation).
	assert.WithinDuration(t, clientTime, resp.ClockOutAt, time.Second)
}

func TestClockOut_InvalidRFC3339_ReturnsError(t *testing.T) {
	uc := makeClockOutUsecase(new(MockEmployeeRepo), new(MockAttendanceRepo), new(MockBreakRepo))
	bad := "not-a-timestamp"
	_, err := uc.ClockOut(context.Background(), testEmployeeCode, uuid.New(), domain.ClockOutRequest{
		ClientTimestamp: &bad,
	})
	assert.ErrorIs(t, err, domain.ErrInvalidClientTimestamp)
}

func TestClockOut_FutureTimestamp_ReturnsError(t *testing.T) {
	uc := makeClockOutUsecase(new(MockEmployeeRepo), new(MockAttendanceRepo), new(MockBreakRepo))
	future := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
	_, err := uc.ClockOut(context.Background(), testEmployeeCode, uuid.New(), domain.ClockOutRequest{
		ClientTimestamp: &future,
	})
	assert.ErrorIs(t, err, domain.ErrClientTimestampInFuture)
}

func TestClockOut_TooOldTimestamp_ReturnsError(t *testing.T) {
	uc := makeClockOutUsecase(new(MockEmployeeRepo), new(MockAttendanceRepo), new(MockBreakRepo))
	tooOld := time.Now().Add(-(domain.MaxOfflineDuration + time.Minute)).Format(time.RFC3339)
	_, err := uc.ClockOut(context.Background(), testEmployeeCode, uuid.New(), domain.ClockOutRequest{
		ClientTimestamp: &tooOld,
	})
	assert.ErrorIs(t, err, domain.ErrClientTimestampTooOld)
}

func TestClockOut_NotesPassedThrough(t *testing.T) {
	empRepo := new(MockEmployeeRepo)
	attRepo := new(MockAttendanceRepo)
	brRepo := new(MockBreakRepo)

	setupClockOutMocks(empRepo, attRepo, brRepo, time.Now().Add(-4*time.Hour))

	uc := makeClockOutUsecase(empRepo, attRepo, brRepo)
	note := "leaving early"
	resp, err := uc.ClockOut(context.Background(), testEmployeeCode, uuid.New(), domain.ClockOutRequest{
		Notes: &note,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// Verify UpdateClockOut was called with a record that has Notes set to our value.
	attRepo.AssertCalled(t, "UpdateClockOut", mock.Anything, mock.MatchedBy(func(r *domain.AttendanceRecord) bool {
		return r.Notes != nil && *r.Notes == note
	}))
}

func TestClockOut_MaxOfflineDurationBoundary_Succeeds(t *testing.T) {
	empRepo := new(MockEmployeeRepo)
	attRepo := new(MockAttendanceRepo)
	brRepo := new(MockBreakRepo)

	// One minute inside the allowed window should succeed.
	clientTime := time.Now().Add(-(domain.MaxOfflineDuration - time.Minute))
	clientTS := clientTime.Format(time.RFC3339)
	setupClockOutMocks(empRepo, attRepo, brRepo, clientTime.Add(-4*time.Hour))

	uc := makeClockOutUsecase(empRepo, attRepo, brRepo)
	resp, err := uc.ClockOut(context.Background(), testEmployeeCode, uuid.New(), domain.ClockOutRequest{
		ClientTimestamp: &clientTS,
	})
	assert.NoError(t, err)
	assert.True(t, resp.IsOfflineSubmission)

	fmt.Println("MaxOfflineDuration boundary test passed")
}
