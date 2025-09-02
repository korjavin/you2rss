# Stage 1: Build the Go applications
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source code
COPY . .

# Build the applications
RUN CGO_ENABLED=0 GOOS=linux go build -o ./bin/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o ./bin/worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -o ./bin/scheduler ./cmd/scheduler


# Stage 2: Create the final image
FROM alpine:latest

WORKDIR /app

# Copy the built binaries from the builder stage
COPY --from=builder /app/bin/server .
COPY --from=builder /app/bin/worker .
COPY --from=builder /app/bin/scheduler .

# Copy the templates and migrations
COPY web/templates ./web/templates
COPY migrations ./migrations

# Expose the port for the server
EXPOSE 8080

# The command will be provided in the docker-compose file
CMD ["./server"]
