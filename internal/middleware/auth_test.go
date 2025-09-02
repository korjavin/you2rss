package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"yt-podcaster/internal/models"
	"yt-podcaster/internal/test"
)

func TestAuthMiddleware(t *testing.T) {
	// This is a valid initData string for a user with ID 123, username "testuser"
	// The hash is pre-calculated with a dummy bot token "dummy-token"
	validInitData := "query_id=AAHdF614AAAAAN0Xrhom_pA&user=%7B%22id%22%3A123%2C%22first_name%22%3A%22Test%22%2C%22last_name%22%3A%22User%22%2C%22username%22%3A%22testuser%22%2C%22language_code%22%3A%22en%22%7D&auth_date=1672531200&hash=e51bca5855f98822011a62a939aa68e9be25b5502195f128038d8c364273872f"

	originalToken := telegramBotToken
	SetTestToken("dummy-token")
	defer func() { SetTestToken(originalToken) }()

	t.Run("valid auth data", func(t *testing.T) {
		_, mock := test.NewMockDB(t)
		user := models.User{ID: 1, TelegramID: 123, TelegramUsername: "testuser", CreatedAt: time.Now()}
		rows := sqlmock.NewRows([]string{"id", "telegram_id", "telegram_username", "rss_uuid", "created_at", "updated_at"}).
			AddRow(user.ID, user.TelegramID, user.TelegramUsername, "some-uuid", user.CreatedAt, user.CreatedAt)
		mock.ExpectQuery(`INSERT INTO users`).WithArgs(int64(123), "testuser").WillReturnRows(rows)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "tma "+validInitData)
		rr := httptest.NewRecorder()

		mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctxUser := r.Context().Value(UserContextKey)
			assert.NotNil(t, ctxUser)
			dbUser, ok := ctxUser.(*models.User)
			assert.True(t, ok)
			assert.Equal(t, user.ID, dbUser.ID)
			w.WriteHeader(http.StatusOK)
		})

		AuthMiddleware(mockHandler).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("no authorization header", func(t *testing.T) {
		test.NewMockDB(t) // To setup the db.DB variable
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		AuthMiddleware(nil).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("invalid authorization header format", func(t *testing.T) {
		test.NewMockDB(t)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer sometoken")
		rr := httptest.NewRecorder()
		AuthMiddleware(nil).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	//t.Run("invalid init data hash", func(t *testing.T) {
	//	test.NewMockDB(t)
	//	invalidInitData := "query_id=...&hash=invalidhash"
	//	req := httptest.NewRequest(http.MethodGet, "/", nil)
	//	req.Header.Set("Authorization", "tma "+invalidInitData)
	//	rr := httptest.NewRecorder()
	//	AuthMiddleware(nil).ServeHTTP(rr, req)
	//	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	//})
}
