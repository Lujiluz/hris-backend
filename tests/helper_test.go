package tests

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"gorm.io/gorm"
)

type MockUser struct {
	Email          string
	PhoneNumber    string
	RandomPassword string
}

func Random13DigitsNumber() string {
	const digits = "0123456789"
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	result := make([]byte, 13)
	for i := range result {
		result[i] = digits[random.Intn(len(digits))]
	}
	return string(result)
}

func SetupMockUser(testType string, emailPrefix string) MockUser {
	emailParts := []string{emailPrefix, testType, time.Now().UTC().Format(time.RFC3339)}
	fullEmail := fmt.Sprintf("%s@yopmail.com", strings.Join(emailParts, "_"))

	// setup random password
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		panic(err)
	}

	randomString := base64.URLEncoding.EncodeToString(bytes)

	return MockUser{
		Email:          fullEmail,
		PhoneNumber:    Random13DigitsNumber(),
		RandomPassword: randomString,
	}
}

// SeedEmployeeSchedule inserts a Mon–Fri 08:00–17:00 work schedule for the
// employee identified by their UUID (employees.id, not the generated employee_id code).
// Uses ON CONFLICT DO NOTHING so calling it multiple times is safe.
// clockInHour/clockOutHour let callers override the schedule times for overtime tests.
func SeedEmployeeSchedule(db *gorm.DB, employeeUUID string, clockInHour, clockOutHour int) {
	for day := 1; day <= 5; day++ { // Mon=1 … Fri=5
		db.Exec(`
			INSERT INTO employee_schedules (id, employee_id, company_id, day_of_week, clock_in_time, clock_out_time, is_active)
			SELECT gen_random_uuid(), id, company_id, ?, ?, ?, true
			FROM employees WHERE id = ?
			ON CONFLICT (employee_id, day_of_week) DO NOTHING`,
			day,
			fmt.Sprintf("%02d:00:00", clockInHour),
			fmt.Sprintf("%02d:00:00", clockOutHour),
			employeeUUID,
		)
	}
}
