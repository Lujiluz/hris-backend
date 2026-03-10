package usecase

import (
	"context"
	"hris-backend/internal/domain"
	"hris-backend/pkg/geo"
)

type attendanceUsecase struct {
	companyRepo domain.CompanyRepository
}

func NewAttendanceUsecase(companyRepo domain.CompanyRepository) domain.AttendanceUsecase {
	return &attendanceUsecase{companyRepo: companyRepo}
}

func (uc *attendanceUsecase) ValidateLocation(ctx context.Context, req *domain.ValidateLocationRequest) (*domain.ValidateLocationResponse, error) {
	// 1. Reject fake GPS immediately
	if req.IsMockLocation {
		return nil, domain.ErrMockLocationDetected
	}

	// 2. Reject poor GPS signal (likely network-estimated or spoofed)
	if req.AccuracyMeters > domain.MaxAllowedAccuracyMeters {
		return nil, domain.ErrGPSAccuracyTooLow
	}

	// 3. Fetch company office location
	company, err := uc.companyRepo.GetByID(ctx, req.CompanyID)
	if err != nil {
		return nil, err
	}

	if company.OfficeLatitude == nil || company.OfficeLongitude == nil || company.OfficeRadiusMeters == nil {
		return nil, domain.ErrOfficeNotConfigured
	}

	// 4. Compute distance and evaluate geofence
	distance := geo.Haversine(*req.Latitude, *req.Longitude, *company.OfficeLatitude, *company.OfficeLongitude)

	return &domain.ValidateLocationResponse{
		IsInsideGeofence:    distance <= *company.OfficeRadiusMeters,
		DistanceMeters:      distance,
		AllowedRadiusMeters: *company.OfficeRadiusMeters,
	}, nil
}
