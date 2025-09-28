package worker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"yt-podcaster/internal/db"
	"yt-podcaster/pkg/tasks"

	"github.com/hibiken/asynq"
)

var execCommand = exec.Command
var execCommandContext = exec.CommandContext

// setupCookieFile creates a temporary cookie file from base64 encoded environment variable
func setupCookieFile() (string, func(), error) {
	cookieBase64 := os.Getenv("YOUTUBE_COOKIES_BASE64")
	if cookieBase64 == "" {
		// No cookies provided, return empty string to indicate no cookie file
		return "", func() {}, nil
	}

	// Decode base64 cookie data
	cookieData, err := base64.StdEncoding.DecodeString(cookieBase64)
	if err != nil {
		return "", func() {}, fmt.Errorf("failed to decode base64 cookies: %w", err)
	}

	// Create temporary cookie file
	tmpFile, err := ioutil.TempFile("", "youtube_cookies_*.txt")
	if err != nil {
		return "", func() {}, fmt.Errorf("failed to create temporary cookie file: %w", err)
	}

	// Write cookie data to file
	if _, err := tmpFile.Write(cookieData); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", func() {}, fmt.Errorf("failed to write cookie data: %w", err)
	}

	tmpFile.Close()

	// Return cleanup function
	cleanup := func() {
		os.Remove(tmpFile.Name())
	}

	return tmpFile.Name(), cleanup, nil
}

// getYouTubeRequestDelay returns delay between YouTube requests to be gentle
func getYouTubeRequestDelay() time.Duration {
	delay := 30 * time.Second // default gentle delay
	if env := os.Getenv("YOUTUBE_REQUEST_DELAY_SECONDS"); env != "" {
		if val, err := strconv.Atoi(env); err == nil {
			delay = time.Duration(val) * time.Second
		}
	}
	return delay
}

// calculateExponentialBackoff calculates delay for retry attempts
func calculateExponentialBackoff(attempt int) time.Duration {
	baseDelay := 5 * time.Minute
	maxDelay := 24 * time.Hour

	// Custom base delay from env
	if env := os.Getenv("RETRY_BASE_DELAY_MINUTES"); env != "" {
		if val, err := strconv.Atoi(env); err == nil {
			baseDelay = time.Duration(val) * time.Minute
		}
	}

	// Exponential backoff: 5min, 10min, 20min, 40min, 80min, 160min, then cap at 24h
	delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

// isTemporaryYouTubeError determines if an error is worth retrying
func isTemporaryYouTubeError(output string) bool {
	temporaryErrors := []string{
		"Sign in to confirm you're not a bot",
		"HTTP Error 429", // Rate limited
		"HTTP Error 503", // Service unavailable
		"HTTP Error 502", // Bad gateway
		"HTTP Error 500", // Internal server error
		"timeout",
		"connection refused",
		"connection reset",
		"network is unreachable",
		"temporary failure in name resolution",
	}

	outputLower := strings.ToLower(output)
	for _, errPattern := range temporaryErrors {
		if strings.Contains(outputLower, strings.ToLower(errPattern)) {
			return true
		}
	}

	return false
}

// isPermanentYouTubeError determines if an error is permanent and should not be retried
func isPermanentYouTubeError(output string) bool {
	permanentErrors := []string{
		"Video unavailable",
		"Private video",
		"This video is not available",
		"HTTP Error 404", // Video not found
		"HTTP Error 403", // Forbidden (usually permanent)
		"Video was deleted",
		"Copyright",
		"This video has been removed",
	}

	outputLower := strings.ToLower(output)
	for _, errPattern := range permanentErrors {
		if strings.Contains(outputLower, strings.ToLower(errPattern)) {
			return true
		}
	}

	return false
}

func getProcessVideoTimeout() time.Duration {
	timeout := 15 * time.Minute // default as in original code
	if env := os.Getenv("PROCESS_VIDEO_TIMEOUT_MINUTES"); env != "" {
		if val, err := strconv.Atoi(env); err == nil {
			timeout = time.Duration(val) * time.Minute
		}
	}
	return timeout
}

func getCheckChannelTimeout() time.Duration {
	timeout := 2 * time.Minute // default timeout for channel checking
	if env := os.Getenv("CHECK_CHANNEL_TIMEOUT_MINUTES"); env != "" {
		if val, err := strconv.Atoi(env); err == nil {
			timeout = time.Duration(val) * time.Minute
		}
	}
	return timeout
}

type YtDlpOutput struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Duration    float64 `json:"duration"`
	Filename    string  `json:"_filename"`
	UploadDate  string  `json:"upload_date"`
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

	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(ctx, getProcessVideoTimeout())
	defer cancel()

	// Setup cookie file if available
	cookieFile, cleanupCookie, err := setupCookieFile()
	if err != nil {
		log.Printf("Warning: failed to setup cookie file: %v", err)
	}
	defer cleanupCookie()

	// Build yt-dlp command arguments
	args := []string{
		"-x", // extract audio
		"--audio-format", "m4a",
		"-o", audioPath,
		"--print-json", // print video metadata as JSON
		"--user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"--add-header", "Accept-Language:en-US,en;q=0.9",
		"--extractor-args", "youtube:player_client=android",
	}

	// Add cookie file if available
	if cookieFile != "" {
		args = append(args, "--cookies", cookieFile)
		log.Printf("Using cookie file for authentication")
	}

	// Add the video URL
	args = append(args, fmt.Sprintf("https://www.youtube.com/watch?v=%s", p.YoutubeVideoID))

	cmd := execCommandContext(ctx, "yt-dlp", args...)

	// Add gentle delay before making YouTube request
	time.Sleep(getYouTubeRequestDelay())

	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		log.Printf("failed to execute yt-dlp command: %v, output: %s", err, outputStr)

		// Check if this is a permanent error
		if isPermanentYouTubeError(outputStr) {
			log.Printf("Permanent error detected for video %s, marking as failed", p.YoutubeVideoID)
			db.UpdateEpisodeProcessingFailed(episode.ID)
			return fmt.Errorf("permanent error: %w", err)
		}

		// Check if this is a temporary error worth retrying
		if isTemporaryYouTubeError(outputStr) {
			log.Printf("Temporary error detected for video %s, will retry", p.YoutubeVideoID)
			// Return a regular error - asynq will handle the retry with exponential backoff
			return fmt.Errorf("temporary YouTube error: %w", err)
		}

		// Unknown error - mark as failed for now but could be retried manually
		log.Printf("Unknown error for video %s, marking as failed", p.YoutubeVideoID)
		db.UpdateEpisodeProcessingFailed(episode.ID)
		return fmt.Errorf("unknown error: %w", err)
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

	// Parse the upload date
	var publishedAt time.Time
	if ytDlpOutput.UploadDate != "" {
		if t, err := time.Parse("20060102", ytDlpOutput.UploadDate); err == nil {
			publishedAt = t
		} else {
			publishedAt = time.Now()
		}
	} else {
		publishedAt = time.Now()
	}

	err = db.UpdateEpisodeProcessingSuccess(episode.ID, ytDlpOutput.Title, ytDlpOutput.Description, audioPath, fileInfo.Size(), int(ytDlpOutput.Duration), publishedAt)
	if err != nil {
		return fmt.Errorf("failed to update episode processing success: %w", err)
	}

	log.Printf("Successfully processed video: %s", p.YoutubeVideoID)

	return nil
}

func (h *TaskHandler) HandleRetryFailedEpisodesTask(ctx context.Context, t *asynq.Task) error {
	log.Println("Retrying failed episodes...")

	// Get failed episodes that haven't been updated recently (avoid immediate retries)
	episodes, err := db.GetFailedEpisodesOlderThan(time.Hour)
	if err != nil {
		return fmt.Errorf("failed to get failed episodes: %w", err)
	}

	retriedCount := 0
	for _, episode := range episodes {
		// Check if this looks like a temporary error worth retrying
		// For now, we'll retry all failed episodes that are old enough

		log.Printf("Retrying failed episode: %s", episode.YoutubeVideoID)

		// Reset episode status to pending so it can be processed again
		err = db.UpdateEpisodeStatus(episode.ID, db.StatusPending)
		if err != nil {
			log.Printf("Failed to reset episode status for %s: %v", episode.YoutubeVideoID, err)
			continue
		}

		// Enqueue a new process video task with delay to spread out the load
		task, err := tasks.NewProcessVideoTask(episode.YoutubeVideoID, episode.SubscriptionID)
		if err != nil {
			log.Printf("Failed to create process video task for %s: %v", episode.YoutubeVideoID, err)
			continue
		}

		// Add some delay between retries to be even more gentle
		delay := time.Duration(retriedCount*30) * time.Second
		options := append(tasks.GetProcessVideoTaskOptions(), asynq.ProcessIn(delay))
		_, err = h.asynqClient.Enqueue(task, options...)
		if err != nil {
			log.Printf("Failed to enqueue process video task for %s: %v", episode.YoutubeVideoID, err)
			continue
		}

		retriedCount++
	}

	log.Printf("Retried %d failed episodes", retriedCount)
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

		_, err = h.asynqClient.Enqueue(task, asynq.Queue("high"))
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
	// Create a context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(ctx, getCheckChannelTimeout())
	defer cancel()

	// Setup cookie file if available
	cookieFile, cleanupCookie, err := setupCookieFile()
	if err != nil {
		log.Printf("Warning: failed to setup cookie file: %v", err)
	}
	defer cleanupCookie()

	// Try multiple URL formats for better compatibility
	channelURL := fmt.Sprintf("https://www.youtube.com/channel/%s/videos", subscription.YoutubeChannelID)

	// Build yt-dlp command arguments
	args := []string{
		"--flat-playlist",
		"-j",
		"--playlist-end", "20", // Limit to 20 most recent videos
		"--user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"--add-header", "Accept-Language:en-US,en;q=0.9",
		"--extractor-args", "youtube:player_client=android",
	}

	// Add cookie file if available
	if cookieFile != "" {
		args = append(args, "--cookies", cookieFile)
		log.Printf("Using cookie file for channel check")
	}

	// Add the channel URL
	args = append(args, channelURL)

	cmd := execCommandContext(ctx, "yt-dlp", args...)

	// Add gentle delay before making YouTube request
	time.Sleep(getYouTubeRequestDelay())

	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		log.Printf("failed to execute yt-dlp command for channel check: %v, output: %s", err, outputStr)

		// For channel checking, we're more lenient and always retry temporary errors
		if isTemporaryYouTubeError(outputStr) {
			log.Printf("Temporary error checking channel %s, will retry", subscription.YoutubeChannelID)
			return fmt.Errorf("temporary error checking channel: %w", err)
		}

		// Even for "unknown" errors in channel checking, we'll log but not fail permanently
		log.Printf("Error checking channel %s: %v", subscription.YoutubeChannelID, err)
		return fmt.Errorf("error checking channel: %w", err)
	}

	// The output is a stream of JSON objects, one per line
	var videos []YtDlpOutput
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		var videoInfo YtDlpOutput
		if err := json.Unmarshal([]byte(line), &videoInfo); err != nil {
			log.Printf("failed to unmarshal video info: %v", err)
			continue
		}
		videos = append(videos, videoInfo)
	}

	// Determine if this is a new channel
	isNewChannel, err := db.IsNewChannel(subscription.ID)
	if err != nil {
		return fmt.Errorf("failed to check if channel is new: %w", err)
	}

	// Get the upload date of the newest video
	var newestVideoDate time.Time
	if len(videos) > 0 {
		if t, err := time.Parse("20060102", videos[0].UploadDate); err == nil {
			newestVideoDate = t
		}
	}

	// Calculate the cutoff date (1 year before the newest video)
	cutoffDate := newestVideoDate.AddDate(-1, 0, 0)

	processedCount := 0
	for i, videoInfo := range videos {
		// Check if we already have this video
		_, err := db.GetEpisodeByYoutubeID(videoInfo.ID)
		if err == nil {
			// We already have this video, so we can skip it
			continue
		}

		// If it's a new channel, limit to 50 videos
		if isNewChannel && processedCount >= 50 {
			break
		}

		// Check if the video is older than 1 year from the newest video
		if uploadDate, err := time.Parse("20060102", videoInfo.UploadDate); err == nil {
			if uploadDate.Before(cutoffDate) {
				continue
			}
		}

		// If we don't have this video, create a new episode and enqueue a task to process it
		episode, err := db.CreateEpisode(subscription.ID, videoInfo.ID)
		if err != nil {
			log.Printf("failed to create episode: %v", err)
			continue
		}

		// Enqueue a task to process the video with enhanced retry options
		task, err := tasks.NewProcessVideoTask(episode.YoutubeVideoID, episode.SubscriptionID)
		if err != nil {
			log.Printf("failed to create process video task: %v", err)
			continue
		}

		// Prioritize newer videos (lower index = newer video)
		var opts []asynq.Option
		if i < 10 {
			opts = append(opts, asynq.Queue("high"))
		}

		_, err = h.asynqClient.Enqueue(task, opts...)
		if err != nil {
			log.Printf("failed to enqueue process video task: %v", err)
			continue
		}
		processedCount++
	}

	return nil
}
