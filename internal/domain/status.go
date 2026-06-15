package domain

type NotificationStatus string

const (
	StatusCreated  NotificationStatus = "created"
	StatusPending  NotificationStatus = "pending"
	StatusSent     NotificationStatus = "sent"
	StatusFailed   NotificationStatus = "failed"
	StatusCanceled NotificationStatus = "canceled"
)
