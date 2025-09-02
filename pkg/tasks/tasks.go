package tasks

import (
	"encoding/json"
	"github.com/hibiken/asynq"
)

const (
	TypeCheckChannel          = "channel:check"
	TypeProcessVideo          = "video:process"
	TypeCheckAllSubscriptions = "subscriptions:check"
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

func NewCheckAllSubscriptionsTask() (*asynq.Task, error) {
	return asynq.NewTask(TypeCheckAllSubscriptions, nil), nil
}
