package events

import (
	"context"
	"github.com/akhmed9505/event-booker/internal/config"
	"github.com/akhmed9505/event-booker/internal/domain"
	"github.com/akhmed9505/event-booker/internal/service/notificationService"
	"fmt"

	"github.com/google/uuid"
)

//go:generate mockgen -source=events.go -destination=mocks/mock.go
type EventService interface {
	Create(ctx context.Context, event *domain.Event) (uuid.UUID, error)
	Get(ctx context.Context, id uuid.UUID) (*domain.Event, error)
	ListEvents(ctx context.Context) ([]domain.Event, error)
	GetUserNameByID(ctx context.Context, userID uuid.UUID) (string, error)
	GetEmailByUserID(ctx context.Context, userID uuid.UUID) (string, error)
	UpdateUserEmail(ctx context.Context, userID uuid.UUID, email string) error
}

type BookingService interface {
	BookSeat(ctx context.Context, eventID uuid.UUID, userID uuid.UUID, seatID *int) (uuid.UUID, error)
	Confirm(ctx context.Context, bookingID uuid.UUID, userID uuid.UUID) error
	Cancel(ctx context.Context, bookingID uuid.UUID, userID uuid.UUID) error
	ListBookings(ctx context.Context, eventID uuid.UUID) ([]domain.Booking, error)
	StartExpiredBookingCleanerCron(ctx context.Context, schedule string) error
	CountBookedSeats(ctx context.Context, eventID uuid.UUID) (int, error)
	InsertBookingCancelledEvent(ctx context.Context, event *domain.BookingCancelledEvent) error
	GetContactByBookingID(ctx context.Context, bookingID uuid.UUID) (userEmail string, telegramChatID string, err error)
}

type Notifier interface {
	NotifyUserBookingCanceled(ctx context.Context, cfg *config.Config, userEmail, userTelegramID string, bookingID uuid.UUID) error
}
type Service struct {
	eventService   EventService
	bookingService BookingService
	notifier       Notifier
}

func NewService(eventService EventService, bookingService BookingService, notifier Notifier) *Service {
	return &Service{
		eventService:   eventService,
		bookingService: bookingService,
		notifier:       notifier,
	}
}

func (s *Service) GetUserNameByID(ctx context.Context, userID uuid.UUID) (string, error) {
	return s.eventService.GetUserNameByID(ctx, userID)
}
func (s *Service) Create(ctx context.Context, event *domain.Event) (uuid.UUID, error) {
	return s.eventService.Create(ctx, event)
}

func (s *Service) Cancel(ctx context.Context, bookingID uuid.UUID, userID uuid.UUID) error {
	return s.bookingService.Cancel(ctx, bookingID, userID)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*domain.Event, error) {
	return s.eventService.Get(ctx, id)
}

func (s *Service) BookSeat(ctx context.Context, eventID uuid.UUID, userID uuid.UUID, seatID *int) (uuid.UUID, error) {
	return s.bookingService.BookSeat(ctx, eventID, userID, seatID)
}
func (s *Service) Confirm(ctx context.Context, bookingID uuid.UUID, userID uuid.UUID) error {
	return s.bookingService.Confirm(ctx, bookingID, userID)
}

func (s *Service) ListBookings(ctx context.Context, eventID uuid.UUID) ([]domain.Booking, error) {
	return s.bookingService.ListBookings(ctx, eventID)
}

func (s *Service) StartExpiredBookingCleanerCron(ctx context.Context, schedule string) error {
	return s.bookingService.StartExpiredBookingCleanerCron(ctx, schedule)
}

func (s *Service) ListEvents(ctx context.Context) ([]domain.Event, error) {
	return s.eventService.ListEvents(ctx)
}

func (s *Service) CountBookedSeats(ctx context.Context, eventID uuid.UUID) (int, error) {
	return s.bookingService.CountBookedSeats(ctx, eventID)
}

func (s *Service) InsertBookingCancelledEvent(ctx context.Context, event *domain.BookingCancelledEvent) error {
	return s.bookingService.InsertBookingCancelledEvent(ctx, event)
}

func (s *Service) GetContactByBookingID(ctx context.Context, bookingID uuid.UUID) (userEmail string, telegramChatID string, err error) {
	return s.bookingService.GetContactByBookingID(ctx, bookingID)
}

func (s *Service) GetEmailByUserID(ctx context.Context, userID uuid.UUID) (string, error) {
	return s.eventService.GetEmailByUserID(ctx, userID)
}
func (s *Service) UpdateUserEmail(ctx context.Context, userID uuid.UUID, email string) error {
	return s.eventService.UpdateUserEmail(ctx, userID, email)
}

func (s *Service) NotifyUserBookingCanceled(ctx context.Context, cfg *config.Config, userEmail, userTelegramID string, bookingID uuid.UUID) error {
	emailSender := notificationService.NewEmailChannel(
		cfg.EmailSmpt.SmptPort,
		cfg.EmailSmpt.SmptServer,
		cfg.EmailSmpt.SmptEmail,
		cfg.EmailSmpt.SmptPassword,
	)

	telegramBot, err := notificationService.NewTelegramChannel(cfg.TelegBot.Key)
	if err != nil {
		fmt.Printf("NotifyUserBookingCanceled: Failed to init Telegram channel: %v\n", err)
		return fmt.Errorf("failed to init telegram channel: %w", err)
	}

	err1 := emailSender.Send(ctx, userEmail, fmt.Sprintf("Ваше бронирование %s было отменено", bookingID))

	var err2 error = nil
	if userTelegramID != "" {
		err2 = telegramBot.Send(ctx, userTelegramID, fmt.Sprintf("Ваше бронирование %s было отменено", bookingID))
		if err2 != nil {
			fmt.Printf("NotifyUserBookingCanceled: Error sending Telegram message: %v\n", err2)
		} else {
			fmt.Printf("NotifyUserBookingCanceled: Telegram message sent successfully\n")
		}
	} else {
		fmt.Println("NotifyUserBookingCanceled: Telegram chat ID пуст, пропускаем отправку сообщения")
	}

	if err1 != nil || err2 != nil {
		return fmt.Errorf("errors sending notifications: email: %v, telegram: %v", err1, err2)
	}

	return nil
}
