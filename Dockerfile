# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod ./
# go.sum will be created by go mod download if it doesn't exist
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o caskos ./cmd/caskos

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/caskos .

# Create directories for data and metadata
RUN mkdir -p /data /metadata

# Expose port
EXPOSE 8080

# Run the application
CMD ["./caskos", "-port", "8080", "-data-dir", "/data", "-metadata-dir", "/metadata", "-nodes", "3", "-replication", "2"]

