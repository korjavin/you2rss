package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
	"golang.org/x/time/rate"
	"yt-podcaster/internal/db"
	"yt-podcaster/internal/handlers"
	"yt-podcaster/internal/middleware"
	"yt-podcaster/internal/test"
	"yt-podcaster/pkg/tasks"
)

// CommitSHA is set at build time via ldflags
var CommitSHA = "unknown"

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

	// Load templates with error handling instead of panicking
	templatesPath := filepath.Join(test.ProjectRoot(), "web", "templates", "*.html")
	templates, err := template.ParseGlob(templatesPath)
	if err != nil {
		log.Printf("Warning: Template parsing error: %v", err)
		log.Printf("Creating minimal fallback template...")
		// Create a minimal fallback template that won't crash
		app.templates = template.Must(template.New("fallback").Parse(`
			<html><body>
				<h1>Service Temporarily Unavailable</h1>
				<p>Template parsing error. Please check the logs and fix the templates.</p>
			</body></html>
		`))
	} else {
		app.templates = templates
	}

	// Set audio storage path
	audioStoragePath = os.Getenv("AUDIO_STORAGE_PATH")
	if audioStoragePath == "" {
		audioStoragePath = "audio"
	}

	// Create audio storage directory if it doesn't exist
	if err := os.MkdirAll(audioStoragePath, 0755); err != nil {
		log.Fatalf("Failed to create audio storage directory: %v", err)
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

	// Create rate limiter with configurable values
	rateLimitPerMinute := 100.0 // default
	if env := os.Getenv("RATE_LIMIT_PER_MINUTE"); env != "" {
		if val, err := strconv.ParseFloat(env, 64); err == nil {
			rateLimitPerMinute = val
		}
	}

	rateLimitBurst := 5 // default
	if env := os.Getenv("RATE_LIMIT_BURST"); env != "" {
		if val, err := strconv.Atoi(env); err == nil {
			rateLimitBurst = val
		}
	}

	rateLimiter := middleware.NewRateLimiterMiddleware(rate.Limit(rateLimitPerMinute/60.0), rateLimitBurst)

	// Authenticated handlers
	authMiddleware := func(next http.Handler) http.Handler {
		return middleware.AuthMiddleware(rateLimiter.Middleware(next))
	}

	a.router.Handle("/", http.HandlerFunc(h.ServeWebApp)).Methods("GET")
	a.router.Handle("/auth", authMiddleware(http.HandlerFunc(h.PostAuth))).Methods("POST")
	a.router.Handle("/subscriptions", authMiddleware(http.HandlerFunc(h.GetSubscriptions))).Methods("GET")
	a.router.Handle("/subscriptions", authMiddleware(http.HandlerFunc(h.PostSubscription))).Methods("POST")
	a.router.Handle("/subscriptions/{id}", authMiddleware(http.HandlerFunc(h.DeleteSubscription))).Methods("DELETE")
}

func (a *App) Serve() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server listening on port %s (commit: %s)", port, CommitSHA)
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
