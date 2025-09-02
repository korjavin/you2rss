package models

import "time"

// User represents a user in the database.
type User struct {
	ID               int64     `db:"id"`
	TelegramUsername string    `db:"telegram_username"`
	RSSUUID          string    `db:"rss_uuid"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
}
