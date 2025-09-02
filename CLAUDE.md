# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

YT-Podcaster is a distributed Go service that converts YouTube channels into personal podcast RSS feeds through a Telegram Mini App interface. The system uses background job processing with Redis/Asynq for video downloading and audio extraction.

## Architecture

The service consists of three main components:
- **Web Server** (`cmd/server`): Serves htmx frontend and API endpoints with Telegram authentication
- **Worker** (`cmd/worker`): Processes background jobs (video downloading, audio extraction)  
- **Scheduler** (`cmd/scheduler`): Manages periodic channel checking via cron jobs

Key directories:
- `internal/handlers/`: HTTP request handlers
- `internal/middleware/`: Authentication and other middleware
- `internal/worker/`: Background job handlers
- `internal/db/`: Database models and queries
- `internal/feed/`: RSS feed generation
- `pkg/tasks/`: Task definitions for Asynq
- `migrations/`: Database schema migrations
- `web/templates/`: HTML templates for htmx frontend

## Development Commands

### Build and Run Services
```bash
# Build all services
go build -o bin/server ./cmd/server
go build -o bin/worker ./cmd/worker  
go build -o bin/scheduler ./cmd/scheduler

# Run individual services (requires Redis and PostgreSQL)
go run ./cmd/server
go run ./cmd/worker
go run ./cmd/scheduler

# Run with Docker Compose (includes all dependencies)
docker-compose up --build
```

### Testing
```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run specific test package
go test ./internal/handlers
```

### Database Management
```bash
# Install migration tool
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Run migrations (development)
migrate -database "postgres://user:password@localhost:5432/yt_podcaster?sslmode=disable" -path migrations up

# Run migrations (Docker)
migrate -database "postgres://user:password@postgres:5432/yt_podcaster?sslmode=disable" -path migrations up
```

## Configuration

Environment variables (`.env` file):
- `TELEGRAM_BOT_TOKEN`: Bot token from BotFather
- `DATABASE_URL`: PostgreSQL connection string
- `REDIS_ADDR`: Redis server address (localhost:6379)
- `BASE_URL`: Public-facing URL for RSS feeds
- `AUDIO_STORAGE_PATH`: Local path for audio file storage

## Key Technical Details

### Authentication
Uses Telegram Mini App `initData` validation via `telegram-mini-apps/init-data-golang`. All protected routes require `Authorization: tma <initData>` header.

### Background Jobs
- Built on `hibiken/asynq` with Redis backend
- Task definitions in `pkg/tasks/payloads.go`
- Handlers in `internal/worker/handlers.go`
- Two main job types: `CheckChannelTask` (discovery) and `ProcessVideoTask` (download/extract)

### Audio Processing
Uses `yt-dlp` CLI tool for video downloading and `ffmpeg` for audio extraction to M4A format. Files stored with UUID-based names for security.

### Database Schema
Three main tables:
- `users`: Telegram user data with RSS UUID
- `subscriptions`: User-channel relationships  
- `episodes`: Video metadata and processing status

### Security Considerations
- UUIDs prevent URL enumeration for RSS feeds and audio files
- Command injection prevention via `os/exec.Command` with separate args
- Input validation on all YouTube URLs
- Rate limiting required on subscription endpoints

## API Endpoints

- `GET /`: Serves htmx Mini App frontend
- `POST /auth`: Validates Telegram initData  
- `GET /subscriptions`: Returns user subscriptions (htmx)
- `POST /subscriptions`: Adds new channel subscription (htmx)
- `DELETE /subscriptions/{id}`: Removes subscription (htmx)
- `GET /rss/{user_rss_uuid}`: Serves RSS feed XML
- `GET /audio/{audio_uuid}.m4a`: Serves audio files

## Dependencies

External tools required:
- `yt-dlp`: Video/audio downloading
- `ffmpeg`/`ffprobe`: Audio processing
- `redis-server`: Job queue backend
- `postgresql`: Primary database

Go modules use composition over frameworks, leveraging:
- `gorilla/mux`: HTTP routing
- `jmoiron/sqlx`: Database interface  
- `hibiken/asynq`: Job queue
- `eduncan911/podcast`: RSS generation
- `telegram-mini-apps/init-data-golang`: Auth validation