package models

import "time"

// Subscription represents a user's subscription to a YouTube channel.
type Subscription struct {
	ID                  int       `db:"id"`
	UserID              int64     `db:"user_id"`
	YoutubeChannelID    string    `db:"youtube_channel_id"`
	YoutubeChannelTitle string    `db:"youtube_channel_title"`
	RSSUUID             string    `db:"rss_uuid"`
	CreatedAt           time.Time `db:"created_at"`
}
