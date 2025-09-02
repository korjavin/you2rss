package handlers

import (
	"log"
	"net/http"
	"regexp"
	"strconv"

	"github.com/gorilla/mux"
	"yt-podcaster/internal/db"
	"yt-podcaster/internal/middleware"
	"yt-podcaster/internal/models"
	"yt-podcaster/pkg/tasks"
)

var youtubeChannelIDRegex = regexp.MustCompile(`youtube\.com/channel/([a-zA-Z0-9_-]+)`)

func (h *Handlers) GetRoot(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(middleware.UserContextKey).(*models.User)

	subscriptions, err := db.GetSubscriptionsByUserID(user.ID)
	if err != nil {
		log.Printf("Error getting subscriptions: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Subscriptions": subscriptions,
	}

	err = h.templates.ExecuteTemplate(w, "index.html", data)
	if err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
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
	user := r.Context().Value(middleware.UserContextKey).(*models.User)

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	channelURL := r.FormValue("url")
	matches := youtubeChannelIDRegex.FindStringSubmatch(channelURL)
	if len(matches) < 2 {
		http.Error(w, "Invalid YouTube channel URL", http.StatusBadRequest)
		return
	}
	channelID := matches[1]

	sub, err := db.AddSubscription(user.ID, channelID, channelID)
	if err != nil {
		log.Printf("Error creating subscription: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Enqueue a task to check the channel for new videos
	task, err := tasks.NewCheckChannelTask(sub.ID)
	if err != nil {
		log.Printf("Error creating task: %v", err)
		// Don't block the user, just log the error
	} else {
		_, err = h.asynqClient.Enqueue(task)
		if err != nil {
			log.Printf("Error enqueuing task: %v", err)
			// Don't block the user, just log the error
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
