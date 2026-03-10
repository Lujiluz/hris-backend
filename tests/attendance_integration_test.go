package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"hris-backend/internal/delivery/http/handler"
	"hris-backend/internal/domain"
	repoPostgres "hris-backend/internal/repository/postgres"
	"hris-backend/internal/repository/redis"
	"hris-backend/internal/usecase"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// officeLatitude and officeLongitude represent the GOTO office in Jakarta
const (
	officeLatitude  = -6.2088
	officeLongitude = 106.8456
	officeRadius    = 100.0 // metres
)

func buildAttendanceTestRouter(db interface{}, rdb interface{}) (*gin.Engine, func()) {
	// Using concrete types — re-implemented inline since tests package shares the same package
	return nil, nil
}

func TestAttendanceIntegration(t *testing.T) {
	db, rdb := setupTestDB()
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Init repos
	seqRepo := repoPostgres.NewEmployeeSequenceRepository(db)
	empRepo := repoPostgres.NewEmployeeRepository(db)
	compRepo := repoPostgres.NewCompanyRepository(db)
	otpRepo := redis.NewOTPRepository(rdb)
	attendanceRepo := repoPostgres.NewAttendanceRepository(db)
	breakRepo := repoPostgres.NewAttendanceBreakRepository(db)
	scheduleRepo := repoPostgres.NewEmployeeScheduleRepository(db)

	// Init usecases
	empUC := usecase.NewEmployeeUsecase(seqRepo, empRepo, compRepo)
	authUC := usecase.NewAuthUsecase(empRepo, otpRepo)
	attendanceUC := usecase.NewAttendanceUsecase(empRepo, compRepo, attendanceRepo, breakRepo, scheduleRepo)

	// Init handlers
	api := router.Group("/api/v1")
	handler.NewEmployeeHandler(api, empUC)
	handler.NewAuthHandler(api, authUC)
	handler.NewAttendanceHandler(api, attendanceUC)

	// Cleanup before and after
	now := time.Now().UTC().Format(time.RFC3339)
	testEmailPrefix := "test_integration_" + now
	cleanup := func() {
		db.Where("email LIKE ?", testEmailPrefix+"%").Delete(&domain.Employee{})
		db.Exec(
			"UPDATE companies SET office_latitude=?, office_longitude=?, office_radius_meters=? WHERE company_code='GOTO'",
			officeLatitude, officeLongitude, officeRadius,
		)
	}
	cleanup()
	defer cleanup()

	// --- SETUP ---
	generatedUserMock := SetupMockUser("attendance", testEmailPrefix)
	testEmail := generatedUserMock.Email
	testPassword := generatedUserMock.RandomPassword
	var validToken string

	t.Run("Setup: Register test employee", func(t *testing.T) {
		reqBody := domain.RegisterRequest{
			CompanyCode:   "GOTO",
			Email:         testEmail,
			PhoneNumber:   generatedUserMock.PhoneNumber,
			Password:      testPassword,
			IsTncAccepted: true,
		}
		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("Setup: Configure office geofence", func(t *testing.T) {
		result := db.Exec(
			"UPDATE companies SET office_latitude=?, office_longitude=?, office_radius_meters=? WHERE company_code='GOTO'",
			officeLatitude, officeLongitude, officeRadius,
		)
		require.NoError(t, result.Error)
	})

	t.Run("Setup: Login and get JWT", func(t *testing.T) {
		var emp domain.Employee
		err := db.Where("email = ?", testEmail).First(&emp).Error
		require.NoError(t, err)

		reqBody := domain.LoginRequest{
			EmployeeID: emp.EmployeeID,
			Password:   testPassword,
		}
		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		validToken = resp["token"].(string)
		require.NotEmpty(t, validToken)
	})

	makeReq := func(token string, body interface{}) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/attendance/validate-location", bytes.NewBuffer(b))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	lat := officeLatitude
	lon := officeLongitude

	// --- POSITIVE CASES ---

	t.Run("P1: Inside geofence", func(t *testing.T) {
		w := makeReq(validToken, map[string]interface{}{
			"latitude": lat, "longitude": lon,
			"accuracy_meters": 10.0, "is_mock_location": false,
		})
		assert.Equal(t, http.StatusOK, w.Code)
		var resp domain.ValidateLocationResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.True(t, resp.IsInsideGeofence)
	})

	t.Run("P2: Exactly on boundary (inclusive)", func(t *testing.T) {
		// Move exactly `radius` metres north; 1 degree lat ≈ 111,320 m
		boundaryLat := officeLatitude + (officeRadius / 111320.0)
		w := makeReq(validToken, map[string]interface{}{
			"latitude": boundaryLat, "longitude": lon,
			"accuracy_meters": 10.0, "is_mock_location": false,
		})
		assert.Equal(t, http.StatusOK, w.Code)
		var resp domain.ValidateLocationResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.True(t, resp.IsInsideGeofence)
	})

	t.Run("P3: Response includes correct distance", func(t *testing.T) {
		w := makeReq(validToken, map[string]interface{}{
			"latitude": lat, "longitude": lon,
			"accuracy_meters": 10.0, "is_mock_location": false,
		})
		assert.Equal(t, http.StatusOK, w.Code)
		var resp domain.ValidateLocationResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.InDelta(t, 0.0, resp.DistanceMeters, 1.0)
	})

	t.Run("P4: Response includes allowed radius", func(t *testing.T) {
		w := makeReq(validToken, map[string]interface{}{
			"latitude": lat, "longitude": lon,
			"accuracy_meters": 10.0, "is_mock_location": false,
		})
		assert.Equal(t, http.StatusOK, w.Code)
		var resp domain.ValidateLocationResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, officeRadius, resp.AllowedRadiusMeters)
	})

	// --- NEGATIVE CASES ---

	t.Run("N1: Outside geofence", func(t *testing.T) {
		w := makeReq(validToken, map[string]interface{}{
			"latitude": -33.865, "longitude": 151.209,
			"accuracy_meters": 10.0, "is_mock_location": false,
		})
		assert.Equal(t, http.StatusOK, w.Code)
		var resp domain.ValidateLocationResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.False(t, resp.IsInsideGeofence)
	})

	t.Run("N2: Missing Authorization header", func(t *testing.T) {
		w := makeReq("", map[string]interface{}{
			"latitude": lat, "longitude": lon,
			"accuracy_meters": 10.0, "is_mock_location": false,
		})
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("N3: Invalid JWT", func(t *testing.T) {
		w := makeReq("this.is.not.valid", map[string]interface{}{
			"latitude": lat, "longitude": lon,
			"accuracy_meters": 10.0, "is_mock_location": false,
		})
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("N4: Expired JWT", func(t *testing.T) {
		secret := os.Getenv("JWT_SECRET")
		claims := jwt.MapClaims{
			"employee_id": "test-emp",
			"role":        "staff",
			"company_id":  "some-company-id",
			"exp":         time.Now().Add(-1 * time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		expiredToken, _ := token.SignedString([]byte(secret))

		w := makeReq(expiredToken, map[string]interface{}{
			"latitude": lat, "longitude": lon,
			"accuracy_meters": 10.0, "is_mock_location": false,
		})
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("N5: Missing latitude field", func(t *testing.T) {
		w := makeReq(validToken, map[string]interface{}{
			"longitude": lon, "accuracy_meters": 10.0, "is_mock_location": false,
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("N6: Missing longitude field", func(t *testing.T) {
		w := makeReq(validToken, map[string]interface{}{
			"latitude": lat, "accuracy_meters": 10.0, "is_mock_location": false,
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("N7: Missing accuracy_meters field", func(t *testing.T) {
		w := makeReq(validToken, map[string]interface{}{
			"latitude": lat, "longitude": lon, "is_mock_location": false,
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("N8: Mock location flagged → 403", func(t *testing.T) {
		w := makeReq(validToken, map[string]interface{}{
			"latitude": lat, "longitude": lon,
			"accuracy_meters": 10.0, "is_mock_location": true,
		})
		assert.Equal(t, http.StatusForbidden, w.Code)
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Contains(t, resp["error"], "mock")
	})

	t.Run("N9: GPS accuracy too low → 422", func(t *testing.T) {
		w := makeReq(validToken, map[string]interface{}{
			"latitude": lat, "longitude": lon,
			"accuracy_meters": 200.0, "is_mock_location": false,
		})
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("N10: Office not configured → 422", func(t *testing.T) {
		db.Exec("UPDATE companies SET office_latitude=NULL, office_longitude=NULL, office_radius_meters=NULL WHERE company_code='GOTO'")
		defer db.Exec(
			"UPDATE companies SET office_latitude=?, office_longitude=?, office_radius_meters=? WHERE company_code='GOTO'",
			officeLatitude, officeLongitude, officeRadius,
		)
		w := makeReq(validToken, map[string]interface{}{
			"latitude": lat, "longitude": lon,
			"accuracy_meters": 10.0, "is_mock_location": false,
		})
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Contains(t, resp["error"], "not configured")
	})

	// --- EDGE CASES ---

	t.Run("E1: Coordinates at (0.0, 0.0) — valid equator", func(t *testing.T) {
		zeroLat := 0.0
		zeroLon := 0.0
		w := makeReq(validToken, map[string]interface{}{
			"latitude": zeroLat, "longitude": zeroLon,
			"accuracy_meters": 10.0, "is_mock_location": false,
		})
		assert.Equal(t, http.StatusOK, w.Code)
		var resp domain.ValidateLocationResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.False(t, resp.IsInsideGeofence)
	})

	t.Run("E2: Negative coordinates (Sydney)", func(t *testing.T) {
		w := makeReq(validToken, map[string]interface{}{
			"latitude": -33.865, "longitude": 151.209,
			"accuracy_meters": 10.0, "is_mock_location": false,
		})
		assert.Equal(t, http.StatusOK, w.Code)
		var resp domain.ValidateLocationResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.False(t, resp.IsInsideGeofence)
	})

	t.Run("E3: Accuracy exactly at threshold (50.0) → accepted", func(t *testing.T) {
		w := makeReq(validToken, map[string]interface{}{
			"latitude": lat, "longitude": lon,
			"accuracy_meters": 50.0, "is_mock_location": false,
		})
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("E4: Accuracy just above threshold (50.001) → 422", func(t *testing.T) {
		w := makeReq(validToken, map[string]interface{}{
			"latitude": lat, "longitude": lon,
			"accuracy_meters": 50.001, "is_mock_location": false,
		})
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("E5: is_mock_location false + high accuracy → 422", func(t *testing.T) {
		w := makeReq(validToken, map[string]interface{}{
			"latitude": lat, "longitude": lon,
			"accuracy_meters": 200.0, "is_mock_location": false,
		})
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("E6: Very small radius (1m) + coords exactly at office → inside", func(t *testing.T) {
		db.Exec("UPDATE companies SET office_radius_meters=1 WHERE company_code='GOTO'")
		defer db.Exec("UPDATE companies SET office_radius_meters=? WHERE company_code='GOTO'", officeRadius)

		w := makeReq(validToken, map[string]interface{}{
			"latitude": lat, "longitude": lon,
			"accuracy_meters": 10.0, "is_mock_location": false,
		})
		assert.Equal(t, http.StatusOK, w.Code)
		var resp domain.ValidateLocationResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.True(t, resp.IsInsideGeofence)
	})

	t.Run("E7: Very small radius (1m) + coords 2m away → outside", func(t *testing.T) {
		db.Exec("UPDATE companies SET office_radius_meters=1 WHERE company_code='GOTO'")
		defer db.Exec("UPDATE companies SET office_radius_meters=? WHERE company_code='GOTO'", officeRadius)

		twoMetresNorthLat := officeLatitude + (2.0 / 111320.0)
		w := makeReq(validToken, map[string]interface{}{
			"latitude": twoMetresNorthLat, "longitude": lon,
			"accuracy_meters": 10.0, "is_mock_location": false,
		})
		assert.Equal(t, http.StatusOK, w.Code)
		var resp domain.ValidateLocationResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.False(t, resp.IsInsideGeofence)
	})

	t.Run("E8: JWT with missing company_id claim → 401", func(t *testing.T) {
		secret := os.Getenv("JWT_SECRET")
		claims := jwt.MapClaims{
			"employee_id": "test-emp",
			"role":        "staff",
			// company_id intentionally omitted
			"exp": time.Now().Add(24 * time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		oldToken, _ := token.SignedString([]byte(secret))

		w := makeReq(oldToken, map[string]interface{}{
			"latitude": lat, "longitude": lon,
			"accuracy_meters": 10.0, "is_mock_location": false,
		})
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("E9: accuracy_meters=0 → 400 (fails gt=0)", func(t *testing.T) {
		w := makeReq(validToken, map[string]interface{}{
			"latitude": lat, "longitude": lon,
			"accuracy_meters": 0, "is_mock_location": false,
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
