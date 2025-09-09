package handlers

import (
	"context"
	"fmt"
	"io"
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

// extractChannelInfo extracts channel ID and title from YouTube channel URL using web scraping
func extractChannelInfo(ctx context.Context, channelURL string) (channelID, channelTitle string, err error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", channelURL, nil)
	if err != nil {
		return "", "", err
	}
	
	// Add realistic headers to avoid bot detection
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Connection", "keep-alive")
	
	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return "", "", err
	}
	
	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	
	html := string(body)
	
	// Extract channel ID from various possible patterns
	channelIDPatterns := []string{
		`"channelId":"([^"]+)"`,
		`"browse_id":"([^"]+)"`,
		`<link rel="canonical" href="https://www\.youtube\.com/channel/([^"]+)">`,
		`channel/([A-Za-z0-9_-]+)`,
	}
	
	for _, pattern := range channelIDPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(html)
		if len(matches) > 1 && strings.HasPrefix(matches[1], "UC") {
			channelID = matches[1]
			break
		}
	}
	
	// Extract channel title
	titlePatterns := []string{
		`<title>([^-]+) - YouTube</title>`,
		`"title":"([^"]+)"`,
		`<meta property="og:title" content="([^"]+)">`,
	}
	
	for _, pattern := range titlePatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(html)
		if len(matches) > 1 {
			channelTitle = strings.TrimSpace(matches[1])
			break
		}
	}
	
	if channelID == "" || channelTitle == "" {
		return "", "", fmt.Errorf("could not extract channel info from HTML")
	}
	
	return channelID, channelTitle, nil
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

	// Extract channel ID and title using web scraping (more reliable than yt-dlp)
	ctx, cancel := context.WithTimeout(r.Context(), getChannelInfoTimeout())
	defer cancel()

	log.Printf("Extracting channel info from URL: %s", channelURL)
	channelID, channelTitle, err := extractChannelInfo(ctx, channelURL)
	if err != nil {
		log.Printf("Error extracting channel info from URL '%s': %v", channelURL, err)
		http.Error(w, "Could not extract channel info from URL", http.StatusBadRequest)
		return
	}
	
	log.Printf("Extracted channel info - ID: %s, Title: %s", channelID, channelTitle)

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
