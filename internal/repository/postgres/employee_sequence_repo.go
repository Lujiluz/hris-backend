package postgres

import (
	"context"
	"hris-backend/internal/domain"

	"gorm.io/gorm"
)

type employeeSequenceRepo struct {
	db *gorm.DB
}

func NewEmployeeSequenceRepository(db *gorm.DB) domain.EmployeeSequenceRepository {
	return &employeeSequenceRepo{db: db}
}

func (r *employeeSequenceRepo) IncrementAndGetCounter(ctx context.Context, companyID string, year int) (int, error) {
	var counter int

	// Atomic upsert:
	// if no record for this year -> insert counter = 1
	// if record exists for this year -> update counter += 1
	query := `
			INSERT INTO company_employee_sequences (company_id, year, counter)
			VALUES (?, ?, 1)
			ON CONFLICT (company_id, year)
			DO UPDATE SET counter = company_employee_sequences.counter + 1
			RETURNING counter;
	`

	err := r.db.WithContext(ctx).Raw(query, companyID, year).Scan(&counter).Error

	if err != nil {
		return 0, err
	}

	return counter, nil
}
