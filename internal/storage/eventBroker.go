package storage

import (
	"context"
	"database/sql"
	"errors"
	"github.com/akhmed9505/event-booker/internal/domain"
	"github.com/akhmed9505/event-booker/pkg/e"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

type EventService interface {
	Create(ctx context.Context, event *domain.Event) (uuid.UUID, error)
	Get(ctx context.Context, id uuid.UUID) (*domain.Event, error)
	ListEvents(ctx context.Context) ([]domain.Booking, error)
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

func (p *Postgres) GetUserNameByID(ctx context.Context, userID uuid.UUID) (string, error) {
	var userName string
	query := `SELECT nickname FROM users WHERE id = $1`
	err := p.db.Master.QueryRowContext(ctx, query, userID).Scan(&userName)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("user not found")
		}
		return "", fmt.Errorf("query error: %w", err)
	}
	return userName, nil
}

func (p *Postgres) Create(ctx context.Context, event *domain.Event) (uuid.UUID, error) {
	var id uuid.UUID

	query := `INSERT INTO events(name, date, email, total_seats, requires_payment, booking_ttl)
              VALUES($1,$2,$3,$4,$5,$6) RETURNING id`
	err := p.db.Master.QueryRowContext(ctx, query,
		event.Name,
		event.Date,
		event.Email,
		event.TotalSeats,
		event.RequiresPayment,
		event.BookingTTL,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, e.Wrap("storage/eventBroker/Create", err)
	}
	return id, nil
}

// Cancel отменяет (помечает как deleted) бронирование с указанным id

// Get возвращает событие по ID
func (p *Postgres) Get(ctx context.Context, id uuid.UUID) (*domain.Event, error) {
	var event domain.Event

	query := `SELECT id, name, date, email, total_seats, requires_payment, booking_ttl FROM events WHERE id = $1`
	err := p.db.Master.QueryRowContext(ctx, query, id).Scan(
		&event.ID,
		&event.Name,
		&event.Date,
		&event.Email,
		&event.TotalSeats,
		&event.RequiresPayment,
		&event.BookingTTL,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, e.ErrNotFound
		}
		return nil, e.Wrap("storage/eventBroker/Get", err)
	}

	return &event, nil
}

func (p *Postgres) Cancel(ctx context.Context, bookingID uuid.UUID, userID uuid.UUID) error {
	// Добавляем проверку user_id = $2, чтобы пользователь мог изменять только свои брони
	query := `UPDATE bookings 
          SET status = 'canceled', updated_at = NOW()
          WHERE id = $1 AND user_id = $2 AND status = 'pending'`

	result, err := p.db.Master.ExecContext(ctx, query, bookingID, userID)
	if err != nil {
		return e.Wrap("storage/eventBroker/Cancel", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get number of affected rows: %w", err)
	}

	if rowsAffected == 0 {
		return e.ErrNotFound
	}

	return nil
}

func (p *Postgres) Confirm(ctx context.Context, bookingID uuid.UUID, userID uuid.UUID) error {
	// Аналогично, проверяем, что бронирование принадлежит userID и статус pending
	query := `UPDATE bookings
              SET status = 'confirmed', paid_at = NOW(), updated_at = NOW()
              WHERE id = $1 AND user_id = $2 AND status = 'pending'`
	result, err := p.db.Master.ExecContext(ctx, query, bookingID, userID)
	if err != nil {
		return e.Wrap("storage/eventBroker/Confirm/ExecContext", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return e.ErrNotFound
	}

	return nil
}
func (p *Postgres) BookSeat(ctx context.Context, eventID uuid.UUID, userID uuid.UUID, seatID *int) (uuid.UUID, error) {
	tx, err := p.db.Master.BeginTx(ctx, nil)
	if err != nil {
		return uuid.Nil, e.Wrap("storage/eventBroker/BookSeat/BeginTx", err)
	}

	defer func() {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			p.logger.Error().Err(rollbackErr).Msg("Failed to rollback transaction")
		}
	}()

	var bookedSeats int
	// Проверка занятости конкретного места
	if seatID != nil {
		err = tx.QueryRowContext(ctx, `
            SELECT COUNT(*) FROM bookings WHERE event_id = $1 AND seat_number = $2 AND status IN ('pending', 'confirmed')`,
			eventID, *seatID,
		).Scan(&bookedSeats)
		if err != nil {
			return uuid.Nil, e.Wrap("storage/eventBroker/BookSeat/SeatCheck", err)
		}
		if bookedSeats > 0 {
			return uuid.Nil, e.Wrap("seat is already booked", nil)
		}
	}

	var totalSeats int
	err = tx.QueryRowContext(ctx, `SELECT total_seats FROM events WHERE id = $1`, eventID).Scan(&totalSeats)
	if err != nil {
		return uuid.Nil, e.Wrap("storage/eventBroker/BookSeat/TotalSeatsQuery", err)
	}

	// Проверка общего числа забронированных мест
	var bookedCount int
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM bookings WHERE event_id = $1 AND status IN ('pending', 'confirmed')`, eventID).Scan(&bookedCount)
	if err != nil {
		return uuid.Nil, e.Wrap("storage/eventBroker/BookSeat/CountBooked", err)
	}

	if bookedCount >= totalSeats {
		return uuid.Nil, e.Wrap("no available seats", nil)
	}

	bookingID := uuid.New()
	_, err = tx.ExecContext(ctx, `
        INSERT INTO bookings(id, event_id, user_id, seat_number, status, created_at, updated_at)
        VALUES ($1, $2, $3, $4, 'pending', NOW(), NOW())`,
		bookingID, eventID, userID, seatID,
	)
	if err != nil {
		return uuid.Nil, e.Wrap("storage/eventBroker/BookSeat/Insert", err)
	}

	if err := tx.Commit(); err != nil {
		return uuid.Nil, e.Wrap("storage/eventBroker/BookSeat/Commit", err)
	}

	return bookingID, nil
}

// List возвращает список бронирований для указанного события
func (p *Postgres) ListBookings(ctx context.Context, eventID uuid.UUID) ([]domain.Booking, error) {
	query := `
        SELECT id, event_id, user_id, seat_number, status, created_at, updated_at, paid_at
        FROM bookings
        WHERE event_id = $1
        ORDER BY created_at
    `
	rows, err := p.db.Master.QueryContext(ctx, query, eventID)
	if err != nil {
		return nil, e.Wrap("storage/eventBroker/List/QueryContext", err)
	}
	defer rows.Close()

	var bookings []domain.Booking
	for rows.Next() {
		var b domain.Booking
		var userID sql.NullString
		var seatNumber sql.NullInt32
		var paidAt sql.NullTime

		err := rows.Scan(
			&b.ID,
			&b.EventID,
			&userID,
			&seatNumber,
			&b.Status,
			&b.CreatedAt,
			&b.UpdatedAt,
			&paidAt,
		)
		if err != nil {
			return nil, e.Wrap("storage/eventBroker/List/Rows.Scan", err)
		}

		if userID.Valid {
			uid, err := uuid.Parse(userID.String)
			if err == nil {
				b.UserID = &uid
			}
		}

		if seatNumber.Valid {
			b.SeatNumber = int(seatNumber.Int32)
		}

		if paidAt.Valid {
			b.PaidAt = &paidAt.Time
		}

		bookings = append(bookings, b)
	}

	if err := rows.Err(); err != nil {
		return nil, e.Wrap("storage/eventBroker/List/rows.Err", err)
	}

	return bookings, nil
}

func (p *Postgres) StartExpiredBookingCleanerCron(ctx context.Context, schedule string) error {
	c := cron.New(cron.WithChain(
		cron.SkipIfStillRunning(cron.DefaultLogger), // пропуск, если предыдущий запуск ещё идет
	))

	_, err := c.AddFunc(schedule, func() {
		if err := p.cleanExpiredBookings(ctx); err != nil {
			p.logger.Error().Err(err).Msg("Error cleaning expired bookings")
		} else {
			p.logger.Info().Msg("Expired bookings cleaned successfully")
		}
	})
	if err != nil {
		return err
	}

	c.Start()

	// Ждём отмены контекста (например, сигнала завершения программы)
	<-ctx.Done()

	// Завершаем cron с таймаутом до 5 секунд, чтобы дать шанс завершить текущую задачу
	done := make(chan struct{})
	go func() {
		c.Stop() // Останавливает запуск новых заданий и ждёт завершения текущих
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info().Msg("Cron stopped gracefully")
	case <-time.After(5 * time.Second):
		p.logger.Warn().Msg("Timeout waiting for cron to stop")
	}

	return nil
}
func (p *Postgres) cleanExpiredBookings(ctx context.Context) error {
	query := `
    WITH expired AS (
      SELECT b.id
      FROM bookings b
      JOIN events e ON b.event_id = e.id
      WHERE b.status = 'pending'
        AND b.created_at + e.booking_ttl < NOW()
    )
    UPDATE bookings
    SET status = 'canceled', updated_at = NOW()
    WHERE id IN (SELECT id FROM expired);
`

	_, err := p.db.Master.ExecContext(ctx, query)
	return err
}

func (p *Postgres) ListEvents(ctx context.Context) ([]domain.Event, error) {
	query := `
        SELECT id, name, date, total_seats, requires_payment, booking_ttl, created_at, updated_at
        FROM events
        ORDER BY date
    `
	rows, err := p.db.Master.QueryContext(ctx, query)
	if err != nil {
		return nil, e.Wrap("storage/eventHandler/ListEvents/QueryContext", err)
	}
	defer rows.Close()

	var events []domain.Event
	for rows.Next() {
		var ev domain.Event
		var bookingTTLStr string

		err := rows.Scan(
			&ev.ID,
			&ev.Name,
			&ev.Date,
			&ev.TotalSeats,
			&ev.RequiresPayment,
			&bookingTTLStr,
			&ev.CreatedAt,
			&ev.UpdatedAt,
		)
		if err != nil {
			return nil, e.Wrap("storage/eventHandler/ListEvents/Rows.Scan", err)
		}

		dur, err := time.ParseDuration(parsePgIntervalToDuration(bookingTTLStr))
		if err != nil {
			return nil, e.Wrap("storage/eventHandler/ListEvents/ParseDuration", err)
		}
		ev.BookingTTL = domain.PgDuration{Duration: dur}

		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, e.Wrap("storage/eventHandler/ListEvents/rows.Err", err)
	}
	return events, nil
}
func parsePgIntervalToDuration(s string) string {
	parts := strings.Split(s, ":")
	if len(parts) == 3 {
		return fmt.Sprintf("%sh%sm%ss", parts[0], parts[1], parts[2])
	}
	return s
}

func (p *Postgres) CountBookedSeats(ctx context.Context, eventID uuid.UUID) (int, error) {
	var count int
	query := `
        SELECT COUNT(*) 
        FROM bookings 
        WHERE event_id = $1 AND status IN ('pending', 'confirmed')
    `
	err := p.db.Master.QueryRowContext(ctx, query, eventID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("CountBookedSeats query error: %w", err)
	}
	return count, nil
}

func (p *Postgres) InsertBookingCancelledEvent(ctx context.Context, event *domain.BookingCancelledEvent) error {
	query := `
        INSERT INTO booking_cancelled_events (booking_id, user_id, user_email, telegram_chat_id, cancelled_at)
        VALUES ($1, $2, $3, $4, $5)
    `
	_, err := p.db.Master.ExecContext(ctx, query,
		event.BookingID,
		event.UserID,
		event.UserEmail,
		event.TelegramChatID,
		event.CancelledAt,
	)
	if err != nil {
		return e.Wrap("storage/InsertBookingCancelledEvent", err)
	}
	return nil
}

func (p *Postgres) GetContactByBookingID(ctx context.Context, bookingID uuid.UUID) (userEmail string, telegramChatID string, err error) {
	query := `
        SELECT u.email, u.telegram_id
        FROM bookings b
        JOIN users u ON b.user_id = u.id
        WHERE b.id = $1
        LIMIT 1
    `
	// Объявляем sql.NullString для безопасного сканирования nullable полей
	var email sql.NullString
	var telegramID sql.NullString

	err = p.db.Master.QueryRowContext(ctx, query, bookingID).Scan(&email, &telegramID)
	if err != nil {
		return "", "", e.Wrap("storage/GetContactByBookingID", err)
	}

	if email.Valid {
		userEmail = email.String
	}
	if telegramID.Valid {
		telegramChatID = telegramID.String
	}
	return userEmail, telegramChatID, nil
}

func (p *Postgres) GetEmailByUserID(ctx context.Context, userID uuid.UUID) (string, error) {
	var email sql.NullString
	query := `SELECT email FROM users WHERE id = $1`
	err := p.db.Master.QueryRowContext(ctx, query, userID).Scan(&email)
	if err != nil {
		return "", err
	}
	if email.Valid {
		return email.String, nil
	}
	return "", nil
}

func (p *Postgres) UpdateUserEmail(ctx context.Context, userID uuid.UUID, email string) error {
	query := `UPDATE users SET email = $1, updated_at = NOW() WHERE id = $2`
	_, err := p.db.Master.ExecContext(ctx, query, email, userID)
	return err
}
