package handlers

import (
	"html/template"
	"yt-podcaster/pkg/tasks"
)

type Handlers struct {
	templates      *template.Template
	asynqClient    tasks.TaskEnqueuer
	audioStoragePath string
}

func New(templates *template.Template, asynqClient tasks.TaskEnqueuer, audioStoragePath string) *Handlers {
	return &Handlers{
		templates:      templates,
		asynqClient:    asynqClient,
		audioStoragePath: audioStoragePath,
	}
}
