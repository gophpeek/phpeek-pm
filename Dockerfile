# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-w -s -X main.version=${VERSION}" \
    -o phpeek-pm \
    ./cmd/phpeek-pm

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata

# Create non-root user
RUN addgroup -g 1000 phpeek && \
    adduser -D -u 1000 -G phpeek phpeek

# Copy binary from builder
COPY --from=builder /build/phpeek-pm /usr/local/bin/phpeek-pm
RUN chmod +x /usr/local/bin/phpeek-pm

# Set up directories
RUN mkdir -p /etc/phpeek-pm && \
    chown -R phpeek:phpeek /etc/phpeek-pm

# Switch to non-root user
USER phpeek

# Expose ports
EXPOSE 9090 9180

# Health check
HEALTHCHECK --interval=10s --timeout=3s --start-period=30s --retries=3 \
    CMD wget -q -O- http://localhost:9180/api/v1/health || exit 1

# Run phpeek-pm
ENTRYPOINT ["/usr/local/bin/phpeek-pm"]
