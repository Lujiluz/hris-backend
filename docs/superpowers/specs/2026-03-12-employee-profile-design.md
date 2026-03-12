# Employee Profile Endpoint — Design Spec

**Date:** 2026-03-12
**Status:** Approved

---

## Overview

Add a `GET /api/v1/employee/profile` endpoint that returns the authenticated employee's profile. Also adds three new columns to the `employees` table: `first_name`, `last_name`, and `profile_picture`.

---

## Approach

Option A — extend the existing `EmployeeUsecase` and `EmployeeHandler`. No new usecase or handler files.

> **Future note:** If an "edit profile" or "change avatar/profile picture" endpoint is requested, the profile logic should be extracted into a dedicated `ProfileUsecase` and `ProfileHandler` at that point.

---

## Database Migration

File: `migrations/000013_add_profile_fields_to_employees.up.sql`

```sql
ALTER TABLE employees
  ADD COLUMN first_name      VARCHAR(100) NULL,
  ADD COLUMN last_name       VARCHAR(100) NULL,
  ADD COLUMN profile_picture TEXT         NOT NULL DEFAULT 'https://res.cloudinary.com/dmvot15pm/image/upload/v1773207988/attendance/selfies/public_id_12345.png';
```

File: `migrations/000013_add_profile_fields_to_employees.down.sql`

```sql
ALTER TABLE employees
  DROP COLUMN first_name,
  DROP COLUMN last_name,
  DROP COLUMN profile_picture;
```

---

## Domain Changes

### `Employee` struct (`internal/domain/employee.go`)

Add three fields:

```go
FirstName      *string `gorm:"type:varchar(100)" json:"first_name"`
LastName       *string `gorm:"type:varchar(100)" json:"last_name"`
ProfilePicture string  `gorm:"type:text;not null" json:"profile_picture"`
```

- `first_name` and `last_name` are nullable pointers — no default name is assumed; users fill these in via a future edit-profile endpoint.
- `profile_picture` is non-null with a default Cloudinary URL set at the DB level.

### New DTOs

```go
type EmployeeProfileResponse struct {
    FirstName      *string            `json:"first_name"`
    LastName       *string            `json:"last_name"`
    Email          string             `json:"email"`
    Role           string             `json:"role"`
    ProfilePicture string             `json:"profile_picture"`
    Company        CompanyProfileData `json:"company"`
}

type CompanyProfileData struct {
    CompanyName string `json:"company_name"`
    CompanyCode string `json:"company_code"`
}
```

### `EmployeeUsecase` interface

Add:

```go
GetProfile(ctx context.Context, employeeID string) (*EmployeeProfileResponse, error)
```

---

## Repository

### New method on `EmployeeRepository` interface

```go
GetProfileByEmployeeID(ctx context.Context, employeeID string) (*Employee, error)
```

- Uses GORM `Preload("Company")` to eager-load the company relation.
- The existing `GetByEmployeeID` (used by auth and attendance flows) is left unchanged to avoid unintended side effects.

---

## Usecase

`employeeUsecase.GetProfile`:
1. Calls `employeeRepo.GetProfileByEmployeeID(ctx, employeeID)`.
2. Returns `domain.ErrEmployeeNotFound` if GORM returns `gorm.ErrRecordNotFound`.
3. Maps the result to `EmployeeProfileResponse`, populating `CompanyProfileData` from the preloaded `Company`.

---

## Handler & Route

### Route

```
GET /api/v1/employee/profile
```

Protected by `middleware.JWTAuth()`.

### Handler logic

1. Extract `employee_id` from JWT context using the existing `extractClaims` helper (already defined in `attendance_handler.go`).
2. Call `employeeUsecase.GetProfile(ctx, employeeID)`.
3. Return response.

### HTTP responses

| Status | Condition |
|--------|-----------|
| 200    | Profile returned successfully |
| 401    | Missing or invalid JWT |
| 404    | Employee not found |
| 500    | Unexpected DB error |

---

## Error Sentinel

Add to `internal/domain/errors.go` (or `employee.go`):

```go
var ErrEmployeeNotFound = errors.New("employee not found")
```

---

## Files Touched

| File | Change |
|------|--------|
| `migrations/000013_add_profile_fields_to_employees.up.sql` | New |
| `migrations/000013_add_profile_fields_to_employees.down.sql` | New |
| `internal/domain/employee.go` | Add fields, DTOs, interface method, error sentinel |
| `internal/repository/postgres/employee_repo.go` | Add `GetProfileByEmployeeID` |
| `internal/usecase/employee_uc.go` | Add `GetProfile` implementation |
| `internal/delivery/http/handler/employee_handler.go` | Add `GetProfile` handler + route |
