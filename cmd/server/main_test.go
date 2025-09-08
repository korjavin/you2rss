package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"yt-podcaster/internal/middleware"
	"yt-podcaster/internal/models"
	"yt-podcaster/internal/test"
)

var validInitData = "query_id=AAHdF614AAAAAN0Xrhom_pA&user=%7B%22id%22%3A123%2C%22first_name%22%3A%22Test%22%2C%22last_name%22%3A%22User%22%2C%22username%22%3A%22testuser%22%2C%22language_code%22%3A%22en%22%7D&auth_date=1672531200&hash=e51bca5855f98822011a62a939aa68e9be25b5502195f128038d8c364273872f"

func TestGetRootHandler(t *testing.T) {
	app := NewApp(nil) // Using nil as we don't enqueue in this handler
	_, mock := test.NewMockDB(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	// The root handler doesn't require authentication - it just serves the HTML page
	// No database queries expected
	
	app.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "<h1>YT-Podcaster</h1>")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPostSubscriptionsHandler(t *testing.T) {
	t.Skip("Skipping POST test that requires yt-dlp mocking for CI compatibility")
	middleware.SetTestToken("dummy-token")
	defer middleware.SetTestToken("")
	
	mockEnqueuer := &test.MockTaskEnqueuer{}
	app := NewApp(mockEnqueuer)
	_, mock := test.NewMockDB(t)

	form := url.Values{}
	form.Add("url", "https://www.youtube.com/channel/UC-lHJZR3Gqxm24_Vd_AJ5Yw")
	req := httptest.NewRequest(http.MethodPost, "/subscriptions", strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "tma "+validInitData)

	rr := httptest.NewRecorder()

	user := &models.User{ID: 1, TelegramID: 123, TelegramUsername: "testuser", CreatedAt: time.Now()}
	userRows := sqlmock.NewRows([]string{"id", "telegram_id", "telegram_username", "rss_uuid", "created_at", "updated_at"}).
		AddRow(user.ID, user.TelegramID, user.TelegramUsername, "some-uuid", user.CreatedAt, user.CreatedAt)
	mock.ExpectQuery(`INSERT INTO users`).WithArgs(int64(123), "testuser").WillReturnRows(userRows)
	
	// Mock the subscription count check
	countRows := sqlmock.NewRows([]string{"count"}).AddRow(0)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM subscriptions WHERE user_id = \$1`).WithArgs(user.ID).WillReturnRows(countRows)

	newSubscription := models.Subscription{ID: 1, UserID: 1, YoutubeChannelID: "UC-lHJZR3Gqxm24_Vd_AJ5Yw", YoutubeChannelTitle: "UC-lHJZR3Gqxm24_Vd_AJ5Yw", CreatedAt: time.Now()}
	rows := sqlmock.NewRows([]string{"id", "user_id", "youtube_channel_id", "youtube_channel_title", "created_at"}).
		AddRow(newSubscription.ID, newSubscription.UserID, newSubscription.YoutubeChannelID, newSubscription.YoutubeChannelTitle, newSubscription.CreatedAt)

	mock.ExpectQuery(`INSERT INTO subscriptions`).WithArgs(user.ID, "UC-lHJZR3Gqxm24_Vd_AJ5Yw", "UC-lHJZR3Gqxm24_Vd_AJ5Yw").WillReturnRows(rows)

	subscriptionsRows := sqlmock.NewRows([]string{"id", "user_id", "youtube_channel_id", "youtube_channel_title", "created_at"}).
		AddRow(newSubscription.ID, newSubscription.UserID, newSubscription.YoutubeChannelID, newSubscription.YoutubeChannelTitle, newSubscription.CreatedAt)
	mock.ExpectQuery(`SELECT (.+) FROM subscriptions`).WithArgs(user.ID).WillReturnRows(subscriptionsRows)

	app.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "UC-lHJZR3Gqxm24_Vd_AJ5Yw")
	assert.NotEmpty(t, mockEnqueuer.EnqueuedTasks)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteSubscriptionHandler(t *testing.T) {
	middleware.SetTestToken("dummy-token")
	defer middleware.SetTestToken("")
	
	app := NewApp(nil)
	_, mock := test.NewMockDB(t)

	req := httptest.NewRequest(http.MethodDelete, "/subscriptions/1", nil)
	req.Header.Set("Authorization", "tma "+validInitData)
	rr := httptest.NewRecorder()

	user := &models.User{ID: 1, TelegramID: 123, TelegramUsername: "testuser", CreatedAt: time.Now()}
	userRows := sqlmock.NewRows([]string{"id", "telegram_username", "rss_uuid", "created_at", "updated_at"}).
		AddRow(user.ID, user.TelegramUsername, "some-uuid", user.CreatedAt, user.CreatedAt)
	mock.ExpectQuery(`INSERT INTO users \(id, telegram_username\) VALUES \(\$1, \$2\) ON CONFLICT \(id\) DO UPDATE SET telegram_username = EXCLUDED\.telegram_username, updated_at = NOW\(\) RETURNING`).WithArgs(int64(123), "testuser").WillReturnRows(userRows)

	mock.ExpectExec("DELETE FROM subscriptions WHERE id = \\$1 AND user_id = \\$2").WithArgs(1, user.ID).WillReturnResult(sqlmock.NewResult(1, 1))

	app.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetRSSFeedHandler(t *testing.T) {
	app := NewApp(nil)
	_, mock := test.NewMockDB(t)
	user := &models.User{ID: 1, RSSUUID: "test-uuid", TelegramUsername: "testuser", CreatedAt: time.Now()}

	req := httptest.NewRequest(http.MethodGet, "/rss/test-uuid", nil)
	rr := httptest.NewRecorder()

	userRows := sqlmock.NewRows([]string{"id", "telegram_id", "telegram_username", "rss_uuid", "created_at", "updated_at"}).
		AddRow(user.ID, 123, user.TelegramUsername, user.RSSUUID, user.CreatedAt, user.CreatedAt)
	mock.ExpectQuery("SELECT (.+) FROM users WHERE rss_uuid = \\$1").WithArgs("test-uuid").WillReturnRows(userRows)

	title := "Test Episode"
	desc := "A test episode."
	audioFile := "audio/audio-uuid.m4a"
	audioSize := int64(12345)
	publishedAt := time.Now()

	episodeRows := sqlmock.NewRows([]string{"id", "title", "description", "audio_path", "audio_size_bytes", "published_at"}).
		AddRow(1, title, desc, audioFile, audioSize, publishedAt)
	mock.ExpectQuery("SELECT (.+) FROM episodes e").WithArgs(user.ID).WillReturnRows(episodeRows)

	app.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/rss+xml", rr.Header().Get("Content-Type"))
	assert.Contains(t, rr.Body.String(), "<title>Test Episode</title>")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestServeAudioHandler(t *testing.T) {
	originalPath := os.Getenv("AUDIO_STORAGE_PATH")
	os.Setenv("AUDIO_STORAGE_PATH", "audio_test")
	defer os.Setenv("AUDIO_STORAGE_PATH", originalPath)

	app := NewApp(nil)
	err := os.MkdirAll("audio_test", 0755)
	assert.NoError(t, err)
	dummyFile, err := os.Create("audio_test/test-audio.m4a")
	assert.NoError(t, err)
	defer os.RemoveAll("audio_test")
	_, err = dummyFile.WriteString("dummy audio data")
	assert.NoError(t, err)
	dummyFile.Close()

	req := httptest.NewRequest(http.MethodGet, "/audio/test-audio.m4a", nil)
	rr := httptest.NewRecorder()

	app.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	
	// MIME type can vary between systems for .m4a files
	contentType := rr.Header().Get("Content-Type")
	assert.Contains(t, []string{"audio/mp4", "audio/mp4a-latm"}, contentType)
	
	assert.Equal(t, "dummy audio data", rr.Body.String())
}
