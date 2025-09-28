package handlers

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"yt-podcaster/internal/db"
	"yt-podcaster/pkg/tasks"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *Handlers) StartTelegramBot() {
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil { // ignore any non-Message updates
			continue
		}

		if !update.Message.IsCommand() { // if you got a message that is not a command
			h.handleTelegramMessage(bot, update.Message)
			continue
		}

		// Extract the command from the Message.
		switch update.Message.Command() {
		case "list":
			h.handleListCommand(bot, update.Message)
		default:
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "I don't know that command")
			bot.Send(msg)
		}
	}
}

func (h *Handlers) handleTelegramMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	log.Printf("[%s] %s", message.From.UserName, message.Text)

	user, err := db.FindOrCreateUserByTelegramID(message.From.ID, message.From.UserName)
	if err != nil {
		log.Printf("Error finding or creating user: %v", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Error creating user.")
		bot.Send(msg)
		return
	}

	channelURL := message.Text
	if !validateYouTubeURL(channelURL) {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Invalid YouTube URL format")
		bot.Send(msg)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), getChannelInfoTimeout())
	defer cancel()

	log.Printf("Extracting channel info from URL: %s", channelURL)
	channelID, channelTitle, err := extractChannelInfo(ctx, channelURL)
	if err != nil {
		log.Printf("Error extracting channel info from URL '%s': %v", channelURL, err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Could not extract channel info from URL")
		bot.Send(msg)
		return
	}

	if channelID == "" || channelTitle == "" || channelTitle == "NA" {
		log.Printf("Could not extract valid channel info for URL '%s': ID='%s', Title='%s'", channelURL, channelID, channelTitle)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Could not extract valid channel info from URL")
		bot.Send(msg)
		return
	}

	sub, err := db.AddSubscription(user.ID, channelID, channelTitle)
	if err != nil {
		log.Printf("Error creating subscription: %v", err)
		if strings.Contains(err.Error(), "subscriptions_user_id_youtube_channel_id_key") {
			msg := tgbotapi.NewMessage(message.Chat.ID, "You are already subscribed to this channel.")
			bot.Send(msg)
			return
		}
		msg := tgbotapi.NewMessage(message.Chat.ID, "Internal server error")
		bot.Send(msg)
		return
	}

	task, err := tasks.NewCheckChannelTask(sub.ID)
	if err != nil {
		log.Printf("Error creating task: %v", err)
	} else {
		_, err = h.asynqClient.Enqueue(task)
		if err != nil {
			log.Printf("Error enqueuing task: %v", err)
		}
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "Subscription added successfully!")
	bot.Send(msg)
}

func (h *Handlers) handleListCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	log.Printf("[%s] %s", message.From.UserName, message.Text)

	user, err := db.FindOrCreateUserByTelegramID(message.From.ID, message.From.UserName)
	if err != nil {
		log.Printf("Error finding or creating user: %v", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Error creating user.")
		bot.Send(msg)
		return
	}

	subscriptions, err := db.GetSubscriptionsByUserID(user.ID)
	if err != nil {
		log.Printf("Error getting subscriptions: %v", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Internal server error")
		bot.Send(msg)
		return
	}

	if len(subscriptions) == 0 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "You have no subscriptions.")
		bot.Send(msg)
		return
	}

	var response string
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	for _, sub := range subscriptions {
		response += fmt.Sprintf("<b>%s</b>: %s/rss/%s\n", sub.YoutubeChannelTitle, baseURL, sub.RSSUUID)
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, response)
	msg.ParseMode = "HTML"
	bot.Send(msg)
}
