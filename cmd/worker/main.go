package main

import (
	"log"
	"os"
	"yt-podcaster/internal/db"
	"yt-podcaster/internal/worker"
	"yt-podcaster/pkg/tasks"

	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
)

// CommitSHA is set at build time via ldflags
var CommitSHA = "unknown"

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}

	db.InitDB()

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "127.0.0.1:6379"
	}

	client := asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
	defer client.Close()

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Concurrency: 10,
		},
	)

	mux := asynq.NewServeMux()
	taskHandler := worker.NewTaskHandler(client)

	mux.HandleFunc(tasks.TypeCheckChannel, taskHandler.HandleCheckChannelTask)
	mux.HandleFunc(tasks.TypeProcessVideo, taskHandler.HandleProcessVideoTask)
	mux.HandleFunc(tasks.TypeCheckAllSubscriptions, taskHandler.HandleCheckAllSubscriptionsTask)

	log.Printf("Worker starting (commit: %s)", CommitSHA)
	if err := srv.Run(mux); err != nil {
		log.Fatalf("could not run server: %v", err)
	}
}
