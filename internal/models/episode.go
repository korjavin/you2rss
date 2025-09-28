package models

import "time"

type Episode struct {
	ID              int        `db:"id"`
	SubscriptionID  int        `db:"subscription_id"`
	YoutubeVideoID  string     `db:"youtube_video_id"`
	Title           *string    `db:"title"`
	Description     *string    `db:"description"`
	PublishedAt     *time.Time `db:"published_at"`
	AudioUUID       string     `db:"audio_uuid"`
	AudioPath       *string    `db:"audio_path"`
	AudioSizeBytes  *int64     `db:"audio_size_bytes"`
	DurationSeconds *int       `db:"duration_seconds"`
	Status          string     `db:"status"`
	CreatedAt       time.Time  `db:"created_at"`
	TaskID          *string    `db:"task_id"`
}
