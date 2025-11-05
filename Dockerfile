# Multi-stage build for reproducible binaries
# The build environment is versioned to ensure consistent builds across all environments

# Stage 1: Builder
FROM golang:1.21-alpine AS builder

# Install git for go mod (if needed), ca-certificates, and file for verification
RUN apk add --no-cache git ca-certificates tzdata file

# Set working directory
WORKDIR /build

# Copy go.mod and go.sum first for better layer caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download
RUN go mod verify

# Copy source code
COPY . .

# Run tests to ensure everything works before building
RUN go test -v ./...

# Build static binary
# CGO_ENABLED=0 ensures static linking (no dynamic dependencies)
# -ldflags="-w -s" strips debug info for smaller binary
# -trimpath removes local path information for reproducibility
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -trimpath \
    -o /build/recal \
    ./cmd/recal

# Verify the binary is statically linked
RUN file /build/recal | grep "statically linked"

# Stage 2: Distroless runtime
FROM gcr.io/distroless/static-debian12:nonroot

# Copy CA certificates from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the static binary
# NOTE: config.yaml is NOT copied - it should be mounted at runtime
COPY --from=builder /build/recal /app/recal

# Set working directory
WORKDIR /app

# Expose port
EXPOSE 8080

# Run as non-root user (distroless nonroot user is UID 65532)
USER nonroot:nonroot

# Set config file location
ENV CONFIG_FILE=/app/config.yaml

# Run the application
ENTRYPOINT ["/app/recal"]
