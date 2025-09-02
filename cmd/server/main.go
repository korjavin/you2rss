package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"text/template"

	"github.com/joho/godotenv"
	"yt-podcaster/internal/db"
	"yt-podcaster/internal/middleware"
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

	log.Printf("Starting server on :%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
