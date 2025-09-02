package feed

import (
	"fmt"
	"net/http"
	"time"

	"github.com/eduncan911/podcast"
	"yt-podcaster/internal/models"
)

func GenerateRSS(user *models.User, episodes []models.Episode, r *http.Request) (string, error) {
	p := podcast.New(
		fmt.Sprintf("%s's Podcast", user.TelegramUsername),
		fmt.Sprintf("%s://%s/rss/%s", r.URL.Scheme, r.URL.Host, user.RSSUUID),
		"A podcast generated from YouTube channels.",
		&time.Time{}, &time.Time{},
	)

	for _, episode := range episodes {
		item := podcast.Item{
			Title:       *episode.Title,
			Description: *episode.Description,
			PubDate:     episode.PublishedAt,
		}
		item.AddEnclosure(fmt.Sprintf("%s://%s/audio/%s.m4a", r.URL.Scheme, r.URL.Host, episode.AudioUUID), podcast.M4A, *episode.AudioSizeBytes)
		if _, err := p.AddItem(item); err != nil {
			return "", err
		}
	}

	return p.String(), nil
}
