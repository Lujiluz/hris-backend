# Offline Clock-Out Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `GET /api/v1/time` server-time endpoint and update `POST /api/v1/attendance/clock-out` to accept an optional `client_timestamp` for offline-first clock-out.

**Architecture:** Domain-first: add types/DTOs/errors without touching the interface yet; then in one atomic TDD task update the interface, usecase, and repository together so the codebase never has a non-compiling intermediate state; then wire a new `UtilityHandler` and update the attendance handler.

**Tech Stack:** Go 1.25, Gin, GORM/PostgreSQL, testify/mock (unit tests), httptest (integration tests), golang-migrate (SQL migrations)

---

## Chunk 1: Foundation — Migration + Domain Types

### Task 1: Create migration files

**Files:**
- Create: `migrations/000012_add_offline_clockout.up.sql`
- Create: `migrations/000012_add_offline_clockout.down.sql`

- [ ] **Step 1: Write up migration**

  File: `migrations/000012_add_offline_clockout.up.sql`
  ```sql
  ALTER TABLE attendance_records
    ADD COLUMN is_offline_submission BOOLEAN NOT NULL DEFAULT FALSE;
  ```

- [ ] **Step 2: Write down migration**

  File: `migrations/000012_add_offline_clockout.down.sql`
  ```sql
  ALTER TABLE attendance_records DROP COLUMN is_offline_submission;
  ```

- [ ] **Step 3: Commit**

  ```bash
  git add migrations/000012_add_offline_clockout.up.sql migrations/000012_add_offline_clockout.down.sql
  git commit -m "feat(migration): add is_offline_submission to attendance_records"
  ```

---

### Task 2: Add domain types (no interface change yet)

**Files:**
- Modify: `internal/domain/attendance.go`

> **Important:** The `AttendanceUsecase` interface is **not** changed in this task. That happens atomically in Task 3 together with the usecase and repo updates, to avoid a non-compiling intermediate state.

- [ ] **Step 1: Add constant**

  In `internal/domain/attendance.go`, add after the `MaxAllowedAccuracyMeters` const line:

  ```go
  // MaxOfflineDuration is the maximum age of a client_timestamp accepted for offline clock-out.
  const MaxOfflineDuration = 24 * time.Hour
  ```

- [ ] **Step 2: Add new error variables**

  In the existing `var (...)` error block, add:
  ```go
  ErrClientTimestampInFuture = errors.New("client_timestamp is in the future")
  ErrClientTimestampTooOld   = errors.New("client_timestamp exceeds max offline duration")
  ErrInvalidClientTimestamp  = errors.New("invalid client_timestamp format, expected RFC3339")
  ```

- [ ] **Step 3: Add `IsOfflineSubmission` field to `AttendanceRecord`**

  In `AttendanceRecord`, add after the `OvertimeMinutes` field:
  ```go
  IsOfflineSubmission bool `json:"is_offline_submission" gorm:"column:is_offline_submission;not null;default:false"`
  ```

- [ ] **Step 4: Add `ClockOutRequest` DTO**

  Add this DTO after `BreakResponse`:
  ```go
  type ClockOutRequest struct {
  	ClientTimestamp *string `json:"client_timestamp"`
  	Notes           *string `json:"notes"`
  }
  ```

- [ ] **Step 5: Update `ClockOutResponse` to add `IsOfflineSubmission`**

  Replace the existing `ClockOutResponse` struct:
  ```go
  type ClockOutResponse struct {
  	ClockOutAt          time.Time `json:"clock_out_at" example:"2026-03-12T17:30:00Z"`
  	WorkingMinutes      int       `json:"working_minutes" example:"480"`
  	OvertimeMinutes     int       `json:"overtime_minutes" example:"30"`
  	Status              string    `json:"status" example:"clocked_out" enums:"clocked_out"`
  	IsOfflineSubmission bool      `json:"is_offline_submission" example:"false"`
  }
  ```

- [ ] **Step 6: Verify the codebase still compiles (interface not changed yet)**

  ```bash
  go build ./...
  ```
  Expected: clean build — no errors. The interface and all callers are still using the old signature.

- [ ] **Step 7: Commit**

  ```bash
  git add internal/domain/attendance.go
  git commit -m "feat(domain): add offline clock-out types, errors, and updated ClockOutResponse"
  ```

---

## Chunk 2: TDD — Tests + Interface + Usecase + Repository (one atomic change)

### Task 3: Write tests, update interface, implement usecase, extend repository — all in one commit

These four changes are kept in a single commit because:
- The interface change in `domain/attendance.go` immediately breaks `attendance_handler.go`'s call to `ClockOut` with the old arity.
- The usecase must satisfy the new interface signature.
- The repository map must include `is_offline_submission`; if the usecase sets it on the struct but the repo map omits it, GORM silently drops it.

All are committed together to keep the codebase in a valid, compiling state at every commit boundary.

**Files:**
- Create: `internal/usecase/attendance_uc_test.go`
- Modify: `internal/domain/attendance.go` (interface only)
- Modify: `internal/usecase/attendance_uc.go`
- Modify: `internal/repository/postgres/attendance_repo.go`

- [ ] **Step 1: Create unit test file**

  File: `internal/usecase/attendance_uc_test.go`

  ```go
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
  	emp := &domain.Employee{ID: empUUID, EmployeeID: testEmployeeCode}
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
  ```

- [ ] **Step 2: Attempt to run tests — expect compile error**

  ```bash
  go test ./internal/usecase/ -run TestClockOut 2>&1 | head -20
  ```
  Expected: compile error because the `AttendanceUsecase` interface `ClockOut` method still has the old arity (3 args), but tests call it with 4 args. This confirms the test is correctly targeting the new interface.

- [ ] **Step 3: Update `AttendanceUsecase` interface in `domain/attendance.go`**

  Change:
  ```go
  ClockOut(ctx context.Context, employeeID string, companyID uuid.UUID) (*ClockOutResponse, error)
  ```
  To:
  ```go
  ClockOut(ctx context.Context, employeeID string, companyID uuid.UUID, req ClockOutRequest) (*ClockOutResponse, error)
  ```

  > **Note:** This change also makes `attendance_handler.go` fail to compile (it still calls `ClockOut` with the old 3-arg form). Do not run `go build ./...` until the handler is updated in Step 6 below.

- [ ] **Step 4: Replace `ClockOut` in `internal/usecase/attendance_uc.go`**

  Find the existing `ClockOut` method and replace it entirely:

  ```go
  func (uc *attendanceUsecase) ClockOut(ctx context.Context, employeeID string, companyID uuid.UUID, req domain.ClockOutRequest) (*domain.ClockOutResponse, error) {
  	empUUID, err := uc.resolveEmployeeUUID(ctx, employeeID)
  	if err != nil {
  		return nil, err
  	}

  	now := time.Now()
  	clockOutAt := now
  	isOffline := false

  	if req.ClientTimestamp != nil {
  		t, err := time.Parse(time.RFC3339, *req.ClientTimestamp)
  		if err != nil {
  			return nil, domain.ErrInvalidClientTimestamp
  		}
  		if t.After(now) {
  			return nil, domain.ErrClientTimestampInFuture
  		}
  		if now.Sub(t) > domain.MaxOfflineDuration {
  			return nil, domain.ErrClientTimestampTooOld
  		}
  		clockOutAt = t
  		isOffline = true
  	}

  	// Derive work date from clockOutAt so midnight-spanning offline submissions find the right record.
  	// Use clockOutAt.Location() — same timezone strategy as the existing code.
  	workDate := time.Date(clockOutAt.Year(), clockOutAt.Month(), clockOutAt.Day(), 0, 0, 0, 0, clockOutAt.Location())

  	record, err := uc.attendanceRepo.GetTodayRecord(ctx, empUUID, workDate)
  	if err != nil {
  		return nil, err
  	}
  	if record == nil {
  		return nil, domain.ErrNotClockedIn
  	}
  	if record.Status == domain.AttendanceStatusClockedOut {
  		return nil, domain.ErrAlreadyClockedOut
  	}

  	// Auto-end open break. Use clockOutAt so break_end_at <= clock_out_at is always maintained.
  	if record.Status == domain.AttendanceStatusOnBreak {
  		if err := uc.breakRepo.EndLatestBreak(ctx, record.ID, clockOutAt); err != nil {
  			return nil, err
  		}
  	}

  	breakMinutes, err := uc.breakRepo.SumBreakMinutes(ctx, record.ID)
  	if err != nil {
  		return nil, err
  	}

  	workingMinutes := int(clockOutAt.Sub(record.ClockInAt).Minutes()) - breakMinutes
  	if workingMinutes < 0 {
  		workingMinutes = 0
  	}

  	overtimeMinutes := 0
  	if record.ScheduledClockOutAt != nil && clockOutAt.After(*record.ScheduledClockOutAt) {
  		overtimeMinutes = int(clockOutAt.Sub(*record.ScheduledClockOutAt).Minutes())
  	}

  	record.ClockOutAt = &clockOutAt
  	record.Status = domain.AttendanceStatusClockedOut
  	record.WorkingMinutes = &workingMinutes
  	record.OvertimeMinutes = &overtimeMinutes
  	record.IsOfflineSubmission = isOffline
  	if req.Notes != nil {
  		record.Notes = req.Notes // Overwrite note only when explicitly provided; nil = leave unchanged.
  	}

  	if err := uc.attendanceRepo.UpdateClockOut(ctx, record); err != nil {
  		return nil, err
  	}

  	return &domain.ClockOutResponse{
  		ClockOutAt:          clockOutAt,
  		WorkingMinutes:      workingMinutes,
  		OvertimeMinutes:     overtimeMinutes,
  		Status:              domain.AttendanceStatusClockedOut,
  		IsOfflineSubmission: isOffline,
  	}, nil
  }
  ```

- [ ] **Step 5: Extend `UpdateClockOut` map in `internal/repository/postgres/attendance_repo.go`**

  Replace the `UpdateClockOut` function body:

  ```go
  func (r *attendanceRepo) UpdateClockOut(ctx context.Context, record *domain.AttendanceRecord) error {
  	updates := map[string]interface{}{
  		"clock_out_at":          record.ClockOutAt,
  		"status":                record.Status,
  		"working_minutes":       record.WorkingMinutes,
  		"overtime_minutes":      record.OvertimeMinutes,
  		"is_offline_submission": record.IsOfflineSubmission,
  		"updated_at":            time.Now(),
  	}
  	if record.Notes != nil {
  		updates["notes"] = *record.Notes
  	}
  	return r.db.WithContext(ctx).Model(record).Updates(updates).Error
  }
  ```

- [ ] **Step 6: Update `attendance_handler.go` to pass `req` to the usecase**

  This minimal change is needed now to restore compile. The full handler update (new error cases, body binding) happens in Task 5. For now, make it compile by:

  Find this line in the `ClockOut` handler method:
  ```go
  resp, err := h.attendanceUsecase.ClockOut(c.Request.Context(), employeeID, companyID)
  ```

  Replace with:
  ```go
  resp, err := h.attendanceUsecase.ClockOut(c.Request.Context(), employeeID, companyID, domain.ClockOutRequest{})
  ```

  > This passes an empty request (server-time behavior) to restore compilation. Task 5 will add body binding and new error mapping.

- [ ] **Step 7: Run unit tests — expect PASS**

  ```bash
  go test -v ./internal/usecase/ -run TestClockOut
  ```
  Expected: all 6 `TestClockOut_*` tests PASS

- [ ] **Step 8: Run full build to confirm no compile errors**

  ```bash
  go build ./...
  ```
  Expected: clean build

- [ ] **Step 9: Commit all four changed files together**

  ```bash
  git add internal/usecase/attendance_uc_test.go \
          internal/domain/attendance.go \
          internal/usecase/attendance_uc.go \
          internal/repository/postgres/attendance_repo.go \
          internal/delivery/http/handler/attendance_handler.go
  git commit -m "feat: implement offline clock-out timestamp resolution (usecase + repo + tests)"
  ```

---

## Chunk 3: Handler Layer + Wiring

### Task 4: Create `UtilityHandler`

**Files:**
- Create: `internal/delivery/http/handler/utility_handler.go`

- [ ] **Step 1: Create the file**

  ```go
  package handler

  import (
  	"net/http"
  	"time"

  	"github.com/gin-gonic/gin"
  	"hris-backend/internal/delivery/http/middleware"
  )

  // UtilityHandler serves general-purpose utility endpoints.
  type UtilityHandler struct{}

  // NewUtilityHandler registers utility routes on the provided router group.
  // JWT is applied per-route (not at group level) since these routes are independent.
  func NewUtilityHandler(r *gin.RouterGroup) {
  	h := &UtilityHandler{}
  	r.GET("/time", middleware.JWTAuth(), h.GetServerTime)
  }

  // @Summary Get current server time
  // @Description Returns the current server time in RFC3339 format with timezone offset.
  // @Description Used by mobile clients to anchor offline clock-out timestamp reconstruction.
  // @Tags Utility
  // @Produce json
  // @Security BearerAuth
  // @Success 200 {object} map[string]string "Server time in RFC3339 format"
  // @Failure 401 {object} map[string]interface{} "Missing or invalid JWT token"
  // @Router /time [get]
  func (h *UtilityHandler) GetServerTime(c *gin.Context) {
  	c.JSON(http.StatusOK, gin.H{"server_time": time.Now().Format(time.RFC3339)})
  }
  ```

- [ ] **Step 2: Build check**

  ```bash
  go build ./internal/delivery/...
  ```
  Expected: clean build

- [ ] **Step 3: Commit**

  ```bash
  git add internal/delivery/http/handler/utility_handler.go
  git commit -m "feat(handler): add UtilityHandler with GET /time endpoint"
  ```

---

### Task 5: Complete `ClockOut` handler update

**Files:**
- Modify: `internal/delivery/http/handler/attendance_handler.go`

- [ ] **Step 1: Add `fmt` and `io` imports**

  Check the existing import block in `attendance_handler.go`. Ensure both `"fmt"` and `"io"` are present. Add them if missing.

- [ ] **Step 2: Replace the `ClockOut` method**

  Replace the entire `ClockOut` method (including Swagger annotations):

  ```go
  // @Summary Clock out for today
  // @Description Records the employee's clock-out for today. Automatically ends any open break.
  // @Description Accepts an optional client_timestamp (RFC3339) for offline-first submissions.
  // @Description If client_timestamp is omitted, server time is used.
  // @Tags Attendance
  // @Accept json
  // @Produce json
  // @Security BearerAuth
  // @Param request body domain.ClockOutRequest false "Optional clock-out request body"
  // @Success 200 {object} domain.ClockOutResponse "Clock-out recorded successfully"
  // @Failure 400 {object} map[string]interface{} "Invalid or out-of-range client_timestamp"
  // @Failure 401 {object} map[string]interface{} "Missing or invalid JWT token"
  // @Failure 404 {object} map[string]interface{} "No clock-in record found for today"
  // @Failure 409 {object} map[string]interface{} "Already clocked out"
  // @Failure 500 {object} map[string]interface{} "Internal server error"
  // @Router /attendance/clock-out [post]
  func (h *AttendanceHandler) ClockOut(c *gin.Context) {
  	employeeID, companyID, ok := extractClaims(c)
  	if !ok {
  		return
  	}

  	var req domain.ClockOutRequest
  	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
  		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
  		return
  	}

  	resp, err := h.attendanceUsecase.ClockOut(c.Request.Context(), employeeID, companyID, req)
  	if err != nil {
  		switch {
  		case errors.Is(err, domain.ErrInvalidClientTimestamp):
  			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
  		case errors.Is(err, domain.ErrClientTimestampInFuture):
  			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
  		case errors.Is(err, domain.ErrClientTimestampTooOld):
  			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("client_timestamp exceeds max offline duration of %d hours", int(domain.MaxOfflineDuration.Hours()))})
  		case errors.Is(err, domain.ErrNotClockedIn):
  			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
  		case errors.Is(err, domain.ErrAlreadyClockedOut):
  			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
  		default:
  			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
  		}
  		return
  	}

  	c.JSON(http.StatusOK, resp)
  }
  ```

- [ ] **Step 3: Build check**

  ```bash
  go build ./internal/delivery/...
  ```
  Expected: clean build

- [ ] **Step 4: Commit**

  ```bash
  git add internal/delivery/http/handler/attendance_handler.go
  git commit -m "feat(handler): update ClockOut to accept and bind optional ClockOutRequest"
  ```

---

### Task 6: Wire `NewUtilityHandler` in `main.go`

**Files:**
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Add `NewUtilityHandler` call**

  Find the handler registration block (looks like):
  ```go
  handler.NewEmployeeHandler(apiV1, empUsecase)
  handler.NewAuthHandler(apiV1, authUsecase)
  handler.NewAttendanceHandler(apiV1, attendanceUsecase)
  ```

  Add after the last line:
  ```go
  handler.NewUtilityHandler(apiV1)
  ```

- [ ] **Step 2: Full build verification**

  ```bash
  go build ./...
  ```
  Expected: clean build

- [ ] **Step 3: Commit**

  ```bash
  git add cmd/api/main.go
  git commit -m "feat(main): register UtilityHandler for GET /api/v1/time"
  ```

---

## Chunk 4: Integration Tests

### Task 7: Register `NewUtilityHandler` in test routers and add `GET /time` tests

**Files:**
- Modify: `tests/attendance_flow_integration_test.go`

- [ ] **Step 1: Add `NewUtilityHandler` to the test router in `TestAttendanceFullFlow`**

  Find the handler block in `TestAttendanceFullFlow` (lines ~42-44):
  ```go
  handler.NewEmployeeHandler(api, empUC)
  handler.NewAuthHandler(api, authUC)
  handler.NewAttendanceHandler(api, attendanceUC)
  ```
  Add:
  ```go
  handler.NewUtilityHandler(api)
  ```

- [ ] **Step 2: Add server-time sub-tests after the `P9` sub-test in `TestAttendanceFullFlow`**

  ```go
  t.Run("T1: GET /time returns RFC3339 server time", func(t *testing.T) {
  	w := doReq(http.MethodGet, "/api/v1/time", nil, flowToken)
  	require.Equal(t, http.StatusOK, w.Code)
  	var resp map[string]string
  	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
  	serverTime, ok := resp["server_time"]
  	require.True(t, ok, "response must contain server_time key")
  	parsed, err := time.Parse(time.RFC3339, serverTime)
  	require.NoError(t, err, "server_time must be valid RFC3339")
  	// Server time must be within the last 5 seconds.
  	assert.WithinDuration(t, time.Now(), parsed, 5*time.Second)
  })

  t.Run("T2: GET /time without auth returns 401", func(t *testing.T) {
  	w := doReq(http.MethodGet, "/api/v1/time", nil, "")
  	assert.Equal(t, http.StatusUnauthorized, w.Code)
  })
  ```

- [ ] **Step 3: Update the existing P8 and N2 sub-tests to assert `is_offline_submission` field**

  Find `P8: clock-out succeeds` — add one assertion:
  ```go
  assert.False(t, resp.IsOfflineSubmission) // server-time clock-out is not offline
  ```

  Find `N2: clock-out twice → 409` — no changes needed (body is nil, code is 409, no `ClockOutResponse` decoded).

- [ ] **Step 4: Run the new time tests**

  ```bash
  go test -v ./tests/ -run "TestAttendanceFullFlow/T1|TestAttendanceFullFlow/T2"
  ```
  Expected: PASS

---

### Task 8: Add `TestOfflineClockOut` integration test function

**Files:**
- Modify: `tests/attendance_flow_integration_test.go`

- [ ] **Step 1: Add `TestOfflineClockOut` at the end of the file**

  ```go
  // TestOfflineClockOut tests the offline clock-out timestamp resolution logic end-to-end.
  func TestOfflineClockOut(t *testing.T) {
  	db, rdb := setupTestDB()
  	gin.SetMode(gin.TestMode)
  	router := gin.New()
  	api := router.Group("/api/v1")

  	seqRepo := repoPostgres.NewEmployeeSequenceRepository(db)
  	empRepo := repoPostgres.NewEmployeeRepository(db)
  	compRepo := repoPostgres.NewCompanyRepository(db)
  	otpRepo := redis.NewOTPRepository(rdb)
  	attendanceRepo := repoPostgres.NewAttendanceRepository(db)
  	breakRepo := repoPostgres.NewAttendanceBreakRepository(db)
  	scheduleRepo := repoPostgres.NewEmployeeScheduleRepository(db)

  	empUC := usecase.NewEmployeeUsecase(seqRepo, empRepo, compRepo)
  	authUC := usecase.NewAuthUsecase(empRepo, otpRepo, &noopEmailEnqueuer{})
  	attendanceUC := usecase.NewAttendanceUsecase(empRepo, compRepo, attendanceRepo, breakRepo, scheduleRepo)

  	handler.NewEmployeeHandler(api, empUC)
  	handler.NewAuthHandler(api, authUC)
  	handler.NewAttendanceHandler(api, attendanceUC)
  	handler.NewUtilityHandler(api)

  	offlineEmail := fmt.Sprintf("test_offline_%s@yopmail.com", uuid.New().String()[:8])
  	offlinePassword := "rahasia123"

  	cleanup := func() {
  		db.Exec("DELETE FROM attendance_breaks WHERE attendance_record_id IN (SELECT id FROM attendance_records WHERE employee_id IN (SELECT id FROM employees WHERE email LIKE 'test_offline_%'))")
  		db.Exec("DELETE FROM attendance_records WHERE employee_id IN (SELECT id FROM employees WHERE email LIKE 'test_offline_%')")
  		db.Exec("DELETE FROM employee_schedules WHERE employee_id IN (SELECT id FROM employees WHERE email LIKE 'test_offline_%')")
  		db.Where("email LIKE ?", "test_offline_%").Delete(&domain.Employee{})
  		db.Exec("UPDATE companies SET office_latitude=?, office_longitude=?, office_radius_meters=? WHERE company_code='GOTO'",
  			officeLatitude, officeLongitude, officeRadius)
  	}
  	cleanup()
  	defer cleanup()

  	doReq := func(method, path string, body interface{}, token string) *httptest.ResponseRecorder {
  		var buf *bytes.Buffer
  		if body != nil {
  			b, _ := json.Marshal(body)
  			buf = bytes.NewBuffer(b)
  		} else {
  			buf = bytes.NewBuffer(nil)
  		}
  		req, _ := http.NewRequest(method, path, buf)
  		req.Header.Set("Content-Type", "application/json")
  		if token != "" {
  			req.Header.Set("Authorization", "Bearer "+token)
  		}
  		w := httptest.NewRecorder()
  		router.ServeHTTP(w, req)
  		return w
  	}

  	var token string

  	// Setup: register + login once for the whole test function.
  	t.Run("Setup", func(t *testing.T) {
  		w := doReq(http.MethodPost, "/api/v1/register", domain.RegisterRequest{
  			Name:        "Offline Tester",
  			Email:       offlineEmail,
  			Password:    offlinePassword,
  			PhoneNumber: "081234000999",
  			CompanyCode: "GOTO",
  		}, "")
  		require.Equal(t, http.StatusCreated, w.Code)

  		w = doReq(http.MethodPost, "/api/v1/auth/login", domain.LoginRequest{
  			Email:    offlineEmail,
  			Password: offlinePassword,
  		}, "")
  		require.Equal(t, http.StatusOK, w.Code)
  		var loginResp map[string]interface{}
  		json.Unmarshal(w.Body.Bytes(), &loginResp)
  		token = loginResp["token"].(string)
  	})

  	// clockIn is a helper to create a fresh clock-in record for the offline test employee.
  	// It deletes any existing attendance record first so clock-in can always proceed.
  	clockIn := func(t *testing.T) {
  		t.Helper()
  		db.Exec("DELETE FROM attendance_breaks WHERE attendance_record_id IN (SELECT id FROM attendance_records WHERE employee_id IN (SELECT id FROM employees WHERE email = ?))", offlineEmail)
  		db.Exec("DELETE FROM attendance_records WHERE employee_id IN (SELECT id FROM employees WHERE email = ?)", offlineEmail)
  		w := doReq(http.MethodPost, "/api/v1/attendance/clock-in", map[string]interface{}{
  			"selfie_url": "https://res.cloudinary.com/test/selfie.jpg",
  			"latitude":   officeLatitude,
  			"longitude":  officeLongitude,
  		}, token)
  		require.Equal(t, http.StatusCreated, w.Code, "clock-in must succeed for the test to proceed")
  	}

  	t.Run("O1: clock-out with no body uses server time, is_offline_submission=false", func(t *testing.T) {
  		clockIn(t)
  		w := doReq(http.MethodPost, "/api/v1/attendance/clock-out", nil, token)
  		require.Equal(t, http.StatusOK, w.Code)
  		var resp domain.ClockOutResponse
  		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
  		assert.Equal(t, "clocked_out", resp.Status)
  		assert.False(t, resp.IsOfflineSubmission)
  	})

  	t.Run("O2: clock-out with valid client_timestamp sets is_offline_submission=true", func(t *testing.T) {
  		clockIn(t)
  		clientTS := time.Now().Add(-30 * time.Minute).Format(time.RFC3339)
  		w := doReq(http.MethodPost, "/api/v1/attendance/clock-out", map[string]interface{}{
  			"client_timestamp": clientTS,
  		}, token)
  		require.Equal(t, http.StatusOK, w.Code)
  		var resp domain.ClockOutResponse
  		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
  		assert.Equal(t, "clocked_out", resp.Status)
  		assert.True(t, resp.IsOfflineSubmission)
  	})

  	t.Run("O3: clock-out with notes persists notes in attendance record", func(t *testing.T) {
  		clockIn(t)
  		w := doReq(http.MethodPost, "/api/v1/attendance/clock-out", map[string]interface{}{
  			"notes": "left early due to emergency",
  		}, token)
  		require.Equal(t, http.StatusOK, w.Code)
  		// Confirm the note is visible via the today-status endpoint.
  		w2 := doReq(http.MethodGet, "/api/v1/attendance/today", nil, token)
  		require.Equal(t, http.StatusOK, w2.Code)
  		var today domain.TodayStatusResponse
  		require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &today))
  		require.NotNil(t, today.Notes)
  		assert.Equal(t, "left early due to emergency", *today.Notes)
  	})

  	t.Run("O4: future client_timestamp returns 400", func(t *testing.T) {
  		clockIn(t)
  		future := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
  		w := doReq(http.MethodPost, "/api/v1/attendance/clock-out", map[string]interface{}{
  			"client_timestamp": future,
  		}, token)
  		assert.Equal(t, http.StatusBadRequest, w.Code)
  		// Record is still open (clock-out was rejected); clean it up for the next test.
  		db.Exec("DELETE FROM attendance_records WHERE employee_id IN (SELECT id FROM employees WHERE email = ?)", offlineEmail)
  	})

  	t.Run("O5: client_timestamp older than 24h returns 400 with duration in message", func(t *testing.T) {
  		clockIn(t)
  		tooOld := time.Now().Add(-25 * time.Hour).Format(time.RFC3339)
  		w := doReq(http.MethodPost, "/api/v1/attendance/clock-out", map[string]interface{}{
  			"client_timestamp": tooOld,
  		}, token)
  		assert.Equal(t, http.StatusBadRequest, w.Code)
  		var body map[string]string
  		json.Unmarshal(w.Body.Bytes(), &body)
  		assert.Contains(t, body["error"], "24", "error message must mention the 24-hour limit")
  		db.Exec("DELETE FROM attendance_records WHERE employee_id IN (SELECT id FROM employees WHERE email = ?)", offlineEmail)
  	})

  	t.Run("O6: invalid RFC3339 client_timestamp returns 400", func(t *testing.T) {
  		clockIn(t)
  		w := doReq(http.MethodPost, "/api/v1/attendance/clock-out", map[string]interface{}{
  			"client_timestamp": "not-a-date",
  		}, token)
  		assert.Equal(t, http.StatusBadRequest, w.Code)
  		db.Exec("DELETE FROM attendance_records WHERE employee_id IN (SELECT id FROM employees WHERE email = ?)", offlineEmail)
  	})
  }
  ```

- [ ] **Step 2: Run the offline clock-out integration tests**

  ```bash
  go test -v ./tests/ -run "TestOfflineClockOut"
  ```
  Expected: all `O1`–`O6` sub-tests PASS

- [ ] **Step 3: Run the full integration suite to confirm no regressions**

  ```bash
  go test -v ./tests/ -run "TestAttendanceFullFlow"
  ```
  Expected: all sub-tests (P1–P9, T1–T2, N1–N7, E1–E5) PASS. P8 and N2 call clock-out with nil body — this still passes because the handler accepts an empty body gracefully (EOF is not treated as an error).

- [ ] **Step 4: Run unit tests one final time**

  ```bash
  go test -v ./internal/usecase/ -run TestClockOut
  ```
  Expected: PASS

- [ ] **Step 5: Commit**

  ```bash
  git add tests/attendance_flow_integration_test.go
  git commit -m "test(integration): add offline clock-out and GET /time integration tests"
  ```

---

### Task 9: Regenerate Swagger docs

- [ ] **Step 1: Regenerate**

  ```bash
  swag init -g cmd/api/main.go --parseDependency --parseInternal
  ```

- [ ] **Step 2: Commit**

  ```bash
  git add docs/
  git commit -m "docs(swagger): regenerate for offline clock-out and GET /time"
  ```

---

## Summary of files changed

| File | Change |
|---|---|
| `migrations/000012_add_offline_clockout.up.sql` | New |
| `migrations/000012_add_offline_clockout.down.sql` | New |
| `internal/domain/attendance.go` | const, errors, struct field, ClockOutRequest DTO, ClockOutResponse, interface |
| `internal/usecase/attendance_uc.go` | ClockOut reimplemented |
| `internal/usecase/attendance_uc_test.go` | New — unit tests |
| `internal/repository/postgres/attendance_repo.go` | UpdateClockOut map extended |
| `internal/delivery/http/handler/utility_handler.go` | New |
| `internal/delivery/http/handler/attendance_handler.go` | ClockOut handler updated |
| `cmd/api/main.go` | NewUtilityHandler registered |
| `tests/attendance_flow_integration_test.go` | NewUtilityHandler in test router, new T1/T2/O1–O6 sub-tests |
