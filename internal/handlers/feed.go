package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/eduncan911/podcast"
	"yt-podcaster/internal/db"
)

func RSSFeedHandler(w http.ResponseWriter, r *http.Request) {
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		log.Println("BASE_URL not set")
		http.Error(w, "Server configuration error", http.StatusInternalServerError)
		return
	}

	// Get user_rss_uuid from URL path: /rss/{user_rss_uuid}
	uuid := strings.TrimPrefix(r.URL.Path, "/rss/")
	if uuid == "" {
		http.Error(w, "Invalid RSS feed URL", http.StatusBadRequest)
		return
	}

	user, err := db.GetUserByRSSUUID(uuid)
	if err != nil {
		log.Printf("Error getting user by rss_uuid %s: %v", uuid, err)
		http.NotFound(w, r)
		return
	}

	episodes, err := db.GetCompletedEpisodesByUserID(user.ID)
	if err != nil {
		log.Printf("Error getting episodes for user %d: %v", user.ID, err)
		http.Error(w, "Could not retrieve episodes", http.StatusInternalServerError)
		return
	}

	p := podcast.New(
		fmt.Sprintf("%s's Podcast", user.TelegramUsername),
		fmt.Sprintf("%s/rss/%s", baseURL, user.RSSUUID),
		"A podcast generated from YouTube channels.",
		&user.CreatedAt, &user.UpdatedAt,
	)
	p.AddAuthor(user.TelegramUsername, "")
	p.AddImage(fmt.Sprintf("%s/icon.png", baseURL)) // Assuming a static icon

	for _, episode := range episodes {
		audioURL := fmt.Sprintf("%s/audio/%s.m4a", baseURL, episode.AudioUUID)

		title := ""
		if episode.Title != nil {
			title = *episode.Title
		}

		description := ""
		if episode.Description != nil {
			description = *episode.Description
		}

		size := int64(0)
		if episode.AudioSizeBytes != nil {
			size = *episode.AudioSizeBytes
		}

		item := podcast.Item{
			Title:       title,
			Description: description,
			Link:        audioURL,
			PubDate:     &episode.CreatedAt,
		}
		item.AddEnclosure(audioURL, podcast.M4A, size)
		if _, err := p.AddItem(item); err != nil {
			log.Printf("Error adding item to podcast: %v", err)
		}
	}

	w.Header().Set("Content-Type", "application/rss+xml")
	if err := p.Encode(w); err != nil {
		log.Printf("Error encoding RSS feed: %v", err)
		http.Error(w, "Could not generate RSS feed", http.StatusInternalServerError)
	}
}

func AudioServerHandler(w http.ResponseWriter, r *http.Request) {
	audioStoragePath := os.Getenv("AUDIO_STORAGE_PATH")
	if audioStoragePath == "" {
		log.Println("AUDIO_STORAGE_PATH not set")
		http.Error(w, "Server configuration error", http.StatusInternalServerError)
		return
	}

	// Get audio_uuid from URL path: /audio/{audio_uuid}.m4a
	fileName := strings.TrimPrefix(r.URL.Path, "/audio/")
	filePath := filepath.Join(audioStoragePath, fileName)

	// Security: Check if the resolved path is still within the audio storage directory
	if !strings.HasPrefix(filePath, audioStoragePath) {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	http.ServeFile(w, r, filePath)
}
