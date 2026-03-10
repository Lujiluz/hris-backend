# Stage 1: Build
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# Install swag CLI
RUN go install github.com/swaggo/swag/cmd/swag@latest
# ensure the bin path configured so swag direct call is supported
ENV PATH="/go/bin:${PATH}"

COPY . .

# Generate Swagger docs before building the app
RUN swag init -g cmd/api/main.go --parseDependency --parseInternal

# Build the Go app
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/api/main.go

# Stage 2: Run
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/main .
COPY --from=builder /app/.env . 
EXPOSE 8080
CMD ["./main"]
