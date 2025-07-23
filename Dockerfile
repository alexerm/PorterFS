# Build stage
FROM golang:1.21-alpine AS builder

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o porter ./cmd/porter

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1001 porter && \
    adduser -D -s /bin/sh -u 1001 -G porter porter

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/porter .

# Copy example config
COPY config.yaml.example config.yaml

# Create data directory and set ownership
RUN mkdir -p /data && chown -R porter:porter /app /data

# Switch to non-root user
USER porter

# Expose port
EXPOSE 9000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9000/ || exit 1

# Set default command
CMD ["./porter", "-config", "config.yaml"]