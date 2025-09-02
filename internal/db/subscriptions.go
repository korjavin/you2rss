package db

import (
	"log"
	"yt-podcaster/internal/models"
)

func GetSubscriptionByID(id int) (models.Subscription, error) {
	subscription := models.Subscription{}
	err := DB.Get(&subscription, "SELECT * FROM subscriptions WHERE id = $1", id)
	return subscription, err
}

func GetSubscriptionsByUserID(userID int64) ([]models.Subscription, error) {
	query := `
		SELECT id, user_id, youtube_channel_id, youtube_channel_title, created_at
		FROM subscriptions
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	var subscriptions []models.Subscription
	err := DB.Select(&subscriptions, query, userID)
	if err != nil {
		log.Printf("Error getting subscriptions for user %d: %v", userID, err)
		return nil, err
	}
	return subscriptions, nil
}

func AddSubscription(userID int64, channelID string, channelTitle string) (*models.Subscription, error) {
	query := `
		INSERT INTO subscriptions (user_id, youtube_channel_id, youtube_channel_title)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, youtube_channel_id, youtube_channel_title, created_at
	`
	sub := &models.Subscription{}
	err := DB.Get(sub, query, userID, channelID, channelTitle)
	if err != nil {
		log.Printf("Error adding subscription for user %d: %v", userID, err)
		return nil, err
	}
	return sub, nil
}

func DeleteSubscription(userID int64, subscriptionID int) error {
	query := `
		DELETE FROM subscriptions
		WHERE id = $1 AND user_id = $2
	`
	_, err := DB.Exec(query, subscriptionID, userID)
	if err != nil {
		log.Printf("Error deleting subscription %d for user %d: %v", subscriptionID, userID, err)
		return err
	}
	return nil
}
