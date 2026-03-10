package handler

import (
	"errors"
	"net/http"

	"hris-backend/internal/delivery/http/middleware"
	"hris-backend/internal/domain"

	"github.com/gin-gonic/gin"
)

type AttendanceHandler struct {
	attendanceUsecase domain.AttendanceUsecase
}

func NewAttendanceHandler(r *gin.RouterGroup, uc domain.AttendanceUsecase) {
	h := &AttendanceHandler{attendanceUsecase: uc}
	g := r.Group("/attendance")
	g.Use(middleware.JWTAuth())
	g.POST("/validate-location", h.ValidateLocation)
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
