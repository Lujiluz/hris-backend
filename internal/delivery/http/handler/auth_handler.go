package handler

import (
	"net/http"

	"hris-backend/internal/domain"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authUsecase domain.AuthUsecase
}

func NewAuthHandler(r *gin.RouterGroup, us domain.AuthUsecase) {
	handler := &AuthHandler{
		authUsecase: us,
	}

	authGroup := r.Group("/auth")
	{
		authGroup.POST("/otp/request", handler.RequestOTP)
		authGroup.POST("/otp/verify", handler.VerifyOTP)
		authGroup.POST("/login", handler.Login)
	}
}

// RequestOTP godoc
// @Summary Meminta kode OTP via Email
// @Description Endpoint untuk mengirimkan 6 digit OTP ke email yang terdaftar
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body domain.RequestOTPRequest true "Payload Request OTP"
// @Success 200 {object} map[string]interface{} "OTP berhasil dikirim"
// @Failure 400 {object} map[string]interface{} "Bad Request"
// @Failure 401 {object} map[string]interface{} "Email tidak terdaftar"
// @Router /auth/otp/request [post]
func (h *AuthHandler) RequestOTP(c *gin.Context) {
	var req domain.RequestOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.authUsecase.RequestOTP(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "OTP has been sent to your email"})
}

// VerifyOTP godoc
// @Summary Verifikasi kode OTP dan Dapatkan Token
// @Description Endpoint untuk memvalidasi OTP dan mengembalikan JWT Token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body domain.VerifyOTPRequest true "Payload Verify OTP"
// @Success 200 {object} map[string]interface{} "Berhasil verifikasi, mengembalikan token"
// @Failure 400 {object} map[string]interface{} "Bad Request"
// @Failure 401 {object} map[string]interface{} "OTP salah atau kadaluarsa"
// @Router /auth/otp/verify [post]
func (h *AuthHandler) VerifyOTP(c *gin.Context) {
	var req domain.VerifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, err := h.authUsecase.VerifyOTP(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "OTP verified successfully",
		"token":   token,
	})
}

// Login godoc
// @Summary Login menggunakan Employee ID, Email, atau Nomor Telepon
// @Description Endpoint untuk mendapatkan JWT Token. Isi salah satu dari: employee_id, email, atau phone_number beserta password.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body domain.LoginRequest true "Payload Login"
// @Success 200 {object} map[string]interface{} "Berhasil login, mengembalikan token"
// @Failure 400 {object} map[string]interface{} "Bad Request"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req domain.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, err := h.authUsecase.Login(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "login successful", "token": token})
}
