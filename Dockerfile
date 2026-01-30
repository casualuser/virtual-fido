# Build stage
FROM golang:1.24-bullseye as builder

WORKDIR /app

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the binary for Linux. CGO is disabled for a static binary.
RUN CGO_ENABLED=0 GOOS=linux go build -o virtual-fido ./cmd/virtual-fido

# Final stage
FROM debian:stable-slim

# Install certificates for HTTPS/WebAuthn interactions
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/virtual-fido .

# Default command
ENTRYPOINT ["/app/virtual-fido"]
CMD ["--help"]
