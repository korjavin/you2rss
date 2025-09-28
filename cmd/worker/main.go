package main

import (
	"log"
	"os"
	"time"
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
			Concurrency: 1, // Process one task at a time to be gentle with YouTube
			Queues: map[string]int{
				"high":    2,
				"default": 1,
			},
			// Custom retry delay function for exponential backoff
			RetryDelayFunc: func(n int, err error, task *asynq.Task) time.Duration {
				// Calculate exponential backoff delay
				delay := time.Duration(5*60*1000) * time.Millisecond        // 5 minutes base
				maxDelay := time.Duration(24*60*60*1000) * time.Millisecond // 24 hours max

				// Exponential backoff: 5min, 10min, 20min, 40min, 80min, etc.
				for i := 0; i < n; i++ {
					delay *= 2
					if delay > maxDelay {
						delay = maxDelay
						break
					}
				}

				log.Printf("Task %s failed %d times, retrying in %v", task.Type(), n+1, delay)
				return delay
			},
		},
	)

	mux := asynq.NewServeMux()
	taskHandler := worker.NewTaskHandler(client)

	mux.HandleFunc(tasks.TypeCheckChannel, taskHandler.HandleCheckChannelTask)
	mux.HandleFunc(tasks.TypeProcessVideo, taskHandler.HandleProcessVideoTask)
	mux.HandleFunc(tasks.TypeCheckAllSubscriptions, taskHandler.HandleCheckAllSubscriptionsTask)
	mux.HandleFunc(tasks.TypeRetryFailedEpisodes, taskHandler.HandleRetryFailedEpisodesTask)

	log.Printf("Worker starting (commit: %s)", CommitSHA)
	if err := srv.Run(mux); err != nil {
		log.Fatalf("could not run server: %v", err)
	}
}
