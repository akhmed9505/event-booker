package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID              uuid.UUID  `json:"id"`
	Name            string     `json:"name"`
	Date            time.Time  `json:"date"`
	Email           string     `json:"email"` // если поддерживать пользователей
	TotalSeats      int        `json:"total_seats"`
	AvailableSeats  int        `json:"available_seats"`
	RequiresPayment bool       `json:"requires_payment"`
	BookingTTL      PgDuration `json:"booking_ttl"` // Время жизни брони, если нужно разное для мероприятий
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type BookingStatus string

const (
	BookingPending   BookingStatus = "pending"   // забронировано, не подтверждено
	BookingConfirmed BookingStatus = "confirmed" // оплачено / подтверждено
	BookingCanceled  BookingStatus = "canceled"  // отменено
)

type Booking struct {
	ID         uuid.UUID     `json:"id"`
	EventID    uuid.UUID     `json:"event_id"`
	UserID     *uuid.UUID    `json:"user_id,omitempty"`
	Email      string        `json:"email"` // если поддерживать пользователей
	SeatNumber int           `json:"seat_number,omitempty"`
	Status     BookingStatus `json:"status"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
	PaidAt     *time.Time    `json:"paid_at,omitempty"` // время оплаты, если оплачено
}

type BookingCancelledEvent struct {
	BookingID      uuid.UUID
	UserID         uuid.UUID
	UserEmail      string
	TelegramChatID string
	CancelledAt    time.Time
}

type PgDuration struct {
	time.Duration
}

func (d *PgDuration) Scan(src interface{}) error {
	switch v := src.(type) {
	case []byte:
		str := string(v)
		// PostgreSQL interval приходит в виде "00:30:00"
		parsed, err := time.ParseDuration(parsePgIntervalToDuration(str))
		if err != nil {
			return err
		}
		d.Duration = parsed
		return nil
	case string:
		parsed, err := time.ParseDuration(parsePgIntervalToDuration(v))
		if err != nil {
			return err
		}
		d.Duration = parsed
		return nil
	default:
		return fmt.Errorf("cannot scan %T into PgDuration", src)
	}
}

// parsePgIntervalToDuration преобразует "01:30:00" в "1h30m0s"
func parsePgIntervalToDuration(s string) string {
	parts := strings.Split(s, ":")
	if len(parts) == 3 {
		return fmt.Sprintf("%sh%sm%ss", parts[0], parts[1], parts[2])
	}
	return s
}

func (d PgDuration) Value() (driver.Value, error) {
	// Преобразовываем time.Duration в строку формата PostgreSQL interval, например: "1h30m0s" -> "01:30:00"
	h := int64(d.Hours())
	m := int64(d.Minutes()) % 60
	s := int64(d.Seconds()) % 60
	intervalStr := fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	return intervalStr, nil
}

func (d PgDuration) MarshalJSON() ([]byte, error) {
	s := d.Duration.String() // например "20m0s"
	// Можно убрать лишние "0s" для аккуратности
	s = strings.TrimSuffix(s, "0s")
	return json.Marshal(s)
}

// unmarshal из строки JSON вида "20m" в PgDuration
func (d *PgDuration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = dur
	return nil
}

type BookingWithUserName struct {
	Booking  Booking
	UserName string
}
