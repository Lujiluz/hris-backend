# Employee Profile Endpoint — Design Spec

**Date:** 2026-03-12
**Status:** Approved

---

## Overview

Add `GET /api/v1/employee/profile` returning the authenticated employee's profile. Adds three new columns to `employees`: `first_name`, `last_name`, `profile_picture`.

---

## Approach

Extend existing `EmployeeUsecase` and `EmployeeHandler` (Option A).

> **Future note:** If an "edit profile" or "change avatar/profile picture" endpoint is requested, extract profile logic into a dedicated `ProfileUsecase` and `ProfileHandler`.

---

## Database Migration

`migrations/000013_add_profile_fields_to_employees.up.sql`
```sql
ALTER TABLE employees
    ADD COLUMN first_name      VARCHAR(100) NULL,
    ADD COLUMN last_name       VARCHAR(100) NULL,
    ADD COLUMN profile_picture TEXT NOT NULL DEFAULT 'https://res.cloudinary.com/dmvot15pm/image/upload/v1773207988/attendance/selfies/public_id_12345.png';
```

Each column uses its own `ADD COLUMN` keyword in a comma-separated list (consistent with migration 000011 pattern).

`migrations/000013_add_profile_fields_to_employees.down.sql`
```sql
ALTER TABLE employees
    DROP COLUMN first_name,
    DROP COLUMN last_name,
    DROP COLUMN profile_picture;
```

---

## Domain Changes (`internal/domain/employee.go`)

### `Employee` struct — add fields

```go
FirstName      *string `gorm:"type:varchar(100)" json:"first_name"`
LastName       *string `gorm:"type:varchar(100)" json:"last_name"`
ProfilePicture string  `gorm:"type:text;not null;default:'https://res.cloudinary.com/dmvot15pm/image/upload/v1773207988/attendance/selfies/public_id_12345.png'" json:"profile_picture"`
```

`first_name` and `last_name` are nullable — users fill them via a future edit-profile endpoint. `profile_picture` is non-null with the default set at DB level.

### New DTOs

```go
type EmployeeProfileResponse struct {
    EmployeeID     string             `json:"employee_id"`
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

### `EmployeeUsecase` interface — add

```go
GetProfile(ctx context.Context, employeeID string) (*EmployeeProfileResponse, error)
```

### `EmployeeRepository` interface — add

```go
GetProfileByEmployeeID(ctx context.Context, employeeID string) (*Employee, error)
```

A separate method from `GetByEmployeeID` so that `Preload("Company")` is scoped only to the profile path — keeping auth/attendance queries lean.

### Error sentinel — add to `internal/domain/employee.go` (same file as `ErrSelfieAlreadyRegistered`)

```go
var ErrEmployeeNotFound = errors.New("employee not found")
```

---

## Repository (`internal/repository/postgres/employee_repo.go`)

Implement `GetProfileByEmployeeID` using `db.Preload("Company").Where("employee_id = ?", employeeID).First(...)`.

---

## Usecase (`internal/usecase/employee_uc.go`)

`GetProfile`:
1. Call `employeeRepo.GetProfileByEmployeeID(ctx, employeeID)`.
2. Return `ErrEmployeeNotFound` on `gorm.ErrRecordNotFound`.
3. Map to `EmployeeProfileResponse` including `CompanyProfileData`.

---

## Handler (`internal/delivery/http/handler/employee_handler.go`)

Route split inside `NewEmployeeHandler` (mirrors `NewAttendanceHandler` pattern):
- `POST /register` — stays directly on `r` (no middleware, unchanged).
- `GET /profile` — new sub-group: `g := r.Group("/employee"); g.Use(middleware.JWTAuth()); g.GET("/profile", handler.GetProfile)`.

> **Implementation ordering:** `GetProfile` must be added to both the `EmployeeUsecase` interface and `employeeUsecase` struct before the package will compile, since `NewEmployeeHandler` accepts `domain.EmployeeUsecase`.

Handler logic:
1. Extract `employee_id` from JWT via `extractClaims`.
2. Call `employeeUsecase.GetProfile`.
3. Return response.

Include full Swagger annotations (`@Summary`, `@Router`, `@Security BearerAuth`, `@Success`, `@Failure`) consistent with project conventions.

`CompanyProfileData` fields are mapped explicitly in the usecase (not embedded from `Company`) so future additions to `Company` do not silently appear in the profile response.

### HTTP responses

| Status | Condition |
|--------|-----------|
| 200 | Profile returned successfully |
| 401 | Missing or invalid JWT |
| 404 | Employee not found |
| 500 | Unexpected DB error |

---

## Files Touched

| File | Change |
|------|--------|
| `migrations/000013_add_profile_fields_to_employees.up.sql` | New |
| `migrations/000013_add_profile_fields_to_employees.down.sql` | New |
| `internal/domain/employee.go` | Add fields, DTOs, interface methods, error sentinel |
| `internal/repository/postgres/employee_repo.go` | Add `GetProfileByEmployeeID` |
| `internal/usecase/employee_uc.go` | Add `GetProfile` implementation |
| `internal/delivery/http/handler/employee_handler.go` | Add `GetProfile` handler + authenticated sub-group |
| `cmd/api/main.go` | No signature change expected; included for awareness of route wiring |
