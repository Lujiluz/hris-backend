# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

**Run the server:**
```bash
go run cmd/api/main.go
```

**Run unit tests:**
```bash
go test -v ./internal/...
```

**Run integration tests** (requires live PostgreSQL + Redis):
```bash
go test -v ./tests/...
```

**Run a single test:**
```bash
go test -v ./internal/usecase/ -run TestEmployeeUsecase
```

**Generate Swagger docs** (run before building if handlers changed):
```bash
swag init -g cmd/api/main.go --parseDependency --parseInternal
```

**Run database migrations** (using golang-migrate):
```bash
migrate -path migrations/ -database "postgres://user:pass@host:port/dbname?sslmode=disable" up
```

**Docker (local development):**
```bash
docker compose up -d        # Start PostgreSQL + Redis + API
docker compose down         # Stop all services
```

## Architecture

This project follows **Clean Architecture** with strict inward dependency flow:

```
Delivery (HTTP) → Usecase → Repository → Domain
```

**Layer responsibilities:**
- `internal/domain/` — Core entities, interfaces (Repository + Usecase contracts), and DTOs. No external dependencies.
- `internal/usecase/` — Business logic. Depends only on domain interfaces.
- `internal/repository/postgres/` and `internal/repository/redis/` — GORM/Redis implementations of domain repository interfaces.
- `internal/delivery/http/handler/` — Gin HTTP handlers. Bind requests, call usecases, return JSON responses.
- `cmd/api/main.go` — Wires all layers via constructor injection.

**Adding a new feature:** Define interfaces in `domain/`, implement in `repository/`, write logic in `usecase/`, expose via handler in `delivery/`.

## Key Conventions

- **Dependency injection**: All constructors (e.g., `NewEmployeeUsecase(seqRepo, empRepo, compRepo)`) receive interfaces, never concrete types. Wire everything in `main.go`.
- **Request validation**: Use Gin struct binding tags (`binding:"required"`, `binding:"email"`, etc.) in domain DTOs.
- **Atomic employee ID**: Employee IDs are generated via PostgreSQL `ON CONFLICT...DO UPDATE SET counter = counter + 1 RETURNING counter` in `EmployeeSequenceRepository` — do not replicate with application-level locking.
- **OTP storage**: Redis keys use format `otp:<email>` with 5-minute TTL.
- **Password hashing**: Always use `bcrypt.DefaultCost`; passwords are tagged `json:"-"` in structs.
- **Swagger annotations**: Add `// @Summary`, `// @Router` etc. to handlers, then regenerate with `swag init`.
- **Set `APP_ENV=production`** to suppress Gin debug output.

## Database

- **ORM**: GORM with PostgreSQL driver
- **Migrations**: golang-migrate, SQL files in `migrations/` (numbered `000001_*.up.sql` / `000002_*.down.sql`)
- **Seeder**: `migrations/000003_seed_initial_company.up.sql` inserts initial company data using `ON CONFLICT DO NOTHING`
- **Docker ports**: PostgreSQL on `5433`, Redis on `6380` (non-standard to avoid conflicts)
