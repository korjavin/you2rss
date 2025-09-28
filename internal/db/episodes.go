package db

import (
	"time"
	"yt-podcaster/internal/models"
)

const (
	StatusPending    = "PENDING"
	StatusProcessing = "PROCESSING"
	StatusCompleted  = "COMPLETED"
	StatusFailed     = "FAILED"
)

func CreateEpisode(subID int, videoID string) (models.Episode, error) {
	episode := models.Episode{}
	err := DB.Get(&episode, "INSERT INTO episodes (subscription_id, youtube_video_id) VALUES ($1, $2) RETURNING *", subID, videoID)
	return episode, err
}

func GetEpisodeByYoutubeID(videoID string) (models.Episode, error) {
	episode := models.Episode{}
	err := DB.Get(&episode, "SELECT * FROM episodes WHERE youtube_video_id = $1", videoID)
	return episode, err
}

func UpdateEpisodeStatus(id int, status string) error {
	_, err := DB.Exec("UPDATE episodes SET status = $1 WHERE id = $2", status, id)
	return err
}

func UpdateEpisodeProcessingSuccess(id int, title string, description string, audioPath string, audioSize int64, duration int, publishedAt time.Time) error {
	_, err := DB.Exec(`
		UPDATE episodes
		SET status = 'COMPLETED', title = $1, description = $2, audio_path = $3, audio_size_bytes = $4, duration_seconds = $5, published_at = $6
		WHERE id = $7`,
		title, description, audioPath, audioSize, duration, publishedAt, id)
	return err
}

func UpdateEpisodeProcessingFailed(id int) error {
	_, err := DB.Exec("UPDATE episodes SET status = 'FAILED' WHERE id = $1", id)
	return err
}

func GetCompletedEpisodesByUserID(userID int64) ([]models.Episode, error) {
	var episodes []models.Episode
	query := `
		SELECT e.*
		FROM episodes e
		JOIN subscriptions s ON e.subscription_id = s.id
		WHERE s.user_id = $1 AND e.status = 'COMPLETED'
		ORDER BY e.created_at DESC
	`
	err := DB.Select(&episodes, query, userID)
	return episodes, err
}

func GetFailedEpisodesOlderThan(duration time.Duration) ([]models.Episode, error) {
	var episodes []models.Episode
	cutoffTime := time.Now().Add(-duration)
	query := `
		SELECT * FROM episodes
		WHERE status = 'FAILED' AND updated_at < $1
		ORDER BY updated_at ASC
		LIMIT 50
	`
	err := DB.Select(&episodes, query, cutoffTime)
	return episodes, err
}

func GetCompletedEpisodesBySubscriptionID(subscriptionID int) ([]models.Episode, error) {
	var episodes []models.Episode
	query := `
		SELECT * FROM episodes
		WHERE subscription_id = $1 AND status = 'COMPLETED'
		ORDER BY published_at DESC
	`
	err := DB.Select(&episodes, query, subscriptionID)
	return episodes, err
}
