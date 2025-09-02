package handlers

import (
	"log"
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
	"yt-podcaster/internal/db"
	"yt-podcaster/internal/feed"
)

func (h *Handlers) GetRSSFeed(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	uuid := vars["uuid"]

	user, err := db.GetUserByRSSUUID(uuid)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	episodes, err := db.GetCompletedEpisodesByUserID(user.ID)
	if err != nil {
		log.Printf("Error getting episodes: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	rss, err := feed.GenerateRSS(user, episodes, r)
	if err != nil {
		log.Printf("Error generating RSS: %v", err)
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
