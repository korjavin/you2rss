package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"yt-podcaster/internal/db"
	"yt-podcaster/internal/models"
	"yt-podcaster/pkg/tasks"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
)

// mockTaskEnqueuer is a mock implementation of tasks.TaskEnqueuer for testing.
type mockTaskEnqueuer struct {
	enqueuedTasks []*asynq.Task
}

func (m *mockTaskEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	m.enqueuedTasks = append(m.enqueuedTasks, task)
	return &asynq.TaskInfo{ID: "test-task-id", Queue: "default"}, nil
}

func TestHandleCheckChannelTask(t *testing.T) {
	// 1. Setup mock database
	mockDb, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mockDb.Close()
	sqlxDB := sqlx.NewDb(mockDb, "sqlmock")
	originalDB := db.DB
	db.DB = sqlxDB
	defer func() { db.DB = originalDB }()

	// 2. Setup mock execCommand and execCommandContext
	originalExecCommand := execCommand
	originalExecCommandContext := execCommandContext
	defer func() {
		execCommand = originalExecCommand
		execCommandContext = originalExecCommandContext
	}()
	mockCommandFunc := func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1", "YT_DLP_ARGS=" + strings.Join(arg, " ")}
		return cmd
	}
	execCommand = mockCommandFunc
	execCommandContext = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return mockCommandFunc(name, arg...)
	}

	// 3. Setup mock task enqueuer
	mockEnqueuer := &mockTaskEnqueuer{}

	// 4. Setup TaskHandler with mocks
	handler := NewTaskHandler(mockEnqueuer)

	// 5. Create task payload
	taskPayload := tasks.CheckChannelTaskPayload{SubscriptionID: 1}
	task := asynq.NewTask(tasks.TypeCheckChannel, mustMarshal(t, taskPayload))

	// 6. Define mock expectations
	sub := models.Subscription{ID: 1, UserID: 1, YoutubeChannelID: "test-channel", YoutubeChannelTitle: "Test Channel", CreatedAt: time.Now()}
	subRows := sqlmock.NewRows([]string{"id", "user_id", "youtube_channel_id", "youtube_channel_title", "created_at"}).AddRow(sub.ID, sub.UserID, sub.YoutubeChannelID, sub.YoutubeChannelTitle, sub.CreatedAt)
	mock.ExpectQuery(`SELECT \* FROM subscriptions WHERE id = \$1`).WithArgs(1).WillReturnRows(subRows)

	// Mock db call for IsNewChannel
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM episodes WHERE subscription_id = \$1`).WithArgs(1).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// Mock db call for checking if video exists and creating a new episode
	mock.ExpectQuery(`SELECT \* FROM episodes WHERE youtube_video_id = \$1`).WithArgs("video1").WillReturnError(sql.ErrNoRows)
	newEpisode := models.Episode{ID: 2, SubscriptionID: 1, YoutubeVideoID: "video1"}
	epRows := sqlmock.NewRows([]string{"id", "subscription_id", "youtube_video_id"}).AddRow(newEpisode.ID, newEpisode.SubscriptionID, newEpisode.YoutubeVideoID)
	mock.ExpectQuery(`INSERT INTO episodes`).WithArgs(1, "video1").WillReturnRows(epRows)

	mock.ExpectQuery(`SELECT \* FROM episodes WHERE youtube_video_id = \$1`).WithArgs("video2").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1)) // video2 already exists

	// 7. Call the handler
	err = handler.HandleCheckChannelTask(context.Background(), task)

	// 8. Assertions
	assert.NoError(t, err)
	assert.Len(t, mockEnqueuer.enqueuedTasks, 1)
	assert.Equal(t, tasks.TypeProcessVideo, mockEnqueuer.enqueuedTasks[0].Type())

	var enqueuedPayload tasks.ProcessVideoTaskPayload
	err = json.Unmarshal(mockEnqueuer.enqueuedTasks[0].Payload(), &enqueuedPayload)
	assert.NoError(t, err)
	assert.Equal(t, "video1", enqueuedPayload.YoutubeVideoID)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestHandleProcessVideoTask(t *testing.T) {
	// 1. Setup mock database
	mockDb, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mockDb.Close()
	sqlxDB := sqlx.NewDb(mockDb, "sqlmock")
	originalDB := db.DB
	db.DB = sqlxDB
	defer func() { db.DB = originalDB }()

	// 2. Setup mock execCommand and mock audio file
	originalExecCommand := execCommand
	originalExecCommandContext := execCommandContext
	defer func() {
		execCommand = originalExecCommand
		execCommandContext = originalExecCommandContext
	}()
	mockCommandFunc := func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1", "YT_DLP_ARGS=" + strings.Join(arg, " ")}
		return cmd
	}
	execCommand = mockCommandFunc
	execCommandContext = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return mockCommandFunc(name, arg...)
	}

	// Create a dummy audio file for os.Stat to work on
	err = os.MkdirAll("audio", 0755)
	assert.NoError(t, err)
	dummyFile, err := os.Create("audio/test-uuid.m4a")
	assert.NoError(t, err)
	defer os.Remove(dummyFile.Name())
	_, err = dummyFile.WriteString("dummy audio data")
	assert.NoError(t, err)
	dummyFile.Close()

	// 3. Setup TaskHandler
	handler := NewTaskHandler(nil) // No enqueuing in this handler

	// 4. Create task payload
	taskPayload := tasks.ProcessVideoTaskPayload{YoutubeVideoID: "video1", SubscriptionID: 1}
	task := asynq.NewTask(tasks.TypeProcessVideo, mustMarshal(t, taskPayload))

	// 5. Define mock expectations
	episode := models.Episode{ID: 1, SubscriptionID: 1, YoutubeVideoID: "video1", AudioUUID: "test-uuid"}
	epRows := sqlmock.NewRows([]string{"id", "subscription_id", "youtube_video_id", "audio_uuid"}).AddRow(episode.ID, episode.SubscriptionID, episode.YoutubeVideoID, episode.AudioUUID)
	mock.ExpectQuery(`SELECT \* FROM episodes WHERE youtube_video_id = \$1`).WithArgs("video1").WillReturnRows(epRows)

	mock.ExpectExec(`UPDATE episodes SET status = \$1 WHERE id = \$2`).WithArgs(db.StatusProcessing, episode.ID).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE episodes SET status = 'COMPLETED', title = \$1, description = \$2, audio_path = \$3, audio_size_bytes = \$4, duration_seconds = \$5, published_at = \$6 WHERE id = \$7`).WithArgs("Test Title", "Test Description", "audio/test-uuid.m4a", int64(16), 123, sqlmock.AnyArg(), episode.ID).WillReturnResult(sqlmock.NewResult(1, 1))

	// 6. Call the handler
	err = handler.HandleProcessVideoTask(context.Background(), task)

	// 7. Assertions
	assert.NoError(t, err)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

// TestHelperProcess isn't a real test. It's used as a helper for tests that
// need to mock exec.Command.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := strings.Split(os.Getenv("YT_DLP_ARGS"), " ")

	if contains(args, "--flat-playlist") {
		today := time.Now().Format("20060102")
		fmt.Println(fmt.Sprintf(`{"id": "video1", "title": "Video 1", "upload_date": "%s"}`, today))
		fmt.Println(`{"id": "video2", "title": "Video 2", "upload_date": "20200101"}`)
		os.Exit(0)
	}

	if contains(args, "-x") { // Extract audio command
		output := YtDlpOutput{
			ID:          "video1",
			Title:       "Test Title",
			Description: "Test Description",
			Duration:    123.45,
			Filename:    "audio/test-uuid.m4a",
			UploadDate:  "20230915",
		}
		jsonOutput, _ := json.Marshal(output)
		fmt.Println(string(jsonOutput))
		os.Exit(0)
	}

	os.Exit(1) // Should not be reached
}

func mustMarshal(t *testing.T, v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	return b
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}
