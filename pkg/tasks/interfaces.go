package tasks

import "github.com/hibiken/asynq"

// TaskEnqueuer defines the interface for enqueuing tasks.
// It's implemented by asynq.Client, and can be mocked for testing.
type TaskEnqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}
