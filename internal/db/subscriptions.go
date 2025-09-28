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
		SELECT id, user_id, youtube_channel_id, youtube_channel_title, rss_uuid, created_at
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

func CountSubscriptionsByUserID(userID int64) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM subscriptions WHERE user_id = $1"
	err := DB.Get(&count, query, userID)
	if err != nil {
		log.Printf("Error counting subscriptions for user %d: %v", userID, err)
		return 0, err
	}
	return count, nil
}

func AddSubscription(userID int64, channelID string, channelTitle string) (*models.Subscription, error) {
	query := `
		INSERT INTO subscriptions (user_id, youtube_channel_id, youtube_channel_title)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, youtube_channel_id, youtube_channel_title, rss_uuid, created_at
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

func GetSubscriptionByRSSUUID(rssUUID string) (models.Subscription, error) {
	subscription := models.Subscription{}
	query := `
		SELECT id, user_id, youtube_channel_id, youtube_channel_title, rss_uuid, created_at
		FROM subscriptions
		WHERE rss_uuid = $1
	`
	err := DB.Get(&subscription, query, rssUUID)
	if err != nil {
		log.Printf("Error getting subscription by rss_uuid: %v", err)
	}
	return subscription, err
}

func GetAllSubscriptions() ([]models.Subscription, error) {
	query := `
		SELECT id, user_id, youtube_channel_id, youtube_channel_title, rss_uuid, created_at
		FROM subscriptions
		ORDER BY created_at DESC
	`
	var subscriptions []models.Subscription
	err := DB.Select(&subscriptions, query)
	if err != nil {
		log.Printf("Error getting all subscriptions: %v", err)
		return nil, err
	}
	return subscriptions, nil
}

func IsNewChannel(subscriptionID int) (bool, error) {
	var count int
	query := "SELECT COUNT(*) FROM episodes WHERE subscription_id = $1"
	err := DB.Get(&count, query, subscriptionID)
	if err != nil {
		log.Printf("Error counting episodes for subscription %d: %v", subscriptionID, err)
		return false, err
	}
	return count == 0, nil
}
