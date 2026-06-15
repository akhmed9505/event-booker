package eventbroker

import (
	"context"
	"github.com/akhmed9505/event-booker/internal/config"
	"github.com/akhmed9505/event-booker/internal/domain"

	"github.com/akhmed9505/event-booker/pkg/jwt"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/wb-go/wbf/ginext"
)

//go:generate mockgen -source=handlers.go -destination=mocks/mock.go
type EventHandler interface {
	Create(ctx context.Context, event *domain.Event) (uuid.UUID, error)
	ListEvents(ctx context.Context) ([]domain.Event, error)
	Get(ctx context.Context, id uuid.UUID) (*domain.Event, error)
	GetUserNameByID(ctx context.Context, userID uuid.UUID) (string, error)
	GetEmailByUserID(ctx context.Context, userID uuid.UUID) (string, error)
	UpdateUserEmail(ctx context.Context, userID uuid.UUID, email string) error
}

type BookingHandler interface {
	BookSeat(ctx context.Context, eventID uuid.UUID, userID uuid.UUID, seatID *int) (uuid.UUID, error)
	Confirm(ctx context.Context, bookingID uuid.UUID, userID uuid.UUID) error
	Cancel(ctx context.Context, bookingID uuid.UUID, userID uuid.UUID) error
	ListBookings(ctx context.Context, eventID uuid.UUID) ([]domain.Booking, error)
	CountBookedSeats(ctx context.Context, eventID uuid.UUID) (int, error)
	InsertBookingCancelledEvent(ctx context.Context, event *domain.BookingCancelledEvent) error
	GetContactByBookingID(ctx context.Context, bookingID uuid.UUID) (userEmail string, telegramChatID string, err error)
}

type Notifier interface {
	NotifyUserBookingCanceled(ctx context.Context, cfg *config.Config, userEmail, userTelegramID string, bookingID uuid.UUID) error
}

type Handler struct {
	logger         zerolog.Logger
	cfg            *config.Config
	EventHandler   EventHandler
	BookingHandler BookingHandler
	notifier       Notifier
}

func NewHandler(cfg *config.Config, logger zerolog.Logger, EventHandler EventHandler,
	BookingHandler BookingHandler, notifier Notifier) *Handler {
	return &Handler{
		cfg:            cfg,
		logger:         logger,
		EventHandler:   EventHandler,
		BookingHandler: BookingHandler,
		notifier:       notifier,
	}
}

// Создание события
func (h *Handler) Create(c *ginext.Context) {
	var req EventRequest
	if err := c.ShouldBind(&req); err != nil {
		h.logger.Error().Err(err).Msg("[Create] bind error")
		c.JSON(http.StatusBadRequest, ginext.H{"error": err.Error()})
		return
	}
	h.logger.Info().Interface("request", req).Msg("[Create] incoming request")

	event := &domain.Event{
		Name:            req.Name,
		Date:            req.Date,
		Email:           req.Email,
		TotalSeats:      req.TotalSeats,
		RequiresPayment: req.RequiresPayment,
		BookingTTL:      domain.PgDuration{Duration: time.Duration(req.BookingTTLMinutes) * time.Minute},
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}

	id, err := h.EventHandler.Create(c.Request.Context(), event)
	if err != nil {
		h.logger.Error().Err(err).Msg("[Create] service error")
		c.JSON(http.StatusInternalServerError, ginext.H{"error": "failed to create event"})
		return
	}

	c.Redirect(http.StatusSeeOther, fmt.Sprintf("/events/%s/book", id.String()))
}

// Получение события по ID в API (JSON)
func (h *Handler) Get(c *ginext.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.logger.Error().Err(err).Msg("[Get] invalid id")
		c.JSON(http.StatusBadRequest, ginext.H{"error": "invalid id"})
		return
	}
	h.logger.Info().Str("event_id", idStr).Msg("[Get] fetching event")

	event, err := h.EventHandler.Get(c.Request.Context(), id)
	if err != nil {
		h.logger.Error().Err(err).Msg("[Get] service error")
		c.JSON(http.StatusNotFound, ginext.H{"error": "event not found"})
		return
	}

	c.JSON(http.StatusOK, event)
}

// Бронирование места с SeatID из тела JSON
func (h *Handler) BookSeat(c *ginext.Context) {
	var req struct {
		SeatID int `form:"seat_id" binding:"required,min=1"`
	}

	userInfo, exists := jwt.GetUserInfoFromContext(c.Request.Context())
	if !exists {
		c.JSON(http.StatusUnauthorized, ginext.H{"error": "unauthorized"})
		return
	}
	userID := userInfo.UserID

	if err := c.ShouldBind(&req); err != nil {
		h.logger.Error().Err(err).Msg("[BookSeat] bind error")
		c.JSON(http.StatusBadRequest, ginext.H{"error": err.Error()})
		return
	}

	eventIDStr := c.Param("id")
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ginext.H{"error": "invalid event id"})
		return
	}

	_, err = h.BookingHandler.BookSeat(c.Request.Context(), eventID, userID, &req.SeatID)
	if err != nil {
		h.logger.Error().Err(err).Msg("[BookSeat] service error")
		c.JSON(http.StatusInternalServerError, ginext.H{"error": "failed to book seat"})
		return
	}

	c.Redirect(http.StatusSeeOther, fmt.Sprintf("/events/%s/bookings", eventID.String()))
}

// Подтверждение брони
func (h *Handler) Confirm(c *ginext.Context) {
	bookingIDStr := c.Param("booking_id")
	userInfo, exists := jwt.GetUserInfoFromContext(c.Request.Context())
	if !exists {
		c.JSON(http.StatusUnauthorized, ginext.H{"error": "unauthorized"})
		return
	}
	userID := userInfo.UserID

	bookingID, err := uuid.Parse(bookingIDStr)
	if err != nil {
		h.logger.Error().Err(err).Msg("[Confirm] invalid booking id")
		c.JSON(http.StatusBadRequest, ginext.H{"error": "invalid booking id"})
		return
	}
	h.logger.Info().Str("booking_id", bookingIDStr).Msg("[Confirm] booking confirmation requested")

	err = h.BookingHandler.Confirm(c.Request.Context(), bookingID, userID)
	if err != nil {
		h.logger.Error().Err(err).Msg("[Confirm] service error")
		c.JSON(http.StatusInternalServerError, ginext.H{"error": "failed to confirm booking"})
		return
	}

	c.Status(http.StatusNoContent)
}

// Отмена брони
func (h *Handler) Cancel(c *ginext.Context) {
	bookingIDStr := c.Param("booking_id")
	userInfo, exists := jwt.GetUserInfoFromContext(c.Request.Context())
	if !exists {
		c.JSON(http.StatusUnauthorized, ginext.H{"error": "unauthorized"})
		return
	}
	userID := userInfo.UserID

	bookingID, err := uuid.Parse(bookingIDStr)
	if err != nil {
		h.logger.Error().Err(err).Msg("[Cancel] invalid booking id")
		c.JSON(http.StatusBadRequest, ginext.H{"error": "invalid booking id"})
		return
	}
	h.logger.Info().Str("booking_id", bookingIDStr).Msg("[Cancel] cancel booking requested")

	err = h.BookingHandler.Cancel(c.Request.Context(), bookingID, userID)
	if err != nil {
		h.logger.Error().Err(err).Msg("[Cancel] service error")
		c.JSON(http.StatusInternalServerError, ginext.H{"error": "failed to cancel booking"})
		return
	}

	userEmail, telegramID, err := h.BookingHandler.GetContactByBookingID(c.Request.Context(), bookingID)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to retrieve contact info for cancelled booking")
	} else {
		if userEmail != "" || telegramID != "" {
			event := domain.BookingCancelledEvent{
				BookingID:      bookingID,
				UserID:         userID,
				UserEmail:      userEmail,
				TelegramChatID: telegramID,
				CancelledAt:    time.Now(),
			}

			err = h.BookingHandler.InsertBookingCancelledEvent(c.Request.Context(), &event)
			if err != nil {
				h.logger.Error().Err(err).Msg("Failed to insert booking cancelled event")
				c.JSON(http.StatusInternalServerError, ginext.H{"error": "failed to record cancellation"})
				return
			}

			if err := h.notifier.NotifyUserBookingCanceled(c.Request.Context(), h.cfg, event.UserEmail, telegramID, bookingID); err != nil {
				h.logger.Error().Err(err).Msg("Failed to notify user about cancelled booking")
				c.JSON(http.StatusInternalServerError, ginext.H{"error": "failed to notify user"})
				return
			}
		} else {
			h.logger.Warn().Msg("Both user email and telegramID are empty, skipping notifications")
		}
	}

	c.Status(http.StatusNoContent)
}

// Показ списка событий в HTML
func (h *Handler) ShowEventList(c *ginext.Context) {
	events, err := h.EventHandler.ListEvents(c.Request.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("[ShowEventList] failed to load events")
		c.JSON(http.StatusInternalServerError, ginext.H{"error": "failed to load events"})
		return
	}
	for i := range events {
		bookedCount, err := h.BookingHandler.CountBookedSeats(c.Request.Context(), events[i].ID)
		if err != nil {
			h.logger.Error().Err(err).Str("event_id", events[i].ID.String()).Msg("[ShowEventList] failed to count booked seats")
			// Можно решить, стоит ли прерывать или продолжить
		}
		events[i].AvailableSeats = events[i].TotalSeats - bookedCount
		if events[i].AvailableSeats < 0 {
			events[i].AvailableSeats = 0
		}
	}
	c.HTML(http.StatusOK, "eventList.html", ginext.H{"Events": events})
}

func (h *Handler) ShowBookingList(c *ginext.Context) {
	eventIDStr := c.Param("id")
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		h.logger.Error().Err(err).Msg("[ShowBookingList] invalid event id")
		c.JSON(http.StatusBadRequest, ginext.H{"error": "invalid event id"})
		return
	}

	bookings, err := h.BookingHandler.ListBookings(c.Request.Context(), eventID)
	if err != nil {
		h.logger.Error().Err(err).Msg("[ShowBookingList] failed to load bookings")
		c.JSON(http.StatusInternalServerError, ginext.H{"error": "failed to load bookings"})
		return
	}

	type BookingWithUserName struct {
		Booking  domain.Booking
		UserName string
	}

	var bookingsWithUsers []BookingWithUserName
	for _, b := range bookings {
		userName, err := h.EventHandler.GetUserNameByID(c.Request.Context(), *b.UserID)
		if err != nil {
			h.logger.Warn().Err(err).Str("user_id", b.UserID.String()).Msg("unknown user for booking")
			userName = "неизвестный пользователь"
		}
		bookingsWithUsers = append(bookingsWithUsers, BookingWithUserName{
			Booking:  b,
			UserName: userName,
		})
	}

	c.HTML(http.StatusOK, "bookingList.html", ginext.H{"Bookings": bookingsWithUsers, "EventID": eventID})
}

// Показ деталей события в HTML
func (h *Handler) ShowEventDetail(c *ginext.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.logger.Error().Err(err).Msg("[ShowEventDetail] invalid id")
		c.JSON(http.StatusBadRequest, ginext.H{"error": "invalid id"})
		return
	}
	event, err := h.EventHandler.Get(c.Request.Context(), id)
	if err != nil {
		h.logger.Error().Err(err).Msg("[ShowEventDetail] event not found")
		c.JSON(http.StatusNotFound, ginext.H{"error": "event not found"})
		return
	}

	bookedCount, err := h.BookingHandler.CountBookedSeats(c.Request.Context(), event.ID)
	if err != nil {
		h.logger.Error().Err(err).Str("event_id", event.ID.String()).Msg("[ShowEventDetail] failed to count booked seats")
		// Не прерываем, продолжаем с AvailableSeats
	}
	event.AvailableSeats = event.TotalSeats - bookedCount
	if event.AvailableSeats < 0 {
		event.AvailableSeats = 0
	}

	c.HTML(http.StatusOK, "eventBook.html", event)
}

// Показ формы создания события в HTML
func (h *Handler) ShowEvent(c *ginext.Context) {
	c.HTML(http.StatusOK, "eventCreate.html", nil)
}

// Показ формы профиля (отображение email)
func (h *Handler) ShowProfile(c *ginext.Context) {
	userInfo, exists := jwt.GetUserInfoFromContext(c.Request.Context())
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}
	userID := userInfo.UserID

	email, err := h.EventHandler.GetEmailByUserID(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error().Err(err).Str("user_id", userID.String()).Msg("[ShowProfile] failed to get email")
		email = ""
	}

	c.HTML(http.StatusOK, "profile.html", ginext.H{
		"Email": email,
	})
}

// Обновление email в профиле
func (h *Handler) UpdateProfile(c *ginext.Context) {
	userInfo, exists := jwt.GetUserInfoFromContext(c.Request.Context())
	if !exists {
		c.JSON(http.StatusUnauthorized, ginext.H{"error": "unauthorized"})
		return
	}
	userID := userInfo.UserID

	email := c.PostForm("email")
	if email == "" {
		c.HTML(http.StatusBadRequest, "profile.html", ginext.H{
			"Error": "Email не может быть пустым",
			"Email": email,
		})
		return
	}

	err := h.EventHandler.UpdateUserEmail(c.Request.Context(), userID, email)
	if err != nil {
		h.logger.Error().Err(err).Str("user_id", userID.String()).Msg("[UpdateProfile] failed to update email")
		c.HTML(http.StatusInternalServerError, "profile.html", ginext.H{
			"Error": "Не удалось сохранить email",
			"Email": email,
		})
		return
	}

	h.logger.Info().Str("user_id", userID.String()).Msg("[UpdateProfile] email updated successfully")

	c.HTML(http.StatusOK, "profile.html", ginext.H{
		"Message": "Email успешно сохранен",
		"Email":   email,
	})
}

func (h *Handler) ShowBookingForm(c *ginext.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.logger.Error().Err(err).Str("event_id", idStr).Msg("[ShowBookingForm] invalid event id")
		c.JSON(http.StatusBadRequest, ginext.H{"error": "invalid event id"})
		return
	}

	event, err := h.EventHandler.Get(c.Request.Context(), id)
	if err != nil {
		h.logger.Error().Err(err).Str("event_id", idStr).Msg("[ShowBookingForm] event not found")
		c.JSON(http.StatusNotFound, ginext.H{"error": "event not found"})
		return
	}

	c.HTML(http.StatusOK, "bookingForm.html", ginext.H{"Event": event})
}
