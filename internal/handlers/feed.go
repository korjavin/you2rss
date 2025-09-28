package handlers

import (
	"log"
	"net/http"
	"path/filepath"

	"yt-podcaster/internal/db"
	"yt-podcaster/internal/feed"

	"github.com/gorilla/mux"
)

func (h *Handlers) GetRSSFeed(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	uuid := vars["uuid"]

	// Get subscription by RSS UUID instead of user
	subscription, err := db.GetSubscriptionByRSSUUID(uuid)
	if err != nil {
		http.Error(w, "Subscription not found", http.StatusNotFound)
		return
	}

	// Get episodes for this specific subscription
	episodes, err := db.GetCompletedEpisodesBySubscriptionID(subscription.ID)
	if err != nil {
		log.Printf("Error getting episodes for subscription %d: %v", subscription.ID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Generate RSS for this specific subscription
	rss, err := feed.GenerateSubscriptionRSS(&subscription, episodes, r)
	if err != nil {
		log.Printf("Error generating RSS for subscription %d: %v", subscription.ID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/rss+xml")
	w.Write([]byte(rss))
}

func (h *Handlers) ServeAudioFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	filename := vars["filename"]

	filePath := filepath.Join(h.audioStoragePath, filename)
	http.ServeFile(w, r, filePath)
}
