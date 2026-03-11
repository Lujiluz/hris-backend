package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"hris-backend/internal/delivery/http/handler"
	"hris-backend/internal/domain"
	repoPostgres "hris-backend/internal/repository/postgres"
	"hris-backend/internal/repository/redis"
	"hris-backend/internal/usecase"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	redisClient "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	gormPostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// === 1. SETUP & TEARDOWN ===
func setupTestDB() (*gorm.DB, *redisClient.Client) {
	// ENV setup
	err := godotenv.Load("../.env")
	if err != nil {
		fmt.Println("Warning: .env file not found, using system environment variables")
	}

	// DB setup
	dbHost := os.Getenv("DB_HOST")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbPort := os.Getenv("DB_PORT")

	if dbHost == "" {
		dbHost = "localhost"
		dbUser = "hris_user"
		dbPass = "hris_password"
		dbName = "hris_db"
		dbPort = "5432"
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPass, dbHost, dbPort, dbName)

	db, err := gorm.Open(gormPostgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("Failed to connect test database")
	}

	// Redis setup
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")
	if redisHost == "" {
		redisHost = "localhost"
		redisPort = "6379"
	}

	rdb := redisClient.NewClient(&redisClient.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
	})

	return db, rdb
}

func cleanUpTestDB(db *gorm.DB, rdb *redisClient.Client) {
	db.Where("email LIKE ?", "test_integration_%").Delete(&domain.Employee{})
	rdb.Del(context.Background(), "otp:test_integration_01@goto.com")
}

func setupTestRouter(db *gorm.DB, rdb *redisClient.Client) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Init Repo
	seqRepo := repoPostgres.NewEmployeeSequenceRepository(db)
	empRepo := repoPostgres.NewEmployeeRepository(db)
	compRepo := repoPostgres.NewCompanyRepository(db)
	otpRepo := redis.NewOTPRepository(rdb)

	// Init Usecase
	empUC := usecase.NewEmployeeUsecase(seqRepo, empRepo, compRepo)
	authUC := usecase.NewAuthUsecase(empRepo, otpRepo)

	// Init Handler
	api := router.Group("/api/v1")
	handler.NewEmployeeHandler(api, empUC)
	handler.NewAuthHandler(api, authUC)

	return router
}

// === 2. SCENARIO TESTS ===

func TestAuthIntegration(t *testing.T) {
	db, rdb := setupTestDB()
	router := setupTestRouter(db, rdb)

	// testing data cleanup before and after running the test
	// cleanUpTestDB(db, rdb)
	// defer cleanUpTestDB(db, rdb)

	// State Variables
	testEmail := "test_integration_01@yopmail.com"
	testPhone := "+6289999999999"
	testPassword := "rahasia123"
	var generatedEmployeeID string
	var validOTP string

	// --- A. TEST REGISTRASI ---
	t.Run("1. [Positive] Register Success", func(t *testing.T) {
		reqBody := domain.RegisterRequest{
			CompanyCode:   "GOTO",
			Email:         testEmail,
			PhoneNumber:   "+6289999999999",
			Password:      testPassword,
			IsTncAccepted: true,
		}
		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/register", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var emp domain.Employee
		err := db.Where("email = ?", testEmail).First(&emp).Error
		assert.NoError(t, err)
		assert.NotEmpty(t, emp.EmployeeID)

		generatedEmployeeID = emp.EmployeeID
	})

	t.Run("2. [Edge] Register Duplicate Email", func(t *testing.T) {
		reqBody := domain.RegisterRequest{
			CompanyCode:   "GOTO",
			Email:         testEmail,
			PhoneNumber:   "+6288888888888",
			Password:      "passwordbaru",
			IsTncAccepted: true,
		}
		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/register", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Must failed
		assert.NotEqual(t, http.StatusCreated, w.Code)
	})

	// --- B. TEST REQUEST OTP ---
	t.Run("3. [Positive] Request OTP Success", func(t *testing.T) {
		reqBody := map[string]string{"email": testEmail}
		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/otp/request", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Check via redis
		otp, err := rdb.Get(context.Background(), "otp:"+testEmail).Result()
		assert.NoError(t, err)
		assert.Len(t, otp, 6)

		validOTP = otp
	})

	t.Run("4. [Negative] Request OTP Unregistered Email", func(t *testing.T) {
		reqBody := map[string]string{"email": "test_integration_ngawur@goto.com"}
		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/otp/request", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	// --- C. TEST VERIFY OTP ---
	t.Run("5. [Negative] Verify OTP Wrong Code", func(t *testing.T) {
		reqBody := map[string]string{
			"email": testEmail,
			"otp":   "000000",
		}
		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/otp/verify", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("6. [Positive] Verify OTP Success & Get Token", func(t *testing.T) {
		reqBody := map[string]string{
			"email": testEmail,
			"otp":   validOTP,
		}
		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/otp/verify", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Assert response JSON with token
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response, "token")
	})

	t.Run("7. [Edge] Verify OTP Double Use (Should Fail)", func(t *testing.T) {
		reqBody := map[string]string{
			"email": testEmail,
			"otp":   validOTP,
		}
		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/otp/verify", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// should be unauthorized
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	// --- D. TEST LOGIN PASSWORD ---
	t.Run("8. [Positive] Login with Employee ID", func(t *testing.T) {
		reqBody := domain.LoginRequest{
			EmployeeID: generatedEmployeeID,
			Password:   testPassword,
		}
		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response, "token")
	})

	t.Run("9. [Positive] Login with Email", func(t *testing.T) {
		reqBody := domain.LoginRequest{
			Email:    testEmail,
			Password: testPassword,
		}
		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response, "token")
	})

	t.Run("10. [Positive] Login with Phone Number", func(t *testing.T) {
		reqBody := domain.LoginRequest{
			PhoneNumber: testPhone,
			Password:    testPassword,
		}
		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response, "token")
	})

	t.Run("11. [Negative] Login with Wrong Password", func(t *testing.T) {
		reqBody := domain.LoginRequest{
			EmployeeID: generatedEmployeeID,
			Password:   "wrongpassword",
		}
		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("12. [Negative] Login with No Identifier", func(t *testing.T) {
		reqBody := domain.LoginRequest{
			Password: testPassword,
		}
		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
