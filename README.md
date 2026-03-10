# 🚀 HRIS Backend API

> A robust, scalable, and secure **Human Resource Information System** backend API — built with **Golang** using **Clean Architecture** principles.

This backend serves as the core engine for the HRIS Mobile Application, expertly crafted by my partner in crime: [@kub1ga](https://github.com/kub1ga).

---

## ✨ Key Features

| Feature                        | Description                                                                                  |
| ------------------------------ | -------------------------------------------------------------------------------------------- |
| 🏛️ **Clean Architecture**      | Strictly divided into Domain, Repository, Usecase, and Delivery layers                       |
| 🔒 **Secure Authentication**   | JWT sessions, Multi-factor login (Password + TTL Email OTP via Redis), Bcrypt hashing        |
| 🆔 **Race-Condition Safe IDs** | Atomic upsert transactions for sequential Employee IDs — format: `COMPANY_CODE-YEAR-COUNTER` |
| 🧪 **Comprehensive Testing**   | High-coverage integration tests with positive, negative, and edge cases                      |
| 📖 **Interactive API Docs**    | Auto-generated Swagger UI                                                                    |

---

## 🛠️ Tech Stack

| Category           | Technology                     |
| ------------------ | ------------------------------ |
| **Language**       | Go 1.21+                       |
| **Framework**      | Gin Web Framework              |
| **Database**       | PostgreSQL (with GORM)         |
| **Caching / OTP**  | Redis                          |
| **Authentication** | JWT (`golang-jwt/v5`) & Bcrypt |
| **Email**          | Gomail.v2 (SMTP)               |
| **Documentation**  | Swaggo (`gin-swagger`)         |
| **Testing**        | `net/http/httptest`, Testify   |
| **Infrastructure** | Docker & Docker Compose        |

---

## 📂 Project Structure

```
hris-backend/
├── cmd/
│   └── api/              # Application entry point (main.go)
├── docs/                 # Auto-generated Swagger documentation
├── internal/
│   ├── delivery/         # HTTP Handlers (Gin routes and controllers)
│   ├── domain/           # Core business entities, DTOs, and Interfaces
│   ├── repository/       # Database and Redis implementations
│   └── usecase/          # Core business logic and rules
├── migrations/           # SQL migration files
├── pkg/                  # Reusable utilities (JWT helper, Mailer, etc.)
└── tests/                # Integration and End-to-End tests
```

---

## 🚀 Getting Started

### Prerequisites

Make sure you have the following installed:

- [Docker & Docker Compose](https://docs.docker.com/get-docker/)
- [Go 1.21+](https://go.dev/dl/) _(only if running locally without Docker)_
- [golang-migrate CLI](https://github.com/golang-migrate/migrate)

### Installation & Setup

**1. Clone the repository**

```bash
git clone https://github.com/YOUR_USERNAME/hris-backend.git
cd hris-backend
```

**2. Configure environment variables**

```bash
cp .env.example .env
# Fill in your credentials inside .env
```

**3. Spin up the infrastructure**

```bash
docker-compose up -d
```

**4. Run database migrations**

```bash
migrate -path migrations \
  -database "postgres://hris_user:hris_password@localhost:5432/hris_db?sslmode=disable" up
```

**5. Start the application**

```bash
go run cmd/api/main.go
```

---

## 📚 API Documentation

Once the server is running, open your browser and navigate to:

```
http://localhost:3030/swagger/index.html
```

To regenerate docs after modifying the code:

```bash
swag init -g cmd/api/main.go --parseDependency --parseInternal
```

---

## 🧪 Running Tests

The project includes robust integration tests that safely interact with the local database and Redis without affecting actual data.

```bash
go test -v ./tests/...
```

---

## 🤝 Contributors

| Role           | Developer                               |
| -------------- | --------------------------------------- |
| **Backend**    | Luji                                    |
| **Mobile App** | [@kub1ga](https://github.com/kub1ga)    |
| **Assistance** | [Claude](https://claude.ai) (Anthropic) |
