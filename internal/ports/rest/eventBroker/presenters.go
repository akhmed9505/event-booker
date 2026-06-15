package eventbroker

import (
	"github.com/akhmed9505/event-booker/internal/domain"
	"time"
)

type SeatBookingRequest struct {
	SeatID *int `form:"seat_id" json:"seat_id" binding:"required"`
}
type EventRequest struct {
	Name              string            `form:"name" json:"name" binding:"required"`
	Email             string            `form:"email" json:"email"` // если поддерживать пользователей
	Date              time.Time         `form:"date" json:"date" binding:"required" time_format:"2006-01-02T15:04"`
	TotalSeats        int               `form:"total_seats" json:"total_seats" binding:"required,min=1"`
	RequiresPayment   bool              `form:"requires_payment" json:"requires_payment"`
	BookingTTL        domain.PgDuration `json:"booking_ttl"`                                  // для JSON API
	BookingTTLMinutes int               `form:"booking_ttl_minutes" binding:"required,min=1"` // для формы
}
