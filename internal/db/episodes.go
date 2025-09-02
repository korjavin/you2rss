package db

import (
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

func UpdateEpisodeProcessingSuccess(id int, title string, description string, audioPath string, audioSize int64, duration int) error {
	_, err := DB.Exec(`
		UPDATE episodes
		SET status = 'COMPLETED', title = $1, description = $2, audio_path = $3, audio_size_bytes = $4, duration_seconds = $5
		WHERE id = $6`,
		title, description, audioPath, audioSize, duration, id)
	return err
}

func UpdateEpisodeProcessingFailed(id int) error {
	_, err := DB.Exec("UPDATE episodes SET status = 'FAILED' WHERE id = $1", id)
	return err
}
