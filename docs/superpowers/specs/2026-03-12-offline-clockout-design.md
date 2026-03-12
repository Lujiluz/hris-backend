# Offline Clock-Out Support & Server Time Endpoint

**Date:** 2026-03-12
**Status:** Approved

## Overview

Add offline-first clock-out support to the HRIS attendance service. Mobile clients that lose connectivity reconstruct an accurate clock-out timestamp from a previously synced server time and device uptime delta. The backend validates and trusts this reconstructed timestamp within a configurable window.

Two changes are required:

1. A new `GET /api/v1/time` endpoint for server time synchronisation.
2. An updated `POST /api/v1/attendance/clock-out` that accepts an optional `client_timestamp`.

---

## 1. New Endpoint: GET /api/v1/time

**Auth:** JWT required, applied per-route inside `NewUtilityHandler` via `middleware.JWTAuth()`. The `apiV1` group itself has no global middleware; JWT must be applied explicitly.

> **Design note:** The task specifies auth-required for this endpoint. An unauthenticated time endpoint would be more resilient for offline clients with expired tokens, but that is outside the scope of this change.

**Purpose:** Mobile syncs server time on every app open/foreground to anchor offline timestamp reconstruction.

### Response 200

```json
{ "server_time": "2026-03-12T17:00:00+07:00" }
```

Format: RFC3339 with timezone offset via `time.Now().Format(time.RFC3339)`.

### Implementation

- New file: `internal/delivery/http/handler/utility_handler.go`
- Struct: `UtilityHandler{}` — no fields, no dependencies
- Constructor: `func NewUtilityHandler(r *gin.RouterGroup)` — no second argument (differs from other handlers that take a usecase)
- Route: `r.GET("/time", middleware.JWTAuth(), h.GetServerTime)`
- Registered in `main.go`: `handler.NewUtilityHandler(apiV1)`

---

## 2. Updated Endpoint: POST /api/v1/attendance/clock-out

### Request Body (all fields optional)

```json
{
  "client_timestamp": "2026-03-12T17:00:00+07:00",
  "notes": "Leaving early today"
}
```

### Timestamp Resolution (priority order)

1. `client_timestamp` provided → validate → use as `clock_out_at`
2. `client_timestamp` absent → use `time.Now()` (existing behaviour)

### Validation Rules for `client_timestamp`

Capture `now := time.Now()` once at the top of the usecase `ClockOut` method and reuse it for all comparisons and calculations.

| Rule                              | Reference point                                                      | Error response                                                  |
| --------------------------------- | -------------------------------------------------------------------- | --------------------------------------------------------------- |
| Not valid RFC3339                 | —                                                                    | 400 `invalid client_timestamp format, expected RFC3339`         |
| `t.After(now)`                    | `now` captured above; strict, no clock-skew tolerance                | 400 `client_timestamp is in the future`                         |
| `now.Sub(t) > MaxOfflineDuration` | `now` captured above; measured from server now, not from `ClockInAt` | 400 `client_timestamp exceeds max offline duration of 24 hours` |

### Date boundary handling

When `client_timestamp` is provided, the `work_date` of the attendance record must be derived from `clockOutAt`, not from `time.Now()`. This matters when the offline submission spans midnight (e.g., clocked in at 11 pm, submitting clock-out at 12:30 am the next day).

```go
workDate := time.Date(clockOutAt.Year(), clockOutAt.Month(), clockOutAt.Day(), 0, 0, 0, 0, clockOutAt.Location())
```

Pass `workDate` to `GetTodayRecord` instead of `today` when `client_timestamp` is provided. When `client_timestamp` is absent, behaviour is unchanged (use `time.Now()` date as before).

### New Field on Attendance Record

`is_offline_submission bool` — `true` when `client_timestamp` was used, `false` otherwise. Persisted to DB and included in `ClockOutResponse`.

### Notes field

If `notes` is provided in the request (`req.Notes != nil`), persist it. If absent, leave the existing value unchanged. Explicitly setting `notes` to JSON `null` is treated the same as omitting the field (i.e., clearing a note is out of scope for this endpoint).

### GetClockOutPreview — intentionally unchanged

`GetClockOutPreview` always uses `time.Now()`. The preview is an estimate; a slight divergence when an offline timestamp is used is acceptable.

### Existing Error Responses (unchanged)

- 401: missing/invalid JWT
- 404: no record found for the resolved `work_date`
- 409: already clocked out
- 500: server error

---

## 3. Database Migration

File: `migrations/000012_add_offline_clockout.up.sql`

```sql
ALTER TABLE attendance_records
  ADD COLUMN is_offline_submission BOOLEAN NOT NULL DEFAULT FALSE;
```

File: `migrations/000012_add_offline_clockout.down.sql`

```sql
ALTER TABLE attendance_records DROP COLUMN is_offline_submission;
```

---

## 4. Domain Layer Changes (`internal/domain/attendance.go`)

### New constant

```go
const MaxOfflineDuration = 24 * time.Hour
```

Declare as `const` (consistent with `MaxAllowedAccuracyMeters` in the same file; `time.Duration` multiplication by an untyped constant is a valid const expression in Go).

### New error variables

```go
ErrClientTimestampInFuture = errors.New("client_timestamp is in the future")
ErrClientTimestampTooOld   = errors.New("client_timestamp exceeds max offline duration")
ErrInvalidClientTimestamp  = errors.New("invalid client_timestamp format, expected RFC3339")
```

### `AttendanceRecord` struct — new field (add after `OvertimeMinutes`)

```go
IsOfflineSubmission bool `json:"is_offline_submission" gorm:"column:is_offline_submission;not null;default:false"`
```

### New request DTO

```go
type ClockOutRequest struct {
    ClientTimestamp *string `json:"client_timestamp"`
    Notes           *string `json:"notes"`
}
```

### Updated `ClockOutResponse` — add field

```go
type ClockOutResponse struct {
    ClockOutAt          time.Time `json:"clock_out_at"`
    WorkingMinutes      int       `json:"working_minutes"`
    OvertimeMinutes     int       `json:"overtime_minutes"`
    Status              string    `json:"status"`
    IsOfflineSubmission bool      `json:"is_offline_submission"`
}
```

### `AttendanceUsecase` interface in `domain/attendance.go` — update signature

Change:

```go
ClockOut(ctx context.Context, employeeID string, companyID uuid.UUID) (*ClockOutResponse, error)
```

To:

```go
ClockOut(ctx context.Context, employeeID string, companyID uuid.UUID, req ClockOutRequest) (*ClockOutResponse, error)
```

`AttendanceRepository` interface: **no signature change** — `UpdateClockOut` stays record-based (see Section 6).

---

## 5. Usecase Layer Changes (`internal/usecase/attendance_uc.go`)

**Updated method signature:**

```go
func (u *attendanceUsecase) ClockOut(ctx context.Context, employeeID string, companyID uuid.UUID, req domain.ClockOutRequest) (*domain.ClockOutResponse, error)
```

**At the top of the method**, capture server time once and resolve `clockOutAt`:

```go
now := time.Now()
clockOutAt := now
isOffline := false

if req.ClientTimestamp != nil {
    t, err := time.Parse(time.RFC3339, *req.ClientTimestamp)
    if err != nil {
        return nil, domain.ErrInvalidClientTimestamp
    }
    if t.After(now) {
        return nil, domain.ErrClientTimestampInFuture
    }
    if now.Sub(t) > domain.MaxOfflineDuration {
        return nil, domain.ErrClientTimestampTooOld
    }
    clockOutAt = t
    isOffline = true
}

// Derive work date from clockOutAt (handles midnight-spanning offline submissions)
workDate := time.Date(clockOutAt.Year(), clockOutAt.Month(), clockOutAt.Day(), 0, 0, 0, 0, clockOutAt.Location())
```

Replace the existing `today := time.Date(now.Year()...)` with `workDate` for the `GetTodayRecord` call. The actual `GetTodayRecord` signature is `GetTodayRecord(ctx context.Context, employeeID uuid.UUID, date time.Time) (*AttendanceRecord, error)` — two parameters plus context, no `companyID`.

**Auto-close open break:** pass `clockOutAt` (not `now`) to `EndLatestBreak`, so `break_end_at <= clock_out_at` is always maintained.

**Duration calculations:** replace all uses of `now` with `clockOutAt` for working minutes and overtime minutes.

**Populate record before UpdateClockOut:**

```go
record.ClockOutAt     = &clockOutAt
record.Status         = domain.AttendanceStatusClockedOut
record.WorkingMinutes = &workingMinutes
record.OvertimeMinutes = &overtimeMinutes
record.IsOfflineSubmission = isOffline
if req.Notes != nil {
    record.Notes = req.Notes
}
```

**Return value:** include `IsOfflineSubmission` in `ClockOutResponse`.

---

## 6. Repository Layer Changes (`internal/repository/postgres/attendance_repo.go`)

`UpdateClockOut` **signature stays unchanged** (`record *domain.AttendanceRecord`). Extend the explicit map with the two new fields:

```go
func (r *attendanceRepo) UpdateClockOut(ctx context.Context, record *domain.AttendanceRecord) error {
    updates := map[string]interface{}{
        "clock_out_at":          record.ClockOutAt,
        "status":                record.Status,
        "working_minutes":       record.WorkingMinutes,
        "overtime_minutes":      record.OvertimeMinutes,
        "is_offline_submission": record.IsOfflineSubmission,
        "updated_at":            time.Now(),
    }
    if record.Notes != nil {
        updates["notes"] = *record.Notes
    }
    return r.db.WithContext(ctx).Model(record).Updates(updates).Error
}
```

`AttendanceRepository` interface in `domain/attendance.go`: **no change needed**.

---

## 7. Handler Layer Changes

### `utility_handler.go` (new file)

```go
package handler

import (
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "hris-backend/internal/delivery/http/middleware"
)

type UtilityHandler struct{}

func NewUtilityHandler(r *gin.RouterGroup) {
    h := &UtilityHandler{}
    r.GET("/time", middleware.JWTAuth(), h.GetServerTime)
}

func (h *UtilityHandler) GetServerTime(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"server_time": time.Now().Format(time.RFC3339)})
}
```

### `attendance_handler.go` — `ClockOut` method

- Bind `domain.ClockOutRequest` from JSON body; no `binding:"required"` tags
- Call `attendanceUsecase.ClockOut(ctx, employeeID, companyID, req)`
- Add 3 new 400 error cases:
  - `domain.ErrInvalidClientTimestamp` → `"invalid client_timestamp format, expected RFC3339"`
  - `domain.ErrClientTimestampInFuture` → `"client_timestamp is in the future"`
  - `domain.ErrClientTimestampTooOld` → format with `fmt.Sprintf("client_timestamp exceeds max offline duration of %d hours", int(domain.MaxOfflineDuration.Hours()))` — do **not** use `err.Error()` here, since the error variable text does not include the duration value
- Return `ClockOutResponse` including `is_offline_submission`

### `main.go`

```go
handler.NewUtilityHandler(apiV1)
```

Add this alongside existing handler registrations.

---

## Files Changed

| File                                                   | Change                                                                                                                                             |
| ------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------- |
| `migrations/000012_add_offline_clockout.up.sql`        | New — add `is_offline_submission` column                                                                                                           |
| `migrations/000012_add_offline_clockout.down.sql`      | New — drop column                                                                                                                                  |
| `internal/domain/attendance.go`                        | Add constant, errors, struct field with GORM tags, `ClockOutRequest` DTO, update `AttendanceUsecase.ClockOut` interface, update `ClockOutResponse` |
| `internal/usecase/attendance_uc.go`                    | Update `ClockOut` signature, add timestamp resolution block, use `workDate` for record lookup, use `clockOutAt` for break close and duration calcs |
| `internal/repository/postgres/attendance_repo.go`      | Extend `UpdateClockOut` map with `is_offline_submission` and conditional `notes`                                                                   |
| `internal/delivery/http/handler/utility_handler.go`    | New — `UtilityHandler`, `GetServerTime`                                                                                                            |
| `internal/delivery/http/handler/attendance_handler.go` | Update `ClockOut` handler: bind request, pass to usecase, map 3 new errors                                                                         |
| `cmd/api/main.go`                                      | Add `handler.NewUtilityHandler(apiV1)`                                                                                                             |
