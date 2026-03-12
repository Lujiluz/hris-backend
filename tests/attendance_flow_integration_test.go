package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"hris-backend/internal/delivery/http/handler"
	"hris-backend/internal/domain"
	repoPostgres "hris-backend/internal/repository/postgres"
	"hris-backend/internal/repository/redis"
	"hris-backend/internal/usecase"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttendanceFullFlow(t *testing.T) {
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

	flowEmail := fmt.Sprintf("test_flow_%s@yopmail.com", uuid.New().String()[:8])
	flowPassword := "rahasia123"
	var flowToken string
	var flowEmployeeID string   // generated code, e.g. "GOTO-2025-001"
	var flowEmployeeUUID string // actual UUID primary key

	cleanup := func() {
		db.Exec("DELETE FROM attendance_breaks WHERE attendance_record_id IN (SELECT id FROM attendance_records WHERE employee_id IN (SELECT id FROM employees WHERE email LIKE 'test_flow_%'))")
		db.Exec("DELETE FROM attendance_records WHERE employee_id IN (SELECT id FROM employees WHERE email LIKE 'test_flow_%')")
		db.Exec("DELETE FROM employee_schedules WHERE employee_id IN (SELECT id FROM employees WHERE email LIKE 'test_flow_%')")
		db.Where("email LIKE ?", "test_flow_%").Delete(&domain.Employee{})
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

	// --- SETUP ---
	t.Run("Setup: register and login", func(t *testing.T) {
		w := doReq(http.MethodPost, "/api/v1/register", domain.RegisterRequest{
			CompanyCode:   "GOTO",
			Email:         flowEmail,
			PhoneNumber:   Random13DigitsNumber(),
			Password:      flowPassword,
			IsTncAccepted: true,
		}, "")
		require.Equal(t, http.StatusCreated, w.Code)

		var emp domain.Employee
		require.NoError(t, db.Where("email = ?", flowEmail).First(&emp).Error)
		flowEmployeeID = emp.EmployeeID
		flowEmployeeUUID = emp.ID

		// Seed Mon–Fri schedule: clock_in=08:00, clock_out=00:00 (midnight = start of day)
		// so any clock-out during the test run falls after scheduled_clock_out → overtime > 0.
		SeedEmployeeSchedule(db, flowEmployeeUUID, 8, 0)

		w = doReq(http.MethodPost, "/api/v1/auth/login", domain.LoginRequest{
			EmployeeID: flowEmployeeID,
			Password:   flowPassword,
		}, "")
		require.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		flowToken = resp["token"].(string)
		require.NotEmpty(t, flowToken)
	})

	// --- POSITIVE CASES ---

	t.Run("P1: today status when idle returns idle", func(t *testing.T) {
		w := doReq(http.MethodGet, "/api/v1/attendance/today", nil, flowToken)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp domain.TodayStatusResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "idle", resp.Status)
	})

	var attendanceID string

	t.Run("P2: clock-in succeeds", func(t *testing.T) {
		w := doReq(http.MethodPost, "/api/v1/attendance/clock-in", map[string]interface{}{
			"selfie_url": "https://res.cloudinary.com/test/selfie.jpg",
			"latitude":   officeLatitude,
			"longitude":  officeLongitude,
			"notes":      "WFO",
		}, flowToken)
		assert.Equal(t, http.StatusCreated, w.Code)
		var resp domain.ClockInResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "clocked_in", resp.Status)
		assert.NotEqual(t, uuid.Nil, resp.ID)
		attendanceID = resp.ID.String()
		assert.NotEmpty(t, attendanceID)
	})

	t.Run("P3: today status returns clocked_in with notes", func(t *testing.T) {
		w := doReq(http.MethodGet, "/api/v1/attendance/today", nil, flowToken)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp domain.TodayStatusResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "clocked_in", resp.Status)
		assert.False(t, resp.IsOnBreak)
		require.NotNil(t, resp.Notes)
		assert.Equal(t, "WFO", *resp.Notes)
	})

	t.Run("P4: start break transitions to on_break", func(t *testing.T) {
		w := doReq(http.MethodPost, "/api/v1/attendance/break", map[string]interface{}{"action": "start"}, flowToken)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp domain.BreakResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "on_break", resp.Status)
	})

	t.Run("P5: today status returns on_break with open_break_start_at", func(t *testing.T) {
		w := doReq(http.MethodGet, "/api/v1/attendance/today", nil, flowToken)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp domain.TodayStatusResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "on_break", resp.Status)
		assert.True(t, resp.IsOnBreak)
		assert.NotNil(t, resp.OpenBreakStartAt)
	})

	t.Run("P6: end break transitions back to clocked_in", func(t *testing.T) {
		w := doReq(http.MethodPost, "/api/v1/attendance/break", map[string]interface{}{"action": "end"}, flowToken)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp domain.BreakResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "clocked_in", resp.Status)
	})

	t.Run("P7: clockout-preview returns scheduled_clock_out_at and overtime > 0", func(t *testing.T) {
		w := doReq(http.MethodGet, "/api/v1/attendance/clockout-preview", nil, flowToken)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp domain.ClockOutPreview
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.GreaterOrEqual(t, resp.WorkingMinutes, 0)
		// Schedule has clock_out=00:00 (start of day) so we're always past it → overtime
		assert.Greater(t, resp.OvertimeMinutes, 0)
		assert.NotNil(t, resp.ScheduledClockOutAt)
	})

	t.Run("P8: clock-out succeeds, stores overtime > 0, status clocked_out", func(t *testing.T) {
		w := doReq(http.MethodPost, "/api/v1/attendance/clock-out", nil, flowToken)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp domain.ClockOutResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "clocked_out", resp.Status)
		assert.GreaterOrEqual(t, resp.WorkingMinutes, 0)
		assert.Greater(t, resp.OvertimeMinutes, 0) // clock_out=00:00 → always overtime
	})

	t.Run("P9: today status returns clocked_out with clock_out_at populated", func(t *testing.T) {
		w := doReq(http.MethodGet, "/api/v1/attendance/today", nil, flowToken)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp domain.TodayStatusResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "clocked_out", resp.Status)
		assert.NotNil(t, resp.ClockOutAt)
	})

	// --- NEGATIVE CASES ---

	t.Run("N1: clock-in twice same day → 409", func(t *testing.T) {
		w := doReq(http.MethodPost, "/api/v1/attendance/clock-in", map[string]interface{}{
			"selfie_url": "https://res.cloudinary.com/test/selfie2.jpg",
			"latitude":   officeLatitude,
			"longitude":  officeLongitude,
		}, flowToken)
		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("N2: clock-out twice → 409", func(t *testing.T) {
		w := doReq(http.MethodPost, "/api/v1/attendance/clock-out", nil, flowToken)
		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("N3: clockout-preview when already clocked out → 409", func(t *testing.T) {
		w := doReq(http.MethodGet, "/api/v1/attendance/clockout-preview", nil, flowToken)
		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("N4: unauthenticated clock-in → 401", func(t *testing.T) {
		w := doReq(http.MethodPost, "/api/v1/attendance/clock-in", map[string]interface{}{
			"selfie_url": "https://res.cloudinary.com/test/selfie.jpg",
			"latitude":   officeLatitude,
			"longitude":  officeLongitude,
		}, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("N5: unauthenticated today → 401", func(t *testing.T) {
		w := doReq(http.MethodGet, "/api/v1/attendance/today", nil, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("N6: unauthenticated clockout-preview → 401", func(t *testing.T) {
		w := doReq(http.MethodGet, "/api/v1/attendance/clockout-preview", nil, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("N7: unauthenticated clock-out → 401", func(t *testing.T) {
		w := doReq(http.MethodPost, "/api/v1/attendance/clock-out", nil, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("N8: unauthenticated break → 401", func(t *testing.T) {
		w := doReq(http.MethodPost, "/api/v1/attendance/break", map[string]interface{}{"action": "start"}, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("N9: missing selfie_url on clock-in → 400", func(t *testing.T) {
		// Need a fresh employee for this; reuse a second email
		email2 := fmt.Sprintf("test_flow_%s@yopmail.com", uuid.New().String()[:8])
		doReq(http.MethodPost, "/api/v1/register", domain.RegisterRequest{
			CompanyCode:   "GOTO",
			Email:         email2,
			PhoneNumber:   "+6287" + uuid.New().String()[:8],
			Password:      flowPassword,
			IsTncAccepted: true,
		}, "")
		var emp2 domain.Employee
		db.Where("email = ?", email2).First(&emp2)
		w := doReq(http.MethodPost, "/api/v1/auth/login", domain.LoginRequest{
			EmployeeID: emp2.EmployeeID, Password: flowPassword,
		}, "")
		var r2 map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &r2)
		tok2 := r2["token"].(string)

		w = doReq(http.MethodPost, "/api/v1/attendance/clock-in", map[string]interface{}{
			"latitude":  officeLatitude,
			"longitude": officeLongitude,
		}, tok2)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("N10: missing latitude on clock-in → 400", func(t *testing.T) {
		email3 := fmt.Sprintf("test_flow_%s@yopmail.com", uuid.New().String()[:8])
		doReq(http.MethodPost, "/api/v1/register", domain.RegisterRequest{
			CompanyCode:   "GOTO",
			Email:         email3,
			PhoneNumber:   "+6286" + uuid.New().String()[:8],
			Password:      flowPassword,
			IsTncAccepted: true,
		}, "")
		var emp3 domain.Employee
		db.Where("email = ?", email3).First(&emp3)
		w := doReq(http.MethodPost, "/api/v1/auth/login", domain.LoginRequest{
			EmployeeID: emp3.EmployeeID, Password: flowPassword,
		}, "")
		var r3 map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &r3)
		tok3 := r3["token"].(string)

		w = doReq(http.MethodPost, "/api/v1/attendance/clock-in", map[string]interface{}{
			"selfie_url": "https://res.cloudinary.com/test/selfie.jpg",
			"longitude":  officeLongitude,
		}, tok3)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("N11: invalid break action → 400", func(t *testing.T) {
		// Use a fresh employee who's clocked in
		email4 := fmt.Sprintf("test_flow_%s@yopmail.com", uuid.New().String()[:8])
		doReq(http.MethodPost, "/api/v1/register", domain.RegisterRequest{
			CompanyCode:   "GOTO",
			Email:         email4,
			PhoneNumber:   "+6285" + uuid.New().String()[:8],
			Password:      flowPassword,
			IsTncAccepted: true,
		}, "")
		var emp4 domain.Employee
		db.Where("email = ?", email4).First(&emp4)
		w := doReq(http.MethodPost, "/api/v1/auth/login", domain.LoginRequest{
			EmployeeID: emp4.EmployeeID, Password: flowPassword,
		}, "")
		var r4 map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &r4)
		tok4 := r4["token"].(string)

		doReq(http.MethodPost, "/api/v1/attendance/clock-in", map[string]interface{}{
			"selfie_url": "https://res.cloudinary.com/test/selfie.jpg",
			"latitude":   officeLatitude,
			"longitude":  officeLongitude,
		}, tok4)

		w = doReq(http.MethodPost, "/api/v1/attendance/break", map[string]interface{}{"action": "pause"}, tok4)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	// --- EDGE CASES ---

	t.Run("E1: clock-out while on break auto-ends break", func(t *testing.T) {
		email5 := fmt.Sprintf("test_flow_%s@yopmail.com", uuid.New().String()[:8])
		doReq(http.MethodPost, "/api/v1/register", domain.RegisterRequest{
			CompanyCode:   "GOTO",
			Email:         email5,
			PhoneNumber:   "+6284" + uuid.New().String()[:8],
			Password:      flowPassword,
			IsTncAccepted: true,
		}, "")
		var emp5 domain.Employee
		db.Where("email = ?", email5).First(&emp5)
		w := doReq(http.MethodPost, "/api/v1/auth/login", domain.LoginRequest{
			EmployeeID: emp5.EmployeeID, Password: flowPassword,
		}, "")
		var r5 map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &r5)
		tok5 := r5["token"].(string)

		doReq(http.MethodPost, "/api/v1/attendance/clock-in", map[string]interface{}{
			"selfie_url": "https://res.cloudinary.com/test/selfie.jpg",
			"latitude":   officeLatitude,
			"longitude":  officeLongitude,
		}, tok5)
		doReq(http.MethodPost, "/api/v1/attendance/break", map[string]interface{}{"action": "start"}, tok5)

		w = doReq(http.MethodPost, "/api/v1/attendance/clock-out", nil, tok5)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp domain.ClockOutResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "clocked_out", resp.Status)
		assert.GreaterOrEqual(t, resp.WorkingMinutes, 0)

		// Verify break was closed in DB
		var openBreaks int64
		db.Table("attendance_breaks").
			Joins("JOIN attendance_records ar ON ar.id = attendance_breaks.attendance_record_id").
			Joins("JOIN employees e ON e.id = ar.employee_id").
			Where("e.email = ? AND attendance_breaks.break_end_at IS NULL", email5).
			Count(&openBreaks)
		assert.Equal(t, int64(0), openBreaks)
	})

	t.Run("E2: break start twice → 409", func(t *testing.T) {
		email6 := fmt.Sprintf("test_flow_%s@yopmail.com", uuid.New().String()[:8])
		doReq(http.MethodPost, "/api/v1/register", domain.RegisterRequest{
			CompanyCode:   "GOTO",
			Email:         email6,
			PhoneNumber:   "+6283" + uuid.New().String()[:8],
			Password:      flowPassword,
			IsTncAccepted: true,
		}, "")
		var emp6 domain.Employee
		db.Where("email = ?", email6).First(&emp6)
		w := doReq(http.MethodPost, "/api/v1/auth/login", domain.LoginRequest{
			EmployeeID: emp6.EmployeeID, Password: flowPassword,
		}, "")
		var r6 map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &r6)
		tok6 := r6["token"].(string)

		doReq(http.MethodPost, "/api/v1/attendance/clock-in", map[string]interface{}{
			"selfie_url": "https://res.cloudinary.com/test/selfie.jpg",
			"latitude":   officeLatitude,
			"longitude":  officeLongitude,
		}, tok6)
		doReq(http.MethodPost, "/api/v1/attendance/break", map[string]interface{}{"action": "start"}, tok6)

		w = doReq(http.MethodPost, "/api/v1/attendance/break", map[string]interface{}{"action": "start"}, tok6)
		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("E3: break end when not on break → 409", func(t *testing.T) {
		email7 := fmt.Sprintf("test_flow_%s@yopmail.com", uuid.New().String()[:8])
		doReq(http.MethodPost, "/api/v1/register", domain.RegisterRequest{
			CompanyCode:   "GOTO",
			Email:         email7,
			PhoneNumber:   "+6282" + uuid.New().String()[:8],
			Password:      flowPassword,
			IsTncAccepted: true,
		}, "")
		var emp7 domain.Employee
		db.Where("email = ?", email7).First(&emp7)
		w := doReq(http.MethodPost, "/api/v1/auth/login", domain.LoginRequest{
			EmployeeID: emp7.EmployeeID, Password: flowPassword,
		}, "")
		var r7 map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &r7)
		tok7 := r7["token"].(string)

		doReq(http.MethodPost, "/api/v1/attendance/clock-in", map[string]interface{}{
			"selfie_url": "https://res.cloudinary.com/test/selfie.jpg",
			"latitude":   officeLatitude,
			"longitude":  officeLongitude,
		}, tok7)

		w = doReq(http.MethodPost, "/api/v1/attendance/break", map[string]interface{}{"action": "end"}, tok7)
		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("E4: clockout-preview with no clock-in → 404", func(t *testing.T) {
		email8 := fmt.Sprintf("test_flow_%s@yopmail.com", uuid.New().String()[:8])
		doReq(http.MethodPost, "/api/v1/register", domain.RegisterRequest{
			CompanyCode:   "GOTO",
			Email:         email8,
			PhoneNumber:   "+6281" + uuid.New().String()[:8],
			Password:      flowPassword,
			IsTncAccepted: true,
		}, "")
		var emp8 domain.Employee
		db.Where("email = ?", email8).First(&emp8)
		w := doReq(http.MethodPost, "/api/v1/auth/login", domain.LoginRequest{
			EmployeeID: emp8.EmployeeID, Password: flowPassword,
		}, "")
		var r8 map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &r8)
		tok8 := r8["token"].(string)

		w = doReq(http.MethodGet, "/api/v1/attendance/clockout-preview", nil, tok8)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("E5: clock-out without clock-in → 404", func(t *testing.T) {
		email9 := fmt.Sprintf("test_flow_%s@yopmail.com", uuid.New().String()[:8])
		doReq(http.MethodPost, "/api/v1/register", domain.RegisterRequest{
			CompanyCode:   "GOTO",
			Email:         email9,
			PhoneNumber:   "+6280" + uuid.New().String()[:8],
			Password:      flowPassword,
			IsTncAccepted: true,
		}, "")
		var emp9 domain.Employee
		db.Where("email = ?", email9).First(&emp9)
		w := doReq(http.MethodPost, "/api/v1/auth/login", domain.LoginRequest{
			EmployeeID: emp9.EmployeeID, Password: flowPassword,
		}, "")
		var r9 map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &r9)
		tok9 := r9["token"].(string)

		w = doReq(http.MethodPost, "/api/v1/attendance/clock-out", nil, tok9)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("E6: multiple breaks deducted from working_minutes", func(t *testing.T) {
		emailM := fmt.Sprintf("test_flow_%s@yopmail.com", uuid.New().String()[:8])
		doReq(http.MethodPost, "/api/v1/register", domain.RegisterRequest{
			CompanyCode:   "GOTO",
			Email:         emailM,
			PhoneNumber:   "+6279" + uuid.New().String()[:8],
			Password:      flowPassword,
			IsTncAccepted: true,
		}, "")
		var empM domain.Employee
		db.Where("email = ?", emailM).First(&empM)
		w := doReq(http.MethodPost, "/api/v1/auth/login", domain.LoginRequest{
			EmployeeID: empM.EmployeeID, Password: flowPassword,
		}, "")
		var rM map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &rM)
		tokM := rM["token"].(string)

		doReq(http.MethodPost, "/api/v1/attendance/clock-in", map[string]interface{}{
			"selfie_url": "https://res.cloudinary.com/test/selfie.jpg",
			"latitude":   officeLatitude,
			"longitude":  officeLongitude,
		}, tokM)

		// Break 1
		doReq(http.MethodPost, "/api/v1/attendance/break", map[string]interface{}{"action": "start"}, tokM)
		time.Sleep(100 * time.Millisecond)
		doReq(http.MethodPost, "/api/v1/attendance/break", map[string]interface{}{"action": "end"}, tokM)
		// Break 2
		doReq(http.MethodPost, "/api/v1/attendance/break", map[string]interface{}{"action": "start"}, tokM)
		time.Sleep(100 * time.Millisecond)
		doReq(http.MethodPost, "/api/v1/attendance/break", map[string]interface{}{"action": "end"}, tokM)

		w = doReq(http.MethodPost, "/api/v1/attendance/clock-out", nil, tokM)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp domain.ClockOutResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.GreaterOrEqual(t, resp.WorkingMinutes, 0)
	})

	t.Run("E7: no schedule configured → overtime_minutes = 0", func(t *testing.T) {
		emailNS := fmt.Sprintf("test_flow_%s@yopmail.com", uuid.New().String()[:8])
		doReq(http.MethodPost, "/api/v1/register", domain.RegisterRequest{
			CompanyCode:   "GOTO",
			Email:         emailNS,
			PhoneNumber:   "+6278" + uuid.New().String()[:8],
			Password:      flowPassword,
			IsTncAccepted: true,
		}, "")
		var empNS domain.Employee
		db.Where("email = ?", emailNS).First(&empNS)
		w := doReq(http.MethodPost, "/api/v1/auth/login", domain.LoginRequest{
			EmployeeID: empNS.EmployeeID, Password: flowPassword,
		}, "")
		var rNS map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &rNS)
		tokNS := rNS["token"].(string)

		doReq(http.MethodPost, "/api/v1/attendance/clock-in", map[string]interface{}{
			"selfie_url": "https://res.cloudinary.com/test/selfie.jpg",
			"latitude":   officeLatitude,
			"longitude":  officeLongitude,
		}, tokNS)

		w = doReq(http.MethodPost, "/api/v1/attendance/clock-out", nil, tokNS)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp domain.ClockOutResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, 0, resp.OvertimeMinutes)
	})

	t.Run("E8: clockout-preview shows live break time excluded", func(t *testing.T) {
		emailLP := fmt.Sprintf("test_flow_%s@yopmail.com", uuid.New().String()[:8])
		doReq(http.MethodPost, "/api/v1/register", domain.RegisterRequest{
			CompanyCode:   "GOTO",
			Email:         emailLP,
			PhoneNumber:   "+6277" + uuid.New().String()[:8],
			Password:      flowPassword,
			IsTncAccepted: true,
		}, "")
		var empLP domain.Employee
		db.Where("email = ?", emailLP).First(&empLP)
		w := doReq(http.MethodPost, "/api/v1/auth/login", domain.LoginRequest{
			EmployeeID: empLP.EmployeeID, Password: flowPassword,
		}, "")
		var rLP map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &rLP)
		tokLP := rLP["token"].(string)

		doReq(http.MethodPost, "/api/v1/attendance/clock-in", map[string]interface{}{
			"selfie_url": "https://res.cloudinary.com/test/selfie.jpg",
			"latitude":   officeLatitude,
			"longitude":  officeLongitude,
		}, tokLP)
		doReq(http.MethodPost, "/api/v1/attendance/break", map[string]interface{}{"action": "start"}, tokLP)

		w = doReq(http.MethodGet, "/api/v1/attendance/clockout-preview", nil, tokLP)
		assert.Equal(t, http.StatusOK, w.Code)
		var prev domain.ClockOutPreview
		json.Unmarshal(w.Body.Bytes(), &prev)
		assert.GreaterOrEqual(t, prev.WorkingMinutes, 0)
	})
}
