package db

import (
	"log"
	"yt-podcaster/internal/models"
)

// UpsertUser inserts a new user or updates an existing one based on the Telegram ID.
func UpsertUser(id int64, username string) (*models.User, error) {
	query := `
		INSERT INTO users (id, telegram_username)
		VALUES ($1, $2)
		ON CONFLICT (id) DO UPDATE SET
			telegram_username = EXCLUDED.telegram_username,
			updated_at = NOW()
		RETURNING id, telegram_username, rss_uuid, created_at, updated_at
	`
	user := &models.User{}
	err := DB.Get(user, query, id, username)
	if err != nil {
		log.Printf("Error upserting user: %v", err)
		return nil, err
	}
	return user, nil
}
