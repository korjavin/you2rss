# Deployment Guide

This guide provides comprehensive instructions for deploying the YT-Podcaster service. We strongly recommend using Docker and Docker Compose for a consistent and reliable setup.

## Prerequisites

### For Docker Deployment (Recommended)
- Docker Engine
- Docker Compose
- `git` for cloning the repository

### For Manual/Development Setup
- Go (Version 1.21 or later)
- PostgreSQL (Version 12 or later)
- Redis (Version 4.0 or later)
- `yt-dlp` (Latest version recommended)
- `ffmpeg` and `ffprobe` (Required by `yt-dlp` for audio processing)
- `golang-migrate/migrate` CLI tool (for database migrations)

## 1. Cloning the Repository

First, clone the project repository to your server or local machine:

```bash
git clone https://github.com/korjavin/you2rss.git
cd you2rss
```
_Note: The repository name is `you2rss`, but the service is named YT-Podcaster._

## 2. Configuration

The service is configured using environment variables. All necessary variables are listed in `.env.example`.

1.  **Create a `.env` file**:
    ```bash
    cp .env.example .env
    ```

2.  **Edit the `.env` file** and provide values for at least the following required variables:

    -   `TELEGRAM_BOT_TOKEN`: The secret token for your Telegram Bot, obtained from BotFather.
    -   `DATABASE_URL`: The connection string for your PostgreSQL database (e.g., `postgres://user:password@localhost:5432/yt_podcaster?sslmode=disable`).
    -   `REDIS_ADDR`: The address for the Redis server (e.g., `localhost:6379`).
    -   `BASE_URL`: The public-facing base URL of the service, including the protocol (e.g., `https://your-service.com`). This is critical for generating correct URLs in the RSS feed.
    -   `AUDIO_STORAGE_PATH`: The absolute local filesystem path where extracted audio files will be stored (e.g., `/var/data/audio`). The Docker setup automatically maps this to a volume.

## 3. Database Migration

This project uses `golang-migrate/migrate` to manage database schema changes.

### Installing the Migration Tool

If you don't have it installed, you can install it with Go:
```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```
Make sure your `$GOPATH/bin` directory is in your system's `PATH`.

### Running Migrations

Before starting the service for the first time, you must run the database migrations.

**Important**: The `DATABASE_URL` in this command must be accessible from where you are running it. If you are using the recommended Docker setup, use the `localhost` address from the `.env` file, but ensure the PostgreSQL container is running first.

```bash
migrate -database "$DATABASE_URL" -path migrations up
```
*Note: You might need to wait a few moments after starting the Docker containers for the database to be ready to accept connections.*

## 4. Running the Service

### With Docker Compose (Recommended)

The `docker-compose.yml` file is configured for a standard production-like setup.

1.  **Build and Start all services**:
    ```bash
    docker-compose up --build -d
    ```
    The `-d` flag runs the containers in detached mode.

2.  **Run Migrations (if you haven't already)**:
    With the containers running, execute the migration command from the previous step.

3.  **Viewing Logs**:
    To view the logs from all running services:
    ```bash
    docker-compose logs -f
    ```
    To view logs for a specific service (e.g., `server`):
    ```bash
    docker-compose logs -f server
    ```

4.  **Stopping the services**:
    ```bash
    docker-compose down
    ```

### Manual Development Setup

For local development without Docker, you need to run three separate processes concurrently. Ensure all prerequisites are installed and the `.env` file is configured correctly.

Run each command in a separate terminal:

```bash
# Terminal 1: Web Server
go run ./cmd/server/main.go

# Terminal 2: Background Worker
go run ./cmd/worker/main.go

# Terminal 3: Job Scheduler
go run ./cmd/scheduler/main.go
```

## 5. Production Deployment (`docker-compose.prod.yml`)

The repository includes a `docker-compose.prod.yml` file optimized for production. It is configured to:
- Use pre-built images from a container registry (by default, GHCR).
- Restart services automatically unless explicitly stopped.

To use it:
```bash
docker-compose -f docker-compose.prod.yml up -d
```

You will need to adjust the `image` tags in `docker-compose.prod.yml` if you are using your own private registry. The provided CI/CD workflow in `.github/workflows/deploy.yml` can be adapted to push to your registry.

## 6. Verifying the Setup

1.  **Access the Web Interface**: The server runs on port 8080 by default. Open your browser to `http://localhost:8080` or `http://<your_server_ip>:8080`.
2.  **Telegram Mini App**: Configure your Telegram bot with the `BASE_URL` to enable the Mini App.
3.  **Check Logs**: Use `docker-compose logs -f` to monitor for any errors during startup or operation.
