package domain

import (
	"time"

	"gorm.io/gorm"
)

type Employee struct {
	ID            string         `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	CompanyID     string         `gorm:"type:uuid;not null" json:"company_id"`
	EmployeeID    string         `gorm:"type:varchar(100);uniqueIndex;not null" json:"employee_id"`
	Email         string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	PhoneNumber   string         `gorm:"type:varchar(50);uniqueIndex;not null" json:"phone_number"`
	Password      string         `gorm:"type:varchar(255);not null" json:"-"`
	IsTncAccepted bool           `gorm:"default:false" json:"is_tnc_accepted"`
	Role          string         `gorm:"type:varchar(50);default:'staff'" json:"role"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	// Relation
	Company Company `gorm:"foreignKey:CompanyID" json:"company,omitzero"`
}
