# Employee Profile Endpoint Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `GET /api/v1/employee/profile` returning the authenticated employee's name, email, role, profile picture, and company info; backed by three new DB columns.

**Architecture:** Extend existing `EmployeeUsecase` and `EmployeeHandler` (Option A). A dedicated `GetProfileByEmployeeID` repo method uses `Preload("Company")` scoped only to the profile path — intentionally separate from `GetByEmployeeID` to keep auth/attendance queries lean (no eager join). Future edit-profile or change-avatar endpoints must be extracted into a dedicated `ProfileUsecase`/`ProfileHandler`.

**Tech Stack:** Go, Gin, GORM, PostgreSQL, golang-migrate, testify/mock

---

## File Map

| File | Action |
|------|--------|
| `migrations/000013_add_profile_fields_to_employees.up.sql` | Create |
| `migrations/000013_add_profile_fields_to_employees.down.sql` | Create |
| `internal/domain/employee.go` | Modify — struct fields, DTOs, interface methods, error sentinel |
| `internal/repository/postgres/employee_repo.go` | Modify — add `GetProfileByEmployeeID` |
| `internal/usecase/employee_uc.go` | Modify — add `GetProfile` |
| `internal/usecase/employee_uc_test.go` | Modify — add mock method + `TestGetProfile` |
| `internal/delivery/http/handler/employee_handler.go` | Modify — add `GetProfile` handler + JWT-protected route |

---

## Chunk 1: Migration + Domain

### Task 1: Create migration files

**Files:**
- Create: `migrations/000013_add_profile_fields_to_employees.up.sql`
- Create: `migrations/000013_add_profile_fields_to_employees.down.sql`

- [ ] **Step 1: Create up migration**

```sql
-- migrations/000013_add_profile_fields_to_employees.up.sql
ALTER TABLE employees
    ADD COLUMN first_name      VARCHAR(100) NULL,
    ADD COLUMN last_name       VARCHAR(100) NULL,
    ADD COLUMN profile_picture TEXT NOT NULL DEFAULT 'https://res.cloudinary.com/dmvot15pm/image/upload/v1773207988/attendance/selfies/public_id_12345.png';
```

- [ ] **Step 2: Create down migration**

```sql
-- migrations/000013_add_profile_fields_to_employees.down.sql
ALTER TABLE employees
    DROP COLUMN first_name,
    DROP COLUMN last_name,
    DROP COLUMN profile_picture;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/000013_add_profile_fields_to_employees.up.sql \
        migrations/000013_add_profile_fields_to_employees.down.sql
git commit -m "feat(migration): add first_name, last_name, profile_picture to employees"
```

---

### Task 2: Update domain + update mock

**Files:**
- Modify: `internal/domain/employee.go`
- Modify: `internal/usecase/employee_uc_test.go`

> **Note:** The mock must be updated in the same step as the interface — otherwise `go test ./internal/...` fails to compile until Task 5. This is an intentional mid-chunk broken window that gets fixed in Task 5.

- [ ] **Step 1: Add fields to `Employee` struct**

In `internal/domain/employee.go`, add these three fields after `SelfieRegisteredAt`:

```go
FirstName      *string `gorm:"type:varchar(100)" json:"first_name"`
LastName       *string `gorm:"type:varchar(100)" json:"last_name"`
ProfilePicture string  `gorm:"type:text;not null;default:'https://res.cloudinary.com/dmvot15pm/image/upload/v1773207988/attendance/selfies/public_id_12345.png'" json:"profile_picture"`
```

> **Important:** After `Register` creates an employee, GORM does not re-read DB-level defaults — so the in-memory struct's `ProfilePicture` field will be an empty string after `Create`. Always read `ProfilePicture` from DB (e.g. via `GetProfileByEmployeeID`), never from the just-created struct.

- [ ] **Step 2: Add DTOs**

Add after the `LoginRequest` struct:

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

- [ ] **Step 3: Add `GetProfile` to `EmployeeUsecase` interface**

```go
GetProfile(ctx context.Context, employeeID string) (*EmployeeProfileResponse, error)
```

- [ ] **Step 4: Add `GetProfileByEmployeeID` to `EmployeeRepository` interface**

```go
// GetProfileByEmployeeID fetches an employee with Company preloaded.
// Kept separate from GetByEmployeeID to avoid a JOIN on the auth/attendance hot path.
GetProfileByEmployeeID(ctx context.Context, employeeID string) (*Employee, error)
```

- [ ] **Step 5: Add `ErrEmployeeNotFound` sentinel in `employee.go`** (alongside `ErrSelfieAlreadyRegistered`)

```go
var ErrEmployeeNotFound = errors.New("employee not found")
```

- [ ] **Step 6: Add `GetProfileByEmployeeID` to `MockEmployeeRepo` in test file**

In `internal/usecase/employee_uc_test.go`, add to `MockEmployeeRepo`:

```go
func (m *MockEmployeeRepo) GetProfileByEmployeeID(ctx context.Context, employeeID string) (*domain.Employee, error) {
	args := m.Called(ctx, employeeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Employee), args.Error(1)
}
```

- [ ] **Step 7: Verify domain compiles in isolation**

```bash
go build ./internal/domain/...
```

Expected: no errors. (`employeeUsecase` doesn't implement `GetProfile` yet — that's fine; only the `domain` package is checked here.)

- [ ] **Step 8: Commit**

```bash
git add internal/domain/employee.go internal/usecase/employee_uc_test.go
git commit -m "feat(domain): add profile fields, DTOs, interfaces, ErrEmployeeNotFound; update mock"
```

---

## Chunk 2: Repository + Usecase (TDD)

### Task 3: Implement repository method

**Files:**
- Modify: `internal/repository/postgres/employee_repo.go`

- [ ] **Step 1: Add `GetProfileByEmployeeID`**

```go
func (r *employeeRepo) GetProfileByEmployeeID(ctx context.Context, employeeID string) (*domain.Employee, error) {
	var employee domain.Employee
	err := r.db.WithContext(ctx).
		Preload("Company").
		Where("employee_id = ?", employeeID).
		First(&employee).Error
	if err != nil {
		return nil, err
	}
	return &employee, nil
}
```

- [ ] **Step 2: Build**

```bash
go build ./internal/repository/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/repository/postgres/employee_repo.go
git commit -m "feat(repo): add GetProfileByEmployeeID with Company preload"
```

---

### Task 4: Write failing usecase test

**Files:**
- Modify: `internal/usecase/employee_uc_test.go`

- [ ] **Step 1: Add `TestGetProfile`**

Add `"gorm.io/gorm"` to the test file's imports, then add:

```go
func TestGetProfile(t *testing.T) {
	profilePic := "https://res.cloudinary.com/dmvot15pm/image/upload/v1773207988/attendance/selfies/public_id_12345.png"
	firstName := "Budi"
	lastName := "Santoso"

	mockEmployee := &domain.Employee{
		EmployeeID:     "ARYA-2026-0001",
		FirstName:      &firstName,
		LastName:       &lastName,
		Email:          "budi@gmail.com",
		Role:           "staff",
		ProfilePicture: profilePic,
		Company: domain.Company{
			CompanyName: "Arya Corp",
			CompanyCode: "ARYA",
		},
	}

	t.Run("Success", func(t *testing.T) {
		mockEmp := new(MockEmployeeRepo)
		mockEmp.On("GetProfileByEmployeeID", mock.Anything, "ARYA-2026-0001").Return(mockEmployee, nil)

		uc := usecase.NewEmployeeUsecase(nil, mockEmp, nil)
		resp, err := uc.GetProfile(context.Background(), "ARYA-2026-0001")

		assert.NoError(t, err)
		assert.Equal(t, "ARYA-2026-0001", resp.EmployeeID)
		assert.Equal(t, &firstName, resp.FirstName)
		assert.Equal(t, &lastName, resp.LastName)
		assert.Equal(t, "budi@gmail.com", resp.Email)
		assert.Equal(t, "staff", resp.Role)
		assert.Equal(t, profilePic, resp.ProfilePicture)
		assert.Equal(t, "Arya Corp", resp.Company.CompanyName)
		assert.Equal(t, "ARYA", resp.Company.CompanyCode)
		mockEmp.AssertExpectations(t)
	})

	t.Run("Not found", func(t *testing.T) {
		mockEmp := new(MockEmployeeRepo)
		mockEmp.On("GetProfileByEmployeeID", mock.Anything, "UNKNOWN").Return(nil, gorm.ErrRecordNotFound)

		uc := usecase.NewEmployeeUsecase(nil, mockEmp, nil)
		resp, err := uc.GetProfile(context.Background(), "UNKNOWN")

		assert.Nil(t, resp)
		assert.ErrorIs(t, err, domain.ErrEmployeeNotFound)
		mockEmp.AssertExpectations(t)
	})

	t.Run("DB error", func(t *testing.T) {
		mockEmp := new(MockEmployeeRepo)
		dbErr := errors.New("connection lost")
		mockEmp.On("GetProfileByEmployeeID", mock.Anything, "ARYA-2026-0001").Return(nil, dbErr)

		uc := usecase.NewEmployeeUsecase(nil, mockEmp, nil)
		resp, err := uc.GetProfile(context.Background(), "ARYA-2026-0001")

		assert.Nil(t, resp)
		assert.ErrorIs(t, err, dbErr)
		mockEmp.AssertExpectations(t)
	})
}
```

- [ ] **Step 2: Run — expect compile failure**

```bash
go test ./internal/usecase/ -run TestGetProfile -v
```

Expected: compile error — `employeeUsecase does not implement domain.EmployeeUsecase (missing GetProfile method)`.

---

### Task 5: Implement usecase

**Files:**
- Modify: `internal/usecase/employee_uc.go`

- [ ] **Step 1: Add `GetProfile`** and add `"gorm.io/gorm"` to imports

```go
func (uc *employeeUsecase) GetProfile(ctx context.Context, employeeID string) (*domain.EmployeeProfileResponse, error) {
	employee, err := uc.employeeRepo.GetProfileByEmployeeID(ctx, employeeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrEmployeeNotFound
		}
		return nil, err
	}

	return &domain.EmployeeProfileResponse{
		EmployeeID:     employee.EmployeeID,
		FirstName:      employee.FirstName,
		LastName:       employee.LastName,
		Email:          employee.Email,
		Role:           employee.Role,
		ProfilePicture: employee.ProfilePicture,
		Company: domain.CompanyProfileData{
			CompanyName: employee.Company.CompanyName,
			CompanyCode: employee.Company.CompanyCode,
		},
	}, nil
}
```

- [ ] **Step 2: Run tests — expect all pass**

```bash
go test ./internal/usecase/ -run TestGetProfile -v
```

Expected: all 3 sub-tests PASS.

- [ ] **Step 3: Run full usecase suite — no regressions**

```bash
go test ./internal/usecase/ -v
```

Expected: all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/usecase/employee_uc.go internal/usecase/employee_uc_test.go
git commit -m "feat(usecase): add GetProfile with tests"
```

---

## Chunk 3: Handler

### Task 6: Add handler and route

**Files:**
- Modify: `internal/delivery/http/handler/employee_handler.go`

- [ ] **Step 1: Add JWT-protected sub-group in `NewEmployeeHandler`**

`POST /register` stays on `r` (public, unchanged). Add below it:

```go
// GET /employee/profile — JWT-protected
g := r.Group("/employee")
g.Use(middleware.JWTAuth())
g.GET("/profile", handler.GetProfile)
```

Add import `"hris-backend/internal/delivery/http/middleware"` if not already present.

- [ ] **Step 2: Add `GetProfile` handler method**

```go
// GetProfile godoc
// @Summary Get authenticated employee profile
// @Description Returns the profile of the currently authenticated employee: name, email, role, profile picture, and company info.
// @Tags Employee
// @Produce json
// @Security BearerAuth
// @Success 200 {object} domain.EmployeeProfileResponse "Employee profile"
// @Failure 401 {object} map[string]interface{} "Missing or invalid JWT token"
// @Failure 404 {object} map[string]interface{} "Employee not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /employee/profile [get]
func (h *EmployeeHandler) GetProfile(c *gin.Context) {
	employeeID, _, ok := extractClaims(c)
	if !ok {
		return
	}

	resp, err := h.employeeUsecase.GetProfile(c.Request.Context(), employeeID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrEmployeeNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}
```

- [ ] **Step 3: Build the entire project**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Run all unit tests**

```bash
go test -v ./internal/...
```

Expected: all tests PASS.

- [ ] **Step 5: Regenerate Swagger docs**

```bash
swag init -g cmd/api/main.go --parseDependency --parseInternal
```

Expected: `docs/` updated. The new endpoint appears under the Employee tag at `GET /api/v1/employee/profile` (BasePath `/api/v1` is set in `main.go`).

- [ ] **Step 6: Commit**

```bash
git add internal/delivery/http/handler/employee_handler.go docs/
git commit -m "feat(handler): add GET /employee/profile endpoint"
```

---

## Smoke Test (manual)

After running the server (`go run cmd/api/main.go`) and applying the migration:

```bash
# 1. Login to get a token
TOKEN=$(curl -s -X POST http://localhost:3030/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"employee_id":"<your-id>","password":"<your-pass>"}' | jq -r '.token')

# 2. Call profile endpoint
curl -s http://localhost:3030/api/v1/employee/profile \
  -H "Authorization: Bearer $TOKEN" | jq .
```

Expected response shape:
```json
{
  "employee_id": "ARYA-2026-0001",
  "first_name": null,
  "last_name": null,
  "email": "budi@gmail.com",
  "role": "staff",
  "profile_picture": "https://res.cloudinary.com/...",
  "company": {
    "company_name": "Arya Corp",
    "company_code": "ARYA"
  }
}
```
