package domain

import (
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID          uuid.UUID           `json:"id"`          // уникальный ID
	Message     string              `json:"message"`     // текст уведомления
	Destination string              `json:"destination"` // куда отправлять (email, telegram username, телефон)
	Channel     NotificationChannel `json:"channel"`     // через какой канал
	Status      NotificationStatus  `json:"status"`      // текущий статус
	DataToSent  time.Time           `json:"data_sent_at"`
	CreatedAt   time.Time           `json:"created_at"`
}

type StatusResponse struct {
	NoteID uuid.UUID `json:"note_id"`
	Status string    `json:"status"`
}
type CancelResponse struct {
	NoteID  uuid.UUID `json:"note_id"`
	Message string    `json:"message"`
}

// RetryPolicy описывает стратегию повторных попыток с экспоненциальной задержкой
type RetryPolicy struct {
	MaxAttempts   int           `json:"max_attempts"`   // максимальное число попыток
	InitialDelay  time.Duration `json:"initial_delay"`  // первая задержка
	MaxDelay      time.Duration `json:"max_delay"`      // максимальная задержка
	BackoffFactor float64       `json:"backoff_factor"` // множитель задержки
}

// NotificationChannelSender интерфейс для отправки уведомлений через разные каналы
type NotificationChannelSender interface {
	Send(message, id int64, destination string) error
}
