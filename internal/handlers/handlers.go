package handlers

import (
	"html/template"
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
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *Handlers) PostAuth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "authenticated"}`))
}
