package main

import (
	"log"
	"os"
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

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "127.0.0.1:6379"
	}

	scheduler := asynq.NewScheduler(
		asynq.RedisClientOpt{Addr: redisAddr},
		&asynq.SchedulerOpts{},
	)

	// Check all subscriptions every hour
	checkSubsTask, err := tasks.NewCheckAllSubscriptionsTask()
	if err != nil {
		log.Fatalf("could not create check subscriptions task: %v", err)
	}
	_, err = scheduler.Register("@every 1h", checkSubsTask)
	if err != nil {
		log.Fatalf("could not register check subscriptions task: %v", err)
	}

	// Retry failed episodes every 6 hours (less frequent to be gentle)
	retryFailedTask, err := tasks.NewRetryFailedEpisodesTask()
	if err != nil {
		log.Fatalf("could not create retry failed episodes task: %v", err)
	}
	_, err = scheduler.Register("@every 6h", retryFailedTask)
	if err != nil {
		log.Fatalf("could not register retry failed episodes task: %v", err)
	}

	log.Printf("Scheduler starting (commit: %s)", CommitSHA)
	if err := scheduler.Run(); err != nil {
		log.Fatalf("could not run scheduler: %v", err)
	}
}
