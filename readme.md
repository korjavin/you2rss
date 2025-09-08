# YT-Podcaster Service

## Overview

YT-Podcaster is a personal, on-demand podcasting service designed to convert public YouTube channel content into private, audio-only RSS feeds. The service provides a seamless user experience through a Telegram Mini App, allowing users to securely authenticate, manage a list of YouTube channel subscriptions, and receive a unique RSS feed URL compatible with any standard podcast client. The backend is engineered to automatically monitor subscribed channels for new videos, download them, extract the audio, and update the user's personal feed, effectively transforming any YouTube channel into a listenable podcast.

## 🏠 Self-Hosting Encouraged

**We strongly encourage self-hosting this project for maximum privacy and security.** When you host YT-Podcaster yourself, you maintain complete control over your data, subscriptions, and audio content. Your YouTube viewing preferences and podcast consumption habits remain entirely private, stored only on infrastructure you control.

Self-hosting benefits:
- **Complete Privacy**: Your subscription data never leaves your servers
- **No Rate Limits**: Configure resource limits according to your needs
- **Full Control**: Customize the service to your specific requirements
- **Data Ownership**: Your audio files and metadata remain under your control

## 🤝 Contributing

We welcome contributions from the community! Whether you're fixing bugs, adding features, improving documentation, or enhancing security, your help makes this project better for everyone.

**How to contribute:**
- Fork the repository
- Create a feature branch (`git checkout -b feature/amazing-feature`)
- Make your changes and add tests
- Ensure all tests pass (`go test ./...`)
- Submit a Pull Request

**Areas where we especially welcome contributions:**
- Additional podcast client compatibility
- Performance optimizations
- Security enhancements
- Docker and deployment improvements
- Documentation and guides

## Core Features

- **Secure User Authentication**: Employs the Telegram Mini App platform's initData mechanism for a secure, passwordless authentication experience.

- **YouTube Channel Subscriptions**: Provides a simple interface for users to add, view, and remove YouTube channels from their personal subscription list.

- **Automated Content Fetching**: Utilizes a robust background job system to regularly poll subscribed channels for new video content, ensuring feeds are kept up-to-date.

- **Audio Extraction & Transcoding**: Automatically downloads new video content using yt-dlp, extracts the audio stream, and transcodes it into a podcast-friendly format (M4A).

- **Personalized RSS Feed Generation**: Generates a unique, secure, and podcast-client-compatible RSS 2.0 feed for each user, complete with necessary iTunes-specific tags for a rich client experience.

- **Secure Audio Hosting**: Serves the extracted audio files through obfuscated, non-enumerable UUID-based URLs to protect user privacy and prevent unauthorized access.

## Technology Stack

The technology stack is carefully selected to leverage specialized, best-in-class libraries and tools prevalent in the Go ecosystem. This compositional approach, favoring dedicated tools over a monolithic framework, provides high performance, flexibility, and maintainability.

- **Backend**: Go (Golang) - Chosen for its exceptional performance in network services, its simple yet powerful concurrency model based on goroutines, and its comprehensive standard library, which is ideal for building efficient and scalable web services.

- **Frontend**: HTML with htmx - Selected to create a dynamic, server-rendered user interface without the overhead and complexity of a full-fledged client-side JavaScript framework. This approach aligns perfectly with a Go backend, simplifying the stack and development workflow.

- **Authentication**: Telegram Mini App Platform - Leveraged for its built-in, secure authentication flow. The initData mechanism provides a reliable way to verify user identity without managing passwords or sessions manually.

- **Background Jobs**: Asynq & Redis - A powerful, Redis-backed task queue library for Go. It is essential for offloading long-running, resource-intensive tasks such as video downloading and transcoding. Asynq provides critical features like job scheduling, automatic retries on failure, and a monitoring UI, ensuring system resilience and reliability.

- **Database**: PostgreSQL - A robust, open-source relational database used for the persistent storage of user accounts, subscriptions, and episode metadata. Its support for UUIDs and transactional integrity makes it an ideal choice. SQLite can be used as a simpler alternative for development.

- **Audio Extraction**: yt-dlp - A feature-rich and actively maintained command-line utility for downloading video and audio from YouTube and thousands of other sites. Its flexibility and power are unmatched for this core task.

- **RSS Generation**: eduncan911/podcast - A specialized Go library for creating fully compliant RSS 2.0 podcast feeds with iTunes extensions. Its focused API is more suitable for this project's needs than a generic XML or feed generation library.

- **Telegram Auth Helper**: init-data-golang - The recommended Go library for securely parsing and validating Telegram initData, providing a robust and tested implementation of the validation algorithm.


## Configuration

The service is configured using environment variables. Create a `.env` file in the project root by copying the `.env.example` file and populating it with your specific values.

### Required Configuration

- **TELEGRAM_BOT_TOKEN**: The secret token for your Telegram Bot, obtained from BotFather.
- **DATABASE_URL**: The connection string for your PostgreSQL database (e.g., `postgres://user:password@localhost:5432/yt_podcaster?sslmode=disable`).
- **REDIS_ADDR**: The address for the Redis server (e.g., `localhost:6379`).
- **BASE_URL**: The public-facing base URL of the service (e.g., `https://your-service.com`). This is critical for generating correct URLs in the RSS feed.
- **AUDIO_STORAGE_PATH**: The absolute local filesystem path where extracted audio files will be stored (e.g., `/var/data/audio`).

### Optional Configuration

- **PORT**: Server port (default: `8080`)
- **RATE_LIMIT_PER_MINUTE**: API requests per minute per user (default: `100`)
- **RATE_LIMIT_BURST**: Rate limiting burst size (default: `5`)
- **MAX_SUBSCRIPTIONS_PER_USER**: Maximum subscriptions per user (default: `100`)
- **PROCESS_VIDEO_TIMEOUT_MINUTES**: Video processing timeout (default: `15`)
- **CHANNEL_INFO_TIMEOUT_SECONDS**: Channel info fetching timeout (default: `15`)

## Installation and Deployment

For detailed instructions on how to set up, configure, and deploy the service, please see the comprehensive **[Deployment Guide](DEPLOYMENT.md)**.

## Running the Service

### With Docker (Development)

For development with local database and redis:

```bash
# Start all services with local database and redis
docker-compose -f docker-compose.dev.yml up --build
```

### With Docker (Production)

For production deployment using pre-built images from GHCR:

```bash
# Set required environment variables and deploy
export TELEGRAM_BOT_TOKEN="your_telegram_bot_token"
export BASE_URL="https://your-domain.com"
export DOMAIN="your-domain.com"

# Deploy with included PostgreSQL and Redis
docker-compose up -d
```

The production setup includes:
- PostgreSQL database (internal)
- Redis cache (internal) 
- All application services
- Traefik reverse proxy labels
- Automatic database setup

Only 3 environment variables are required - everything else has sensible defaults.

### Manual Development Mode

This service consists of multiple long-running processes that must be run concurrently for the system to be fully operational. Run each command in a separate terminal:

```bash
# Terminal 1: Web Server (handles UI and API)
go run ./cmd/server/main.go

# Terminal 2: Worker (processes video downloads)
go run ./cmd/worker/main.go

# Terminal 3: Scheduler (periodic channel checks)
go run ./cmd/scheduler/main.go
```

### Testing

Run the test suite:

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with race detection
go test -race ./...
```

## Production Deployment

For production deployment, see the **[Deployment Guide](DEPLOYMENT.md)** for detailed instructions. The project also includes an automated CI/CD pipeline with GitHub Actions.

## Usage

1. **Access the Web Interface**: Open your browser and navigate to `http://localhost:8080` (or your configured BASE_URL)

2. **Telegram Mini App**: The service is designed to be used as a Telegram Mini App. Configure your Telegram bot and add the Mini App URL.

3. **Add Subscriptions**: Use the web interface to add YouTube channel URLs to your subscription list.

4. **Get Your RSS Feed**: Your personal RSS feed URL will be: `{BASE_URL}/rss/{your_rss_uuid}`

5. **Add to Podcast Client**: Copy your RSS feed URL and add it to any podcast client (Apple Podcasts, Spotify, Pocket Casts, etc.)

## Architecture

The service consists of three main components:

- **Server** (`cmd/server`): Serves the htmx frontend and handles API requests
- **Worker** (`cmd/worker`): Processes background jobs for video downloading and audio extraction  
- **Scheduler** (`cmd/scheduler`): Periodically checks subscribed channels for new content

For detailed architecture information, see `architecture.md`.

## Security Features

- **Telegram Authentication**: Secure, passwordless authentication via Telegram Mini App initData
- **UUID-based URLs**: Non-enumerable URLs for RSS feeds and audio files
- **Rate Limiting**: Configurable per-user API rate limiting
- **Input Validation**: Rigorous validation of YouTube URLs and user input
- **Command Injection Prevention**: Safe execution of external tools (yt-dlp, ffmpeg)

## License

This project is open source and distributed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Support

- **Documentation**: See `architecture.md` for technical details and `DEPLOYMENT.md` for deployment guide
- **Issues**: Report bugs or request features via GitHub Issues
- **Contributions**: See the Contributing section above for how to submit improvements

---

**Remember**: Self-hosting ensures your privacy and gives you full control over your podcast data. We encourage you to deploy your own instance rather than relying on hosted services.
