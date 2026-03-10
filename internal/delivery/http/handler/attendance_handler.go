package handler

import (
	"errors"
	"net/http"

	"hris-backend/internal/delivery/http/middleware"
	"hris-backend/internal/domain"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AttendanceHandler struct {
	attendanceUsecase domain.AttendanceUsecase
}

func NewAttendanceHandler(r *gin.RouterGroup, uc domain.AttendanceUsecase) {
	h := &AttendanceHandler{attendanceUsecase: uc}
	g := r.Group("/attendance")
	g.Use(middleware.JWTAuth())
	g.POST("/validate-location", h.ValidateLocation)
	g.POST("/clock-in", h.ClockIn)
	g.POST("/break", h.ToggleBreak)
	g.GET("/today", h.GetTodayStatus)
	g.GET("/clockout-preview", h.GetClockOutPreview)
	g.POST("/clock-out", h.ClockOut)
}

// @Summary Validate employee GPS location against office geofence
// @Tags Attendance
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body domain.ValidateLocationRequest true "Employee GPS coordinates"
// @Success 200 {object} domain.ValidateLocationResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 422 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /attendance/validate-location [post]
func (h *AttendanceHandler) ValidateLocation(c *gin.Context) {
	var req domain.ValidateLocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	companyID, ok := c.Get("company_id")
	if !ok || companyID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "company_id not found in token"})
		return
	}
	req.CompanyID = companyID.(string)

	resp, err := h.attendanceUsecase.ValidateLocation(c.Request.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrMockLocationDetected):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, domain.ErrGPSAccuracyTooLow):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case errors.Is(err, domain.ErrOfficeNotConfigured):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

// @Summary Clock in for today
// @Tags Attendance
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body domain.ClockInRequest true "Clock-in details"
// @Success 201 {object} domain.ClockInResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /attendance/clock-in [post]
func (h *AttendanceHandler) ClockIn(c *gin.Context) {
	var req domain.ClockInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	employeeID, companyID, ok := extractClaims(c)
	if !ok {
		return
	}
	req.EmployeeID = employeeID
	req.CompanyID = companyID

	resp, err := h.attendanceUsecase.ClockIn(c.Request.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrAlreadyClockedIn):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// @Summary Start or end a break
// @Tags Attendance
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body domain.BreakRequest true "Break action (start or end)"
// @Success 200 {object} domain.BreakResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /attendance/break [post]
func (h *AttendanceHandler) ToggleBreak(c *gin.Context) {
	var req domain.BreakRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	employeeID, _, ok := extractClaims(c)
	if !ok {
		return
	}
	req.EmployeeID = employeeID

	resp, err := h.attendanceUsecase.ToggleBreak(c.Request.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidBreakAction):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, domain.ErrNotClockedIn):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, domain.ErrAlreadyClockedOut),
			errors.Is(err, domain.ErrAlreadyOnBreak),
			errors.Is(err, domain.ErrNotOnBreak):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

// @Summary Get today's attendance status
// @Tags Attendance
// @Produce json
// @Security BearerAuth
// @Success 200 {object} domain.TodayStatusResponse
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /attendance/today [get]
func (h *AttendanceHandler) GetTodayStatus(c *gin.Context) {
	employeeID, _, ok := extractClaims(c)
	if !ok {
		return
	}

	resp, err := h.attendanceUsecase.GetTodayStatus(c.Request.Context(), employeeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// @Summary Get clock-out preview (working time and overtime estimate)
// @Tags Attendance
// @Produce json
// @Security BearerAuth
// @Success 200 {object} domain.ClockOutPreview
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /attendance/clockout-preview [get]
func (h *AttendanceHandler) GetClockOutPreview(c *gin.Context) {
	employeeID, companyID, ok := extractClaims(c)
	if !ok {
		return
	}

	resp, err := h.attendanceUsecase.GetClockOutPreview(c.Request.Context(), employeeID, companyID)
	if err != nil {
		switch {
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

// @Summary Clock out for today
// @Tags Attendance
// @Produce json
// @Security BearerAuth
// @Success 200 {object} domain.ClockOutResponse
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /attendance/clock-out [post]
func (h *AttendanceHandler) ClockOut(c *gin.Context) {
	employeeID, companyID, ok := extractClaims(c)
	if !ok {
		return
	}

	resp, err := h.attendanceUsecase.ClockOut(c.Request.Context(), employeeID, companyID)
	if err != nil {
		switch {
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

// extractClaims parses employee_id (string code) and company_id (UUID) from JWT context.
// Returns false and writes a 401 if either is missing or unparseable.
func extractClaims(c *gin.Context) (employeeID string, companyID uuid.UUID, ok bool) {
	empIDRaw, exists := c.Get("employee_id")
	if !exists || empIDRaw == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "employee_id not found in token"})
		return "", uuid.Nil, false
	}
	compIDRaw, exists := c.Get("company_id")
	if !exists || compIDRaw == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "company_id not found in token"})
		return "", uuid.Nil, false
	}

	employeeID, ok = empIDRaw.(string)
	if !ok || employeeID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid employee_id in token"})
		return "", uuid.Nil, false
	}
	companyID, err := uuid.Parse(compIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid company_id in token"})
		return "", uuid.Nil, false
	}
	return employeeID, companyID, true
}
