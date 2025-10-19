package middleware

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"yt-podcaster/internal/db"
	"yt-podcaster/internal/models"

	initdata "github.com/telegram-mini-apps/init-data-golang"
)

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
		log.Printf("AuthMiddleware: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

		authHeader := r.Header.Get("Authorization")
		log.Printf("AuthMiddleware: Authorization header present: %v", authHeader != "")

		if authHeader == "" || !strings.HasPrefix(authHeader, "tma ") {
			log.Printf("AuthMiddleware: Missing or invalid auth header")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		initData := strings.TrimPrefix(authHeader, "tma ")
		log.Printf("AuthMiddleware: InitData length: %d, first 100 chars: %s", len(initData), func() string {
			if len(initData) > 100 {
				return initData[:100] + "..."
			}
			return initData
		}())

		// Try URL decoding in case the initData got encoded
		if decodedInitData, err := url.QueryUnescape(initData); err == nil && decodedInitData != initData {
			log.Printf("AuthMiddleware: Detected URL-encoded initData, using decoded version")
			initData = decodedInitData
		}

		// Skip validation in test mode
		if getTelegramBotToken() == "dummy-token" {
			log.Printf("AuthMiddleware: Using test mode (dummy-token)")
			// In test mode, skip validation and parse directly
			data, err := initdata.Parse(initData)
			if err != nil {
				log.Printf("AuthMiddleware: Failed to parse initData in test mode: %v", err)
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}

			log.Printf("AuthMiddleware: Parsed user data - ID: %d, Username: %s", data.User.ID, data.User.Username)

			user, err := db.UpsertUser(data.User.ID, data.User.Username)
			if err != nil {
				log.Printf("AuthMiddleware: Failed to upsert user: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			log.Printf("AuthMiddleware: Successfully authenticated user %d", user.ID)
			ctx := context.WithValue(r.Context(), models.UserContextKey, user)
			log.Printf("AuthMiddleware: Calling next handler for %s %s (test mode)", r.Method, r.URL.Path)
			next.ServeHTTP(w, r.WithContext(ctx))
			log.Printf("AuthMiddleware: Completed request %s %s (test mode)", r.Method, r.URL.Path)
			return
		}

		// Validate the initData for production
		log.Printf("AuthMiddleware: Validating initData for production")
		err := initdata.Validate(initData, getTelegramBotToken(), 1*time.Hour)
		if err != nil {
			log.Printf("AuthMiddleware: InitData validation failed: %v", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Parse the validated initData
		data, err := initdata.Parse(initData)
		if err != nil {
			log.Printf("AuthMiddleware: Failed to parse validated initData: %v", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		log.Printf("AuthMiddleware: Parsed user data - ID: %d, Username: %s", data.User.ID, data.User.Username)

		user, err := db.UpsertUser(data.User.ID, data.User.Username)
		if err != nil {
			log.Printf("AuthMiddleware: Failed to upsert user: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		log.Printf("AuthMiddleware: Successfully authenticated user %d", user.ID)
		ctx := context.WithValue(r.Context(), models.UserContextKey, user)
		log.Printf("AuthMiddleware: Calling next handler for %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r.WithContext(ctx))
		log.Printf("AuthMiddleware: Completed request %s %s", r.Method, r.URL.Path)
	})
}
