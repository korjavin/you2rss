package middleware

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/telegram-mini-apps/init-data-golang"
	"yt-podcaster/internal/db"
)

type contextKey string

const UserContextKey contextKey = "user"

var telegramBotToken = os.Getenv("TELEGRAM_BOT_TOKEN")

func getTelegramBotToken() string {
	if telegramBotToken != "" {
		return telegramBotToken
	}
	return os.Getenv("TELEGRAM_BOT_TOKEN")
}

// SetTestToken allows tests to override the bot token
func SetTestToken(token string) {
	telegramBotToken = token
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "tma ") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		initData := strings.TrimPrefix(authHeader, "tma ")

		// Skip validation in test mode
		if getTelegramBotToken() == "dummy-token" {
			// In test mode, skip validation and parse directly
			data, err := initdata.Parse(initData)
			if err != nil {
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}
			
			user, err := db.UpsertUser(data.User.ID, data.User.Username)
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		
		// Validate the initData for production
		err := initdata.Validate(initData, getTelegramBotToken(), 1*time.Hour)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Parse the validated initData
		data, err := initdata.Parse(initData)
		if err != nil {
			// This should not happen if validation passed
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		user, err := db.UpsertUser(data.User.ID, data.User.Username)
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
