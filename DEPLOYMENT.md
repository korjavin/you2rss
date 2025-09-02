# Deployment Guide

This guide provides instructions on how to deploy the YT-Podcaster application using Docker Compose.

## Prerequisites

- Docker
- Docker Compose

## Configuration

The application is configured using environment variables. You will need to create a `.env` file in the root of the project with the following content:

```
TELEGRAM_BOT_TOKEN=your_telegram_bot_token
```

Replace `your_telegram_bot_token` with your actual Telegram bot token.

## Running the Application

To run the application, use the following command:

```
docker-compose up --build
```

This will build the Docker images and start all the services. The web server will be available on port 8080.

## Running Migrations

The database migrations are not run automatically. To run the migrations, you will need to have `golang-migrate/migrate` installed. You can install it with the following command:

```
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

Once installed, you can run the migrations with the following command:

```
migrate -path migrations -database "postgres://user:password@localhost:5432/yt_podcaster?sslmode=disable" up
```

Make sure the PostgreSQL container is running before executing this command.
