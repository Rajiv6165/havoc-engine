FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o havoc-engine ./cmd/havoc-engine

# Use a minimal alpine image for the final stage
FROM alpine:3.19

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/havoc-engine .
COPY --from=builder /app/config.yaml .

# Expose port 9090 for Prometheus metrics
EXPOSE 9090

# Command to run the executable
ENTRYPOINT ["./havoc-engine"]
