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
