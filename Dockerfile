# Stage 1: Build the Go application
FROM golang:1.25.6-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
# Kita build binary dari cmd/api/main.go
RUN go build -o main ./cmd/api/main.go

# Stage 2: Run the application
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/main .

# Copy the views and public assets
COPY --from=builder /app/views ./views
COPY --from=builder /app/public ./public
COPY --from=builder /app/.env ./.env

# Expose the port the app runs on
EXPOSE 3000

# Command to run the application
CMD ["./main"]
