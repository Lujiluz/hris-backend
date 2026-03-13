package handler

import (
	"errors"
	"hris-backend/internal/delivery/http/middleware"
	"hris-backend/internal/domain"
	"net/http"

	"github.com/gin-gonic/gin"
)

type EmployeeHandler struct {
	employeeUsecase domain.EmployeeUsecase
}

func NewEmployeeHandler(r *gin.RouterGroup, us domain.EmployeeUsecase) {
	handler := &EmployeeHandler{
		employeeUsecase: us,
	}

	r.POST("/register", handler.Register)

	// GET /employee/profile — JWT-protected
	g := r.Group("/employee")
	g.Use(middleware.JWTAuth())
	g.GET("/profile", handler.GetProfile)
}

// Register godoc
// @Summary Mendaftarkan karyawan baru
// @Description Endpoint untuk mendaftarkan karyawan baru ke dalam sistem HRIS. Membutuhkan Company Code yang valid. Employee ID akan di-generate otomatis.
// @Tags Employee
// @Accept json
// @Produce json
// @Param request body domain.RegisterRequest true "Payload Registrasi Karyawan"
// @Success 201 {object} map[string]interface{} "Berhasil mendaftarkan karyawan"
// @Failure 400 {object} domain.ErrorResponse "Bad Request (validasi gagal)"
// @Failure 409 {object} domain.ErrorResponse "Conflict (email atau phone sudah terdaftar)"
// @Failure 500 {object} map[string]interface{} "Internal Server Error"
// @Router /register [post]
func (h *EmployeeHandler) Register(c *gin.Context) {
	var req domain.RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.employeeUsecase.Register(c.Request.Context(), &req); err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidEmailDomain):
			c.JSON(http.StatusBadRequest, domain.NewFieldError("registration failed", "email", "The email you've provided is invalid"))
		case errors.Is(err, domain.ErrInvalidCompanyCode):
			c.JSON(http.StatusBadRequest, domain.NewFieldError("registration failed", "company_code", "Company code is not registered to any company"))
		case errors.Is(err, domain.ErrEmailAlreadyRegistered):
			c.JSON(http.StatusConflict, domain.NewFieldError("registration failed", "email", "Email is already registered"))
		case errors.Is(err, domain.ErrPhoneAlreadyRegistered):
			c.JSON(http.StatusConflict, domain.NewFieldError("registration failed", "phone_number", "Phone number is already registered"))
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "employee registered successfully"})
}

// GetProfile godoc
// @Summary Get authenticated employee profile
// @Description Returns the profile of the currently authenticated employee: name, email, role, profile picture, and company info.
// @Tags Employee
// @Produce json
// @Security BearerAuth
// @Success 200 {object} domain.EmployeeProfileResponse "Employee profile"
// @Failure 401 {object} map[string]interface{} "Missing or invalid JWT token"
// @Failure 404 {object} map[string]interface{} "Employee not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /employee/profile [get]
func (h *EmployeeHandler) GetProfile(c *gin.Context) {
	employeeID, _, ok := extractClaims(c)
	if !ok {
		return
	}

	resp, err := h.employeeUsecase.GetProfile(c.Request.Context(), employeeID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrEmployeeNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}
