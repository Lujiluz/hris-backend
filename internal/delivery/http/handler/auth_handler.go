package handler

import (
	"errors"
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
// @Summary Request a 6-digit OTP via email
// @Description Sends a 6-digit OTP code to the registered email address. OTP expires after 5 minutes.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body domain.RequestOTPRequest true "Email address"
// @Success 200 {object} map[string]interface{} "OTP sent successfully"
// @Failure 400 {object} map[string]interface{} "Invalid request body or email format"
// @Failure 401 {object} map[string]interface{} "Email is not registered"
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
// @Summary Verify OTP and get JWT token
// @Description Validates the 6-digit OTP code and returns a JWT token on success. OTP is single-use and expires after 5 minutes.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body domain.VerifyOTPRequest true "Email and OTP code"
// @Success 200 {object} map[string]interface{} "OTP verified, returns JWT token"
// @Failure 400 {object} map[string]interface{} "Invalid request body"
// @Failure 401 {object} map[string]interface{} "Invalid or expired OTP"
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
// @Summary Login with Employee ID, Email, or Phone Number
// @Description Authenticate using password plus one of: employee_id, email, or phone_number.
// @Description Provide exactly one identifier field alongside the password. Returns a JWT token on success.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body domain.LoginRequest true "Login credentials (one identifier + password)"
// @Success 200 {object} map[string]interface{} "Login successful, returns JWT token"
// @Failure 400 {object} map[string]interface{} "No identifier provided (need employee_id, email, or phone_number)"
// @Failure 401 {object} map[string]interface{} "Account not found or wrong password"
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req domain.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, err := h.authUsecase.Login(c.Request.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrIdentifierRequired):
			c.JSON(http.StatusBadRequest, domain.NewFieldError("login failed", "identifier", "Provide employee_id, email, or phone_number"))
		case errors.Is(err, domain.ErrAccountNotFound):
			field := loginIdentifierField(&req)
			c.JSON(http.StatusUnauthorized, domain.NewFieldError("login failed", field, identifierNotFoundMessage(field)))
		case errors.Is(err, domain.ErrWrongPassword):
			c.JSON(http.StatusUnauthorized, domain.NewFieldError("login failed", "password", "Wrong password"))
		default:
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "login successful", "token": token})
}

// loginIdentifierField returns the JSON field name of whichever identifier was provided.
func loginIdentifierField(req *domain.LoginRequest) string {
	switch {
	case req.EmployeeID != "":
		return "employee_id"
	case req.Email != "":
		return "email"
	default:
		return "phone_number"
	}
}

// identifierNotFoundMessage returns a human-readable message for a missing account.
func identifierNotFoundMessage(field string) string {
	switch field {
	case "email":
		return "Email is not registered to any account"
	case "employee_id":
		return "Employee ID is not registered to any account"
	default:
		return "Phone number is not registered to any account"
	}
}
