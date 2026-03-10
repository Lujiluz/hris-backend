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

// Register godoc
// @Summary Mendaftarkan karyawan baru
// @Description Endpoint untuk mendaftarkan karyawan baru ke dalam sistem HRIS. Membutuhkan Company Code yang valid. Employee ID akan di-generate otomatis.
// @Tags Employee
// @Accept json
// @Produce json
// @Param request body domain.RegisterRequest true "Payload Registrasi Karyawan"
// @Success 201 {object} map[string]interface{} "Berhasil mendaftarkan karyawan"
// @Failure 400 {object} map[string]interface{} "Bad Request (Format JSON salah atau validasi gagal)"
// @Failure 500 {object} map[string]interface{} "Internal Server Error (Company code tidak valid atau gagal menyimpan data)"
// @Router /register [post]
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
