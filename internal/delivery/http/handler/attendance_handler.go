package handler

import (
	"errors"
	"fmt"
	"io"
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
	g.POST("/register", h.RegisterSelfie)
	g.GET("/register", h.GetRegisteredSelfie)
	g.POST("/validate-location", h.ValidateLocation)
	g.POST("/clock-in", h.ClockIn)
	g.POST("/break", h.ToggleBreak)
	g.GET("/today", h.GetTodayStatus)
	g.GET("/clockout-preview", h.GetClockOutPreview)
	g.POST("/clock-out", h.ClockOut)
}

// @Summary Validate employee GPS location against office geofence
// @Description Checks if the employee's GPS coordinates fall within the company's configured office geofence.
// @Description GPS accuracy must be ≤ 50 meters. Mock/fake GPS locations are rejected.
// @Description Call this before clock-in to verify the employee is at the office.
// @Tags Attendance
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body domain.ValidateLocationRequest true "Employee GPS coordinates"
// @Success 200 {object} domain.ValidateLocationResponse "Location validation result"
// @Failure 400 {object} map[string]interface{} "Invalid request body"
// @Failure 401 {object} map[string]interface{} "Missing or invalid JWT token"
// @Failure 403 {object} map[string]interface{} "Mock/fake GPS location detected"
// @Failure 422 {object} map[string]interface{} "GPS accuracy too low (>50m) or office location not configured"
// @Failure 500 {object} map[string]interface{} "Internal server error"
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
// @Description Records the employee's clock-in for today. Only one clock-in per day is allowed.
// @Description The selfie_url should be a Cloudinary URL of the employee's face photo for verification.
// @Description Latitude and longitude are stored for audit purposes.
// @Tags Attendance
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body domain.ClockInRequest true "Clock-in details"
// @Success 201 {object} domain.ClockInResponse "Clock-in recorded successfully"
// @Failure 400 {object} map[string]interface{} "Invalid request body"
// @Failure 401 {object} map[string]interface{} "Missing or invalid JWT token"
// @Failure 409 {object} map[string]interface{} "Already clocked in today"
// @Failure 500 {object} map[string]interface{} "Internal server error"
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
// @Description Toggles a break for the employee. Action must be "start" or "end".
// @Description Cannot start a break if already on break or already clocked out.
// @Description Cannot end a break if not currently on break.
// @Tags Attendance
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body domain.BreakRequest true "Break action: 'start' or 'end'"
// @Success 200 {object} domain.BreakResponse "Break toggled successfully"
// @Failure 400 {object} map[string]interface{} "Invalid action (must be 'start' or 'end')"
// @Failure 401 {object} map[string]interface{} "Missing or invalid JWT token"
// @Failure 404 {object} map[string]interface{} "No clock-in record found for today"
// @Failure 409 {object} map[string]interface{} "Already on break / not on break / already clocked out"
// @Failure 500 {object} map[string]interface{} "Internal server error"
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
// @Description Returns the employee's current attendance state for today.
// @Description Status will be one of: "idle" (not clocked in), "clocked_in" (working), "on_break" (on break), "clocked_out" (done for today).
// @Description Fields like attendance_id, clock_in_at, clock_out_at, and break info are only present when applicable.
// @Tags Attendance
// @Produce json
// @Security BearerAuth
// @Success 200 {object} domain.TodayStatusResponse "Today's attendance status"
// @Failure 401 {object} map[string]interface{} "Missing or invalid JWT token"
// @Failure 500 {object} map[string]interface{} "Internal server error"
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
// @Description Returns an estimate of working minutes and overtime minutes if the employee were to clock out now.
// @Description Break time is subtracted from working minutes. Overtime is calculated against the employee's schedule.
// @Description Use this to show the employee a preview before they confirm clock-out.
// @Tags Attendance
// @Produce json
// @Security BearerAuth
// @Success 200 {object} domain.ClockOutPreview "Clock-out preview with time estimates"
// @Failure 401 {object} map[string]interface{} "Missing or invalid JWT token"
// @Failure 404 {object} map[string]interface{} "No clock-in record found for today"
// @Failure 409 {object} map[string]interface{} "Already clocked out"
// @Failure 500 {object} map[string]interface{} "Internal server error"
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

// @Summary Register employee selfie for face recognition
// @Description Registers a selfie URL (Cloudinary) for the employee. Can only be registered once.
// @Description The selfie is used for face verification during clock-in.
// @Tags Attendance
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body domain.RegisterSelfieRequest true "Cloudinary selfie URL"
// @Success 200 {object} map[string]interface{} "Selfie registered successfully"
// @Failure 400 {object} map[string]interface{} "Invalid request body or URL"
// @Failure 401 {object} map[string]interface{} "Missing or invalid JWT token"
// @Failure 409 {object} map[string]interface{} "Selfie already registered for this employee"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /attendance/register [post]
func (h *AttendanceHandler) RegisterSelfie(c *gin.Context) {
	var req domain.RegisterSelfieRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	employeeID, _, ok := extractClaims(c)
	if !ok {
		return
	}

	if err := h.attendanceUsecase.RegisterSelfie(c.Request.Context(), employeeID, &req); err != nil {
		switch {
		case errors.Is(err, domain.ErrSelfieAlreadyRegistered):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "selfie registered", "selfie_url": req.SelfieURL})
}

// @Summary Get registered selfie for the authenticated employee
// @Description Returns the employee's registered selfie URL and registration timestamp.
// @Description Returns 404 if no selfie has been registered yet.
// @Tags Attendance
// @Produce json
// @Security BearerAuth
// @Success 200 {object} domain.SelfieStatusResponse "Registered selfie info"
// @Failure 401 {object} map[string]interface{} "Missing or invalid JWT token"
// @Failure 404 {object} map[string]interface{} "No selfie registered for this employee"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /attendance/register [get]
func (h *AttendanceHandler) GetRegisteredSelfie(c *gin.Context) {
	employeeID, _, ok := extractClaims(c)
	if !ok {
		return
	}

	resp, err := h.attendanceUsecase.GetRegisteredSelfie(c.Request.Context(), employeeID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrSelfieNotRegistered):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
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
