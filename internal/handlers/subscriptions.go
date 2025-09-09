package handlers

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"yt-podcaster/internal/db"
	"yt-podcaster/internal/middleware"
	"yt-podcaster/internal/models"
	"yt-podcaster/pkg/tasks"
)

// execCommandContext can be mocked in tests
var execCommandContext = exec.CommandContext

func getMaxSubscriptionsPerUser() int {
	maxSubs := 100 // default as suggested in review
	if env := os.Getenv("MAX_SUBSCRIPTIONS_PER_USER"); env != "" {
		if val, err := strconv.Atoi(env); err == nil {
			maxSubs = val
		}
	}
	return maxSubs
}

func getChannelInfoTimeout() time.Duration {
	timeout := 15 * time.Second // default as in original code
	if env := os.Getenv("CHANNEL_INFO_TIMEOUT_SECONDS"); env != "" {
		if val, err := strconv.Atoi(env); err == nil {
			timeout = time.Duration(val) * time.Second
		}
	}
	return timeout
}

// validateYouTubeURL validates that the URL is a proper YouTube channel URL
// to prevent SSRF attacks by ensuring only YouTube domains are accessed
func validateYouTubeURL(url string) bool {
	// Regular expressions for valid YouTube channel and user URLs
	patterns := []string{
		`^https://(?:www\.)?youtube\.com/channel/[a-zA-Z0-9_-]+(?:/.*)?$`,
		`^https://(?:www\.)?youtube\.com/@[a-zA-Z0-9_.-]+(?:/.*)?$`,
		`^https://(?:www\.)?youtube\.com/user/[a-zA-Z0-9_.-]+(?:/.*)?$`,
		`^https://(?:www\.)?youtube\.com/c/[a-zA-Z0-9_.-]+(?:/.*)?$`,
	}
	
	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, url); matched {
			return true
		}
	}
	
	return false
}

func (h *Handlers) GetSubscriptions(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(middleware.UserContextKey).(*models.User)

	subscriptions, err := db.GetSubscriptionsByUserID(user.ID)
	if err != nil {
		log.Printf("Error getting subscriptions: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	err = h.templates.ExecuteTemplate(w, "subscriptions.html", subscriptions)
	if err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *Handlers) PostSubscription(w http.ResponseWriter, r *http.Request) {
	log.Printf("PostSubscription called - Method: %s, URL: %s, Content-Type: %s", r.Method, r.URL.String(), r.Header.Get("Content-Type"))
	
	user := r.Context().Value(middleware.UserContextKey).(*models.User)
	if user == nil {
		log.Printf("PostSubscription: No user in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	log.Printf("PostSubscription: User ID %d (%s) requesting subscription", user.ID, user.TelegramUsername)

	// Check subscription limit
	count, err := db.CountSubscriptionsByUserID(user.ID)
	if err != nil {
		log.Printf("Error counting subscriptions: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if count >= getMaxSubscriptionsPerUser() {
		http.Error(w, "Subscription limit reached", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	channelURL := r.FormValue("url")
	if channelURL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Validate URL to prevent SSRF attacks
	if !validateYouTubeURL(channelURL) {
		http.Error(w, "Invalid YouTube URL format", http.StatusBadRequest)
		return
	}

	// Check if yt-dlp is installed and working
	log.Printf("Checking yt-dlp installation...")
	versionCmd := execCommandContext(context.Background(), "yt-dlp", "--version")
	versionOutput, versionErr := versionCmd.CombinedOutput()
	if versionErr != nil {
		log.Printf("yt-dlp not found or not working: %v\nOutput: %s", versionErr, string(versionOutput))
		http.Error(w, "Video processing service unavailable", http.StatusServiceUnavailable)
		return
	}
	log.Printf("yt-dlp version: %s", strings.TrimSpace(string(versionOutput)))

	// Use yt-dlp to get channel ID and title
	ctx, cancel := context.WithTimeout(r.Context(), getChannelInfoTimeout())
	defer cancel()

	log.Printf("Running yt-dlp command for URL: %s", channelURL)
	cmd := execCommandContext(ctx, "yt-dlp",
		"--print", "%(channel_id)s\n%(channel)s",
		"--playlist-items", "0",
		"--no-warnings",
		"--quiet", 
		"--no-check-certificate",
		channelURL,
	)

	output, err := cmd.CombinedOutput()
	log.Printf("yt-dlp raw output (len=%d): '%s'", len(output), string(output))
	log.Printf("yt-dlp command error: %v", err)
	
	if err != nil {
		log.Printf("Error getting channel info from yt-dlp for URL '%s': %v\nOutput: %s", channelURL, err, string(output))
		http.Error(w, "Invalid or unsupported YouTube URL", http.StatusBadRequest)
		return
	}

	// Clean the output by removing debug lines and empty lines
	outputStr := strings.TrimSpace(string(output))
	lines := []string{}
	for _, line := range strings.Split(outputStr, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "[") && !strings.HasPrefix(line, "Loaded ") {
			lines = append(lines, line)
		}
	}
	
	log.Printf("Cleaned yt-dlp output lines: %v", lines)
	
	if len(lines) < 2 {
		log.Printf("Insufficient channel info from yt-dlp for URL '%s'. Lines found: %d", channelURL, len(lines))
		// Try alternative extraction method
		log.Printf("Attempting fallback channel extraction...")
		fallbackCmd := execCommandContext(ctx, "yt-dlp", 
			"--dump-json", 
			"--playlist-items", "0",
			"--no-warnings",
			channelURL,
		)
		fallbackOutput, fallbackErr := fallbackCmd.CombinedOutput()
		if fallbackErr == nil {
			output := string(fallbackOutput)
			if len(output) > 200 {
				output = output[:200] + "..."
			}
			log.Printf("Fallback JSON output: %s", output)
		}
		http.Error(w, "Could not extract channel info from URL", http.StatusBadRequest)
		return
	}
	channelID := lines[0]
	channelTitle := lines[1]

	if channelID == "" || channelTitle == "" || channelTitle == "NA" {
		log.Printf("Could not extract valid channel info from yt-dlp for URL '%s': ID='%s', Title='%s'", channelURL, channelID, channelTitle)
		http.Error(w, "Could not extract valid channel info from URL", http.StatusBadRequest)
		return
	}

	sub, err := db.AddSubscription(user.ID, channelID, channelTitle)
	if err != nil {
		log.Printf("Error creating subscription: %v", err)
		// Handle potential duplicate subscription error gracefully
		if strings.Contains(err.Error(), "subscriptions_user_id_youtube_channel_id_key") {
			http.Error(w, "You are already subscribed to this channel.", http.StatusConflict)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Enqueue a task to check the channel for new videos
	task, err := tasks.NewCheckChannelTask(sub.ID)
	if err != nil {
		log.Printf("Error creating task: %v", err)
	} else {
		_, err = h.asynqClient.Enqueue(task)
		if err != nil {
			log.Printf("Error enqueuing task: %v", err)
		}
	}

	h.GetSubscriptions(w, r)
}

func (h *Handlers) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(middleware.UserContextKey).(*models.User)
	vars := mux.Vars(r)
	subscriptionID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid subscription ID", http.StatusBadRequest)
		return
	}

	err = db.DeleteSubscription(user.ID, subscriptionID)
	if err != nil {
		log.Printf("Error deleting subscription: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
