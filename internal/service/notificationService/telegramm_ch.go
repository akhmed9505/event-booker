package notificationService

import (
	"context"
	"fmt"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramChannel struct {
	bot *tgbotapi.BotAPI
}

func NewTelegramChannel(botToken string) (*TelegramChannel, error) {
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %w", err)
	}
	return &TelegramChannel{bot: bot}, nil
}

func (t *TelegramChannel) Send(ctx context.Context, destination, text string) error {
	chatID, _ := strconv.ParseInt(destination, 10, 64)
	msg := tgbotapi.NewMessage(chatID, text)

	done := make(chan error, 1)
	go func() {
		_, err := t.bot.Send(msg)
		done <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}
