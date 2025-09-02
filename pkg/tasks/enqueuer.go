package tasks

import "github.com/hibiken/asynq"

type TaskEnqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}
