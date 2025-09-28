package feed

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"yt-podcaster/internal/models"

	"github.com/eduncan911/podcast"
)

func getBaseURL(r *http.Request) string {
	if baseURL := os.Getenv("BASE_URL"); baseURL != "" {
		return baseURL
	}

	scheme := r.URL.Scheme
	if scheme == "" {
		scheme = "https"
		if r.Header.Get("X-Forwarded-Proto") != "" {
			scheme = r.Header.Get("X-Forwarded-Proto")
		}
	}

	return fmt.Sprintf("%s://%s", scheme, r.Host)
}

func GenerateRSS(user *models.User, episodes []models.Episode, r *http.Request) (string, error) {
	baseURL := getBaseURL(r)

	p := podcast.New(
		fmt.Sprintf("%s's Podcast", user.TelegramUsername),
		fmt.Sprintf("%s/rss/%s", baseURL, user.RSSUUID),
		"A podcast generated from YouTube channels.",
		&time.Time{}, &time.Time{},
	)

	for _, episode := range episodes {
		item := podcast.Item{
			Title:       *episode.Title,
			Description: *episode.Description,
			PubDate:     episode.PublishedAt,
		}
		item.AddEnclosure(fmt.Sprintf("%s/audio/%s.m4a", baseURL, episode.AudioUUID), podcast.M4A, *episode.AudioSizeBytes)
		if _, err := p.AddItem(item); err != nil {
			return "", err
		}
	}

	return p.String(), nil
}

func GenerateSubscriptionRSS(subscription *models.Subscription, episodes []models.Episode, r *http.Request) (string, error) {
	baseURL := getBaseURL(r)

	p := podcast.New(
		subscription.YoutubeChannelTitle,
		fmt.Sprintf("%s/rss/%s", baseURL, subscription.RSSUUID),
		fmt.Sprintf("Podcast feed for YouTube channel: %s", subscription.YoutubeChannelTitle),
		&time.Time{}, &time.Time{},
	)

	for _, episode := range episodes {
		item := podcast.Item{
			Title:       *episode.Title,
			Description: *episode.Description,
			PubDate:     episode.PublishedAt,
		}
		item.AddEnclosure(fmt.Sprintf("%s/audio/%s.m4a", baseURL, episode.AudioUUID), podcast.M4A, *episode.AudioSizeBytes)
		if _, err := p.AddItem(item); err != nil {
			return "", err
		}
	}

	return p.String(), nil
}
