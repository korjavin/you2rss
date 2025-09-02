package test

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
	"yt-podcaster/internal/db"
)

// MockTaskEnqueuer is a mock implementation of tasks.TaskEnqueuer for testing.
type MockTaskEnqueuer struct {
	EnqueuedTasks []*asynq.Task
}

func (m *MockTaskEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	m.EnqueuedTasks = append(m.EnqueuedTasks, task)
	return &asynq.TaskInfo{ID: "test-task-id", Queue: "default"}, nil
}

func NewMockDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock) {
	mockDb, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	sqlxDB := sqlx.NewDb(mockDb, "sqlmock")

	originalDB := db.DB
	db.DB = sqlxDB
	t.Cleanup(func() {
		db.DB = originalDB
		mockDb.Close()
	})

	return sqlxDB, mock
}
