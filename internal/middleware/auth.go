package middleware

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"yt-podcaster/internal/db"

	"github.com/telegram-mini-apps/init-data-golang"
)

type contextKey string

// UserContextKey is the key for the user in the context.
const UserContextKey = contextKey("user")

// AuthMiddleware validates the Telegram Mini App initData and upserts the user.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header is required", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "tma" {
			http.Error(w, "Authorization header format must be 'tma <initData>'", http.StatusUnauthorized)
			return
		}

		initData := parts[1]
		botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
		if botToken == "" {
			log.Println("TELEGRAM_BOT_TOKEN is not set")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Validate the initData
		if err := initdata.Validate(initData, botToken, 0); err != nil {
			log.Printf("Invalid init data: %v", err)
			http.Error(w, "Invalid init data", http.StatusUnauthorized)
			return
		}

		// Parse the init data
		data, err := initdata.Parse(initData)
		if err != nil {
			log.Printf("Error parsing init data: %v", err)
			http.Error(w, "Error parsing init data", http.StatusBadRequest)
			return
		}

		// Upsert user
		user, err := db.UpsertUser(data.User.ID, data.User.Username)
		if err != nil {
			http.Error(w, "Failed to authenticate user", http.StatusInternalServerError)
			return
		}

		// Store user in context
		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
