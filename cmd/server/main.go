package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/joho/godotenv"
	"yt-podcaster/internal/db"
	"yt-podcaster/internal/middleware"
	"yt-podcaster/internal/models"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}

	db.InitDB()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	rootHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Naive path resolving. Would be better to use embed or a more robust solution
		fp := filepath.Join("web", "templates", "index.html")
		tmpl, err := template.ParseFiles(fp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tmpl.Execute(w, nil); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	http.Handle("/", middleware.AuthMiddleware(rootHandler))
	http.Handle("/subscriptions", middleware.AuthMiddleware(http.HandlerFunc(subscriptionsHandler)))

	log.Printf("Starting server on :%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func subscriptionsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getSubscriptionsHandler(w, r)
	case http.MethodPost:
		postSubscriptionsHandler(w, r)
	case http.MethodDelete:
		deleteSubscriptionsHandler(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func deleteSubscriptionsHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(middleware.UserContextKey).(*models.User)
	if !ok {
		http.Error(w, "User not found in context", http.StatusInternalServerError)
		return
	}

	// Get ID from URL path: /subscriptions/{id}
	path := strings.TrimPrefix(r.URL.Path, "/subscriptions/")
	id, err := strconv.Atoi(path)
	if err != nil {
		http.Error(w, "Invalid subscription ID", http.StatusBadRequest)
		return
	}

	err = db.DeleteSubscription(user.ID, id)
	if err != nil {
		http.Error(w, "Failed to delete subscription", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func postSubscriptionsHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(middleware.UserContextKey).(*models.User)
	if !ok {
		http.Error(w, "User not found in context", http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	url := r.FormValue("url")
	if url == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Naive way to get channel ID. A proper solution would use the YouTube API.
	parts := strings.Split(url, "/")
	if len(parts) < 2 {
		http.Error(w, "Invalid YouTube channel URL", http.StatusBadRequest)
		return
	}
	channelID := parts[len(parts)-1]

	// For now, we'll just use the channel ID as the title.
	// A better solution would be to fetch the channel title from the YouTube API.
	_, err := db.AddSubscription(user.ID, channelID, channelID)
	if err != nil {
		http.Error(w, "Failed to add subscription", http.StatusInternalServerError)
		return
	}

	// After adding, return the updated list of subscriptions
	getSubscriptionsHandler(w, r)
}

func getSubscriptionsHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(middleware.UserContextKey).(*models.User)
	if !ok {
		http.Error(w, "User not found in context", http.StatusInternalServerError)
		return
	}

	subscriptions, err := db.GetSubscriptionsByUserID(user.ID)
	if err != nil {
		http.Error(w, "Failed to get subscriptions", http.StatusInternalServerError)
		return
	}

	fp := filepath.Join("web", "templates", "subscriptions.html")
	tmpl, err := template.ParseFiles(fp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, subscriptions); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
