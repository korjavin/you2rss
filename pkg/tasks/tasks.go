package tasks

import (
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
)

const (
	TypeCheckChannel          = "channel:check"
	TypeProcessVideo          = "video:process"
	TypeCheckAllSubscriptions = "subscriptions:check"
	TypeRetryFailedEpisodes   = "episodes:retry"
)

type CheckChannelTaskPayload struct {
	SubscriptionID int
}

func NewCheckChannelTask(subscriptionID int) (*asynq.Task, error) {
	payload, err := json.Marshal(CheckChannelTaskPayload{SubscriptionID: subscriptionID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeCheckChannel, payload), nil
}

type ProcessVideoTaskPayload struct {
	YoutubeVideoID string
	SubscriptionID int
}

func NewProcessVideoTask(youtubeVideoID string, subscriptionID int) (*asynq.Task, error) {
	payload, err := json.Marshal(ProcessVideoTaskPayload{
		YoutubeVideoID: youtubeVideoID,
		SubscriptionID: subscriptionID,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeProcessVideo, payload), nil
}

// GetProcessVideoTaskOptions returns options for video processing tasks with higher retry limits
func GetProcessVideoTaskOptions() []asynq.Option {
	return []asynq.Option{
		asynq.MaxRetry(10),                  // Allow up to 10 retries instead of default 3
		asynq.Retention(7 * 24 * time.Hour), // Keep task info for 7 days
	}
}

func NewCheckAllSubscriptionsTask() (*asynq.Task, error) {
	return asynq.NewTask(TypeCheckAllSubscriptions, nil), nil
}

func NewRetryFailedEpisodesTask() (*asynq.Task, error) {
	return asynq.NewTask(TypeRetryFailedEpisodes, nil), nil
}
