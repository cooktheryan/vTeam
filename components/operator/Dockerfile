# Build stage
FROM registry.access.redhat.com/ubi9/go-toolset:1.24 AS builder

USER 0
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application (with flags to avoid segfault)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o operator .

# Final stage
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/operator .

# Set executable permissions and make accessible to any user
RUN chmod +x ./operator && chmod 755 /app

# Command to run the executable
CMD ["./operator"]
