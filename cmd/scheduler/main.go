package main

import (
	"log"
	"os"
	"yt-podcaster/internal/db"
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

	scheduler := asynq.NewScheduler(
		asynq.RedisClientOpt{Addr: redisAddr},
		&asynq.SchedulerOpts{},
	)

	task, err := tasks.NewCheckAllSubscriptionsTask()
	if err != nil {
		log.Fatalf("could not create task: %v", err)
	}

	// Run every hour
	_, err = scheduler.Register("@every 1h", task)
	if err != nil {
		log.Fatalf("could not register task: %v", err)
	}

	log.Printf("Scheduler starting (commit: %s)", CommitSHA)
	if err := scheduler.Run(); err != nil {
		log.Fatalf("could not run scheduler: %v", err)
	}
}
