package handlers

import (
	"html/template"
	"log"
	"net/http"
	"yt-podcaster/pkg/tasks"
)

type Handlers struct {
	templates        *template.Template
	asynqClient      tasks.TaskEnqueuer
	audioStoragePath string
}

func New(templates *template.Template, asynqClient tasks.TaskEnqueuer, audioStoragePath string) *Handlers {
	return &Handlers{
		templates:        templates,
		asynqClient:      asynqClient,
		audioStoragePath: audioStoragePath,
	}
}

func (h *Handlers) ServeWebApp(w http.ResponseWriter, r *http.Request) {
	err := h.templates.ExecuteTemplate(w, "index.html", nil)
	if err != nil {
		log.Printf("Template execution error: %v", err)
		// Try to serve the fallback template
		if fallbackErr := h.templates.ExecuteTemplate(w, "fallback", nil); fallbackErr != nil {
			// If even the fallback fails, serve a basic error page
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`
				<html><body>
					<h1>Service Temporarily Unavailable</h1>
					<p>Template execution error. Please check the server logs.</p>
				</body></html>
			`))
		}
	}
}

func (h *Handlers) PostAuth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "authenticated"}`))
}
