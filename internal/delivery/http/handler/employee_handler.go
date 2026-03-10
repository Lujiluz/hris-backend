package handler

import (
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
}

func (h *EmployeeHandler) Register(c *gin.Context) {
	var req domain.RegisterRequest

	// use c.ShouldBindJSON to handle validation by struct
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"Error": err.Error()})
		return
	}

	if err := h.employeeUsecase.Register(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"Error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "employee registered successfully"})
}
