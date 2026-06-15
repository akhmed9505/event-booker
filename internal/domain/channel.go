package domain

type NotificationChannel string

const (
	ChannelEmail    NotificationChannel = "email"
	ChannelTelegram NotificationChannel = "telegram"
	ChannelSMS      NotificationChannel = "sms"
)

func (c NotificationChannel) String() string {
	return string(c)
}
