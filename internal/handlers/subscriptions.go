package handlers

import (
	"context"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"yt-podcaster/internal/db"
	"yt-podcaster/internal/middleware"
	"yt-podcaster/internal/models"
	"yt-podcaster/pkg/tasks"
)

const maxSubscriptionsPerUser = 20

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
	user := r.Context().Value(middleware.UserContextKey).(*models.User)

	// Check subscription limit
	count, err := db.CountSubscriptionsByUserID(user.ID)
	if err != nil {
		log.Printf("Error counting subscriptions: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if count >= maxSubscriptionsPerUser {
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

	// Use yt-dlp to get channel ID and title
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--print", "%(channel_id)s\n%(channel)s",
		"--playlist-items", "0",
		channelURL,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error getting channel info from yt-dlp for URL '%s': %v\nOutput: %s", channelURL, err, string(output))
		http.Error(w, "Invalid or unsupported YouTube URL", http.StatusBadRequest)
		return
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		log.Printf("Unexpected output from yt-dlp for URL '%s': %s", channelURL, string(output))
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
