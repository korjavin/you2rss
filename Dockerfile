# Stage 1: Build the Go applications
FROM golang:1.22-alpine AS builder

# Add build argument for commit SHA
ARG COMMIT_SHA=unknown

WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source code
COPY . .

# Build the applications with commit SHA embedded
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.CommitSHA=${COMMIT_SHA}" -o ./bin/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.CommitSHA=${COMMIT_SHA}" -o ./bin/worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.CommitSHA=${COMMIT_SHA}" -o ./bin/scheduler ./cmd/scheduler


# Stage 2: Create the final image
FROM alpine:latest

WORKDIR /app

# Install required dependencies for yt-dlp and ffmpeg
RUN apk add --no-cache \
    python3 \
    py3-pip \
    ffmpeg \
    && pip3 install --no-cache-dir --break-system-packages yt-dlp

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
