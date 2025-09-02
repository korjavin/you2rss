package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
	"yt-podcaster/internal/db"
	"yt-podcaster/internal/handlers"
	"yt-podcaster/internal/middleware"
	"yt-podcaster/internal/test"
	"yt-podcaster/pkg/tasks"
)

type App struct {
	router      *mux.Router
	templates   *template.Template
	asynqClient tasks.TaskEnqueuer
}

var audioStoragePath string

func NewApp(enqueuer tasks.TaskEnqueuer) *App {
	app := &App{
		router:      mux.NewRouter(),
		asynqClient: enqueuer,
	}

	if app.asynqClient == nil {
		redisAddr := os.Getenv("REDIS_ADDR")
		if redisAddr == "" {
			redisAddr = "127.0.0.1:6379"
		}
		app.asynqClient = asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
	}

	// Load templates
	templatesPath := filepath.Join(test.ProjectRoot(), "web", "templates", "*.html")
	app.templates = template.Must(template.ParseGlob(templatesPath))

	// Set audio storage path
	audioStoragePath = os.Getenv("AUDIO_STORAGE_PATH")
	if audioStoragePath == "" {
		audioStoragePath = "audio"
	}

	app.registerHandlers()

	return app
}

func (a *App) registerHandlers() {
	// Create handlers
	h := handlers.New(a.templates, a.asynqClient, audioStoragePath)

	// Public handlers
	a.router.HandleFunc("/rss/{uuid}", h.GetRSSFeed).Methods("GET")
	a.router.HandleFunc("/audio/{filename:.+}", h.ServeAudioFile).Methods("GET")

	// Authenticated handlers
	a.router.Handle("/", middleware.AuthMiddleware(http.HandlerFunc(h.GetRoot))).Methods("GET")
	a.router.Handle("/subscriptions", middleware.AuthMiddleware(http.HandlerFunc(h.GetSubscriptions))).Methods("GET")
	a.router.Handle("/subscriptions", middleware.AuthMiddleware(http.HandlerFunc(h.PostSubscription))).Methods("POST")
	a.router.Handle("/subscriptions/{id}", middleware.AuthMiddleware(http.HandlerFunc(h.DeleteSubscription))).Methods("DELETE")
}

func (a *App) Serve() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server listening on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, a.router))
}

func main() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found")
	}

	// Initialize database
	db.InitDB()

	app := NewApp(nil)
	app.Serve()
}
