package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"yt-podcaster/internal/db"
	"yt-podcaster/pkg/tasks"

	"github.com/hibiken/asynq"
)

var execCommand = exec.Command

type YtDlpOutput struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Duration    float64 `json:"duration"`
	Filename    string  `json:"_filename"`
}

type TaskHandler struct {
	asynqClient tasks.TaskEnqueuer
}

func NewTaskHandler(client tasks.TaskEnqueuer) *TaskHandler {
	return &TaskHandler{asynqClient: client}
}

func (h *TaskHandler) HandleProcessVideoTask(ctx context.Context, t *asynq.Task) error {
	var p tasks.ProcessVideoTaskPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("failed to unmarshal task payload: %w", err)
	}

	log.Printf("Processing video: %s", p.YoutubeVideoID)

	episode, err := db.GetEpisodeByYoutubeID(p.YoutubeVideoID)
	if err != nil {
		return fmt.Errorf("failed to get episode by youtube id: %w", err)
	}

	err = db.UpdateEpisodeStatus(episode.ID, db.StatusProcessing)
	if err != nil {
		return fmt.Errorf("failed to update episode status to processing: %w", err)
	}

	// Create a unique filename for the audio file
	audioFilename := fmt.Sprintf("%s.m4a", episode.AudioUUID)
	audioPath := filepath.Join("audio", audioFilename)

	// yt-dlp command
	cmd := execCommand("yt-dlp",
		"-x", // extract audio
		"--audio-format", "m4a",
		"-o", audioPath,
		"--print-json", // print video metadata as JSON
		fmt.Sprintf("https://www.youtube.com/watch?v=%s", p.YoutubeVideoID),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("failed to execute yt-dlp command: %v, output: %s", err, string(output))
		db.UpdateEpisodeProcessingFailed(episode.ID)
		return fmt.Errorf("failed to execute yt-dlp command: %w", err)
	}

	var ytDlpOutput YtDlpOutput
	// Sometimes yt-dlp prints other things to stdout before the JSON.
	// We'll try to extract the JSON from the output.
	jsonStartIndex := strings.Index(string(output), "{")
	if jsonStartIndex == -1 {
		log.Printf("no JSON found in yt-dlp output, output: %s", string(output))
		db.UpdateEpisodeProcessingFailed(episode.ID)
		return fmt.Errorf("no JSON found in yt-dlp output")
	}

	err = json.Unmarshal(output[jsonStartIndex:], &ytDlpOutput)
	if err != nil {
		log.Printf("failed to unmarshal yt-dlp output: %v, output: %s", err, string(output))
		db.UpdateEpisodeProcessingFailed(episode.ID)
		return fmt.Errorf("failed to unmarshal yt-dlp output: %w", err)
	}

	fileInfo, err := os.Stat(audioPath)
	if err != nil {
		log.Printf("failed to get file info for audio file: %v", err)
		db.UpdateEpisodeProcessingFailed(episode.ID)
		return fmt.Errorf("failed to get file info: %w", err)
	}

	err = db.UpdateEpisodeProcessingSuccess(episode.ID, ytDlpOutput.Title, ytDlpOutput.Description, audioPath, fileInfo.Size(), int(ytDlpOutput.Duration))
	if err != nil {
		return fmt.Errorf("failed to update episode processing success: %w", err)
	}

	log.Printf("Successfully processed video: %s", p.YoutubeVideoID)

	return nil
}

func (h *TaskHandler) HandleCheckAllSubscriptionsTask(ctx context.Context, t *asynq.Task) error {
	log.Println("Checking all subscriptions...")

	subscriptions, err := db.GetAllSubscriptions()
	if err != nil {
		return fmt.Errorf("failed to get all subscriptions: %w", err)
	}

	for _, sub := range subscriptions {
		task, err := tasks.NewCheckChannelTask(sub.ID)
		if err != nil {
			log.Printf("failed to create check channel task for subscription %d: %v", sub.ID, err)
			continue
		}

		_, err = h.asynqClient.Enqueue(task)
		if err != nil {
			log.Printf("failed to enqueue check channel task for subscription %d: %v", sub.ID, err)
			continue
		}
	}

	log.Println("Finished checking all subscriptions.")
	return nil
}

func (h *TaskHandler) HandleCheckChannelTask(ctx context.Context, t *asynq.Task) error {
	var p tasks.CheckChannelTaskPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("failed to unmarshal task payload: %w", err)
	}
	log.Printf("Checking channel for subscription: %d", p.SubscriptionID)

	// Get subscription
	subscription, err := db.GetSubscriptionByID(p.SubscriptionID)
	if err != nil {
		return fmt.Errorf("failed to get subscription by id: %w", err)
	}

	// Use yt-dlp to get the latest videos from the channel
	cmd := execCommand("yt-dlp",
		"--flat-playlist",
		"-j",
		fmt.Sprintf("https://www.youtube.com/channel/%s", subscription.YoutubeChannelID),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("failed to execute yt-dlp command: %v, output: %s", err, string(output))
		return fmt.Errorf("failed to execute yt-dlp command: %w", err)
	}

	// The output is a stream of JSON objects, one per line
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		var videoInfo struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal([]byte(line), &videoInfo); err != nil {
			log.Printf("failed to unmarshal video info: %v", err)
			continue
		}

		// Check if we already have this video
		_, err := db.GetEpisodeByYoutubeID(videoInfo.ID)
		if err == nil {
			// We already have this video, so we can skip it
			continue
		}

		// If we don't have this video, create a new episode and enqueue a task to process it
		episode, err := db.CreateEpisode(subscription.ID, videoInfo.ID)
		if err != nil {
			log.Printf("failed to create episode: %v", err)
			continue
		}

		// Enqueue a task to process the video
		task, err := tasks.NewProcessVideoTask(episode.YoutubeVideoID, episode.SubscriptionID)
		if err != nil {
			log.Printf("failed to create process video task: %v", err)
			continue
		}

		_, err = h.asynqClient.Enqueue(task)
		if err != nil {
			log.Printf("failed to enqueue process video task: %v", err)
			continue
		}
	}

	return nil
}
