# Stage 1: Build the Go binary
FROM golang:alpine AS builder

WORKDIR /app

# Copy dependency definitions
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the Linux binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o main ./cmd/api/main.go

# Stage 2: Create the final run image
FROM nginx:alpine

# Install runtime tools (tzdata and ca-certificates)
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy build output from the builder stage
COPY --from=builder /app/main .

# Copy template and static assets
COPY views ./views
COPY public ./public
COPY .env ./.env

EXPOSE 3000

CMD ["./main"]
