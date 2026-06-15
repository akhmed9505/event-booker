package notificationService

import "context"

type NotificationChannel interface {
	Send(ctx context.Context, destination, text string) error
}
