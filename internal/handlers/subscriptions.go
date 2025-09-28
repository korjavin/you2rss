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

	"yt-podcaster/internal/db"
	"yt-podcaster/internal/models"
	"yt-podcaster/pkg/tasks"

	"github.com/gorilla/mux"
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
	timeout := 10 * time.Second // reduced timeout for web scraping
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
	// Create HTTP client with timeout and redirect handling
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Allow up to 10 redirects
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	// Try different URL formats to bypass consent pages
	urlsToTry := []string{
		channelURL,
		channelURL + "?hl=en&gl=US", // Force English locale and US region
	}

	var html string
	for _, url := range urlsToTry {
		// Create request with context
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			continue
		}

		// Add realistic headers to avoid bot detection
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("Accept-Encoding", "identity") // Don't use compression to avoid issues
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Pragma", "no-cache")

		// Make the request
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("HTTP request failed for %s: %v", url, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			log.Printf("HTTP %d for URL: %s", resp.StatusCode, url)
			continue
		}

		// Read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Failed to read response body: %v", err)
			continue
		}

		html = string(body)

		// Check if we got actual YouTube content (not a consent page)
		if strings.Contains(html, "youtube.com") && !strings.Contains(html, "consent.youtube.com") {
			break
		}

		log.Printf("Got consent page or invalid response for %s, trying next URL", url)
	}

	if html == "" {
		return "", "", fmt.Errorf("could not fetch valid YouTube page content")
	}

	// Log HTML snippet for debugging (first 500 chars)
	htmlSnippet := html
	if len(htmlSnippet) > 500 {
		htmlSnippet = htmlSnippet[:500] + "..."
	}
	log.Printf("HTML snippet for debugging: %s", htmlSnippet)

	// Extract channel ID from various possible patterns
	channelIDPatterns := []string{
		`"channelId":"([^"]+)"`,
		`"browse_id":"([^"]+)"`,
		`<link rel="canonical" href="https://www\.youtube\.com/channel/([^"]+)">`,
		`<link rel="canonical" href="https://www\.youtube\.com/channel/([^"?]+)`,
		`channel/([A-Za-z0-9_-]{24})`, // YouTube channel IDs are exactly 24 characters
		`"externalId":"([^"]+)"`,
		`data-channel-external-id="([^"]+)"`,
	}

	log.Printf("Searching for channel ID...")
	for i, pattern := range channelIDPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(html)
		log.Printf("Pattern %d (%s): matches=%v", i+1, pattern, len(matches) > 1)
		if len(matches) > 1 {
			candidate := matches[1]
			log.Printf("Found candidate channel ID: %s", candidate)
			// Check if it looks like a valid YouTube channel ID (starts with UC and is 24 chars)
			if strings.HasPrefix(candidate, "UC") && len(candidate) == 24 {
				channelID = candidate
				log.Printf("Valid channel ID found: %s", channelID)
				break
			}
		}
	}

	// Extract channel title
	titlePatterns := []string{
		`<title>([^-\|]+) - YouTube</title>`,
		`<title>([^-\|]+)\s*-\s*YouTube</title>`,
		`"title":"([^"]+)"`,
		`<meta property="og:title" content="([^"]+)"`,
		`<meta name="title" content="([^"]+)"`,
	}

	log.Printf("Searching for channel title...")
	for i, pattern := range titlePatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(html)
		log.Printf("Title pattern %d: matches=%v", i+1, len(matches) > 1)
		if len(matches) > 1 {
			candidate := strings.TrimSpace(matches[1])
			if candidate != "" && candidate != "YouTube" {
				channelTitle = candidate
				log.Printf("Channel title found: %s", channelTitle)
				break
			}
		}
	}

	log.Printf("Final extraction results - ID: '%s', Title: '%s'", channelID, channelTitle)

	if channelID == "" || channelTitle == "" {
		return "", "", fmt.Errorf("could not extract channel info from HTML (ID found: %v, Title found: %v)", channelID != "", channelTitle != "")
	}

	return channelID, channelTitle, nil
}

func (h *Handlers) GetSubscriptions(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(models.UserContextKey).(*models.User)

	subscriptions, err := db.GetSubscriptionsByUserID(user.ID)
	if err != nil {
		log.Printf("Error getting subscriptions: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get BASE_URL from environment
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080" // fallback for development
	}

	// Create template data with subscriptions and base URL for individual RSS feeds
	templateData := struct {
		Subscriptions []models.Subscription
		BaseURL       string
	}{
		Subscriptions: subscriptions,
		BaseURL:       baseURL,
	}

	err = h.templates.ExecuteTemplate(w, "subscriptions.html", templateData)
	if err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *Handlers) PostSubscription(w http.ResponseWriter, r *http.Request) {
	log.Printf("PostSubscription called - Method: %s, URL: %s, Content-Type: %s", r.Method, r.URL.String(), r.Header.Get("Content-Type"))

	user := r.Context().Value(models.UserContextKey).(*models.User)
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
		// Use default options for channel checking (less aggressive than video processing)
		_, err = h.asynqClient.Enqueue(task)
		if err != nil {
			log.Printf("Error enqueuing task: %v", err)
		}
	}

	h.GetSubscriptions(w, r)
}

func (h *Handlers) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(models.UserContextKey).(*models.User)
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
