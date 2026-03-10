package domain

import (
	"time"

	"gorm.io/gorm"
)

type Company struct {
	ID                 string         `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	CompanyName        string         `gorm:"type:varchar(255);not null" json:"company_name"`
	CompanyCode        string         `gorm:"type:varchar(50);uniqueIndex;not null" json:"company_code"`
	CompanyPhoneNumber string         `gorm:"type:varchar(50)" json:"company_phone_number"`
	OfficeLatitude     *float64       `gorm:"type:double precision" json:"office_latitude,omitempty"`
	OfficeLongitude    *float64       `gorm:"type:double precision" json:"office_longitude,omitempty"`
	OfficeRadiusMeters *float64       `gorm:"type:double precision" json:"office_radius_meters,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
}
