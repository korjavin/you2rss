# Project Title: YT-Podcaster Service

## Overview

YT-Podcaster is a personal, on-demand podcasting service designed to convert public YouTube channel content into private, audio-only RSS feeds. The service provides a seamless user experience through a Telegram Mini App, allowing users to securely authenticate, manage a list of YouTube channel subscriptions, and receive a unique RSS feed URL compatible with any standard podcast client. The backend is engineered to automatically monitor subscribed channels for new videos, download them, extract the audio, and update the user's personal feed, effectively transforming any YouTube channel into a listenable podcast.

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

## Prerequisites

The following software must be installed and available in the system's PATH on both development and production environments:

- Go (Version 1.21 or later)
- Redis (Version 4.0 or later)
- yt-dlp (Latest version recommended)
- ffmpeg and ffprobe (Required dependencies for yt-dlp to perform audio extraction and format conversion).

## Configuration

The service is configured using environment variables. Create a .env file in the project root by copying the .env.example file and populating it with your specific values.

- **TELEGRAM_BOT_TOKEN**: The secret token for your Telegram Bot, obtained from BotFather.
- **DATABASE_URL**: The connection string for your PostgreSQL database (e.g., postgres://user:password@localhost:5432/yt_podcaster?sslmode=disable).
- **REDIS_ADDR**: The address for the Redis server (e.g., localhost:6379).
- **BASE_URL**: The public-facing base URL of the service (e.g., https://your-service.com). This is critical for generating correct URLs in the RSS feed.
- **AUDIO_STORAGE_PATH**: The absolute local filesystem path where extracted audio files will be stored (e.g., /var/data/audio).

## Installation & Setup

Clone the repository:

```bash
git clone https://github.com/your-username/yt-podcaster.git
```

Navigate to the project directory:

```bash
cd yt-podcaster
```

Install Go dependencies:

```bash
go mod tidy
```

Set up your environment configuration:

```bash
cp .env.example .env
# Edit .env with your configuration
```

Run database migrations to create the required tables. (Assuming a migration tool is configured).

## Running the Service

This service consists of multiple long-running processes that must be run concurrently for the system to be fully operational.

- **Start the Web Server**: This process handles all user-facing HTTP requests, including the UI and API.

  ```bash
  go run ./cmd/server
  ```

- **Start the Asynq Worker**: This process executes the background jobs, such as downloading and processing videos.

  ```bash
  go run ./cmd/worker
  ```

- **Start the Asynq Scheduler**: This process enqueues periodic jobs to check for new videos from subscribed channels.

  ```bash
  go run ./cmd/scheduler
  ```
