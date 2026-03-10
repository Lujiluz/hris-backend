package domain

import (
	"context"
	"errors"
)

var (
	ErrOfficeNotConfigured  = errors.New("office location not configured for this company")
	ErrMockLocationDetected = errors.New("mock/fake GPS location detected")
	ErrGPSAccuracyTooLow    = errors.New("GPS accuracy is too low, move to an open area and retry")
)

// MaxAllowedAccuracyMeters is the worst acceptable GPS accuracy.
// If AccuracyMeters > this value the request is rejected.
const MaxAllowedAccuracyMeters = 50.0

type AttendanceUsecase interface {
	ValidateLocation(ctx context.Context, req *ValidateLocationRequest) (*ValidateLocationResponse, error)
}

type ValidateLocationRequest struct {
	// *float64 distinguishes "field not sent" (nil → 400) from "sent as 0.0" (valid coordinate at equator/prime-meridian)
	Latitude  *float64 `json:"latitude"  binding:"required"`
	Longitude *float64 `json:"longitude" binding:"required"`

	// AccuracyMeters is the GPS accuracy radius reported by the device OS.
	// Required; values > MaxAllowedAccuracyMeters are rejected.
	AccuracyMeters float64 `json:"accuracy_meters" binding:"required,gt=0"`

	// IsMockLocation is set by the mobile client from the device OS API.
	// Android: Location.isMock() (API 31+) / Location.isFromMockProvider() (legacy)
	// iOS: cannot detect programmatically with certainty; send false
	IsMockLocation bool `json:"is_mock_location"`

	CompanyID string // Injected by handler from JWT claims, NOT from JSON body
}

type ValidateLocationResponse struct {
	IsInsideGeofence    bool    `json:"is_inside_geofence"`
	DistanceMeters      float64 `json:"distance_meters"`
	AllowedRadiusMeters float64 `json:"allowed_radius_meters"`
}
