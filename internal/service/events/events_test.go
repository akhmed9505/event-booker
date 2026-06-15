package events_test

import (
	"context"
	"errors"

	"github.com/akhmed9505/event-booker/internal/domain"

	"testing"

	"github.com/akhmed9505/event-booker/internal/service/events"
	mock_events "github.com/akhmed9505/event-booker/internal/service/events/mocks"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestEventService_Create(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvent := mock_events.NewMockEventService(ctrl)
	mockBooking := mock_events.NewMockBookingService(ctrl)
	mockNotifier := mock_events.NewMockNotifier(ctrl)
	s := events.NewService(mockEvent, mockBooking, mockNotifier)

	ctx := context.Background()
	evt := &domain.Event{Name: "Test Event"}
	id := uuid.New()

	// 1. Успешное создание
	mockEvent.EXPECT().Create(ctx, evt).Return(id, nil).Times(1)
	gotID, err := s.Create(ctx, evt)
	assert.NoError(t, err)
	assert.Equal(t, id, gotID)

	// 2. Ошибка создания
	mockEvent.EXPECT().Create(ctx, evt).Return(uuid.Nil, errors.New("create error")).Times(1)
	gotID, err = s.Create(ctx, evt)
	assert.Error(t, err)
	assert.Equal(t, uuid.Nil, gotID)

	// 3. Пограничный случай: nil event
	mockEvent.EXPECT().Create(ctx, (*domain.Event)(nil)).Return(uuid.Nil, errors.New("nil event")).Times(1)
	gotID, err = s.Create(ctx, nil)
	assert.Error(t, err)
	assert.Equal(t, uuid.Nil, gotID)
}

func TestEventService_Get(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvent := mock_events.NewMockEventService(ctrl)
	mockBooking := mock_events.NewMockBookingService(ctrl)
	mockNotifier := mock_events.NewMockNotifier(ctrl)
	s := events.NewService(mockEvent, mockBooking, mockNotifier)

	ctx := context.Background()
	id := uuid.New()
	expectedEvent := &domain.Event{ID: id, Name: "Get Test"}

	// 1. Успешный get
	mockEvent.EXPECT().Get(ctx, id).Return(expectedEvent, nil).Times(1)
	ev, err := s.Get(ctx, id)
	assert.NoError(t, err)
	assert.Equal(t, expectedEvent, ev)

	// 2. Ошибка get
	mockEvent.EXPECT().Get(ctx, id).Return(nil, errors.New("get error")).Times(1)
	ev, err = s.Get(ctx, id)
	assert.Error(t, err)
	assert.Nil(t, ev)

	// 3. Пограничный случай: несуществующий UUID
	fakeID := uuid.New()
	mockEvent.EXPECT().Get(ctx, fakeID).Return(nil, nil).Times(1)
	ev, err = s.Get(ctx, fakeID)
	assert.NoError(t, err)
	assert.Nil(t, ev)
}

func TestEventService_ListEvents(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvent := mock_events.NewMockEventService(ctrl)
	mockBooking := mock_events.NewMockBookingService(ctrl)
	mockNotifier := mock_events.NewMockNotifier(ctrl)
	s := events.NewService(mockEvent, mockBooking, mockNotifier)

	ctx := context.Background()

	expectedEvents := []domain.Event{{ID: uuid.New(), Name: "Event 1"}}

	// 1. Успешное листинг
	mockEvent.EXPECT().ListEvents(ctx).Return(expectedEvents, nil).Times(1)
	events, err := s.ListEvents(ctx)
	assert.NoError(t, err)
	assert.Equal(t, expectedEvents, events)

	// 2. Ошибка листинг
	mockEvent.EXPECT().ListEvents(ctx).Return(nil, errors.New("list error")).Times(1)
	events, err = s.ListEvents(ctx)
	assert.Error(t, err)
	assert.Nil(t, events)

	// 3. Пустой листинг
	mockEvent.EXPECT().ListEvents(ctx).Return([]domain.Event{}, nil).Times(1)
	events, err = s.ListEvents(ctx)
	assert.NoError(t, err)
	assert.Empty(t, events)
}

func TestEventService_GetUserNameByID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvent := mock_events.NewMockEventService(ctrl)
	mockBooking := mock_events.NewMockBookingService(ctrl)
	mockNotifier := mock_events.NewMockNotifier(ctrl)
	s := events.NewService(mockEvent, mockBooking, mockNotifier)

	ctx := context.Background()
	userID := uuid.New()
	name := "user123"

	// 1. Успешно
	mockEvent.EXPECT().GetUserNameByID(ctx, userID).Return(name, nil).Times(1)
	gotName, err := s.GetUserNameByID(ctx, userID)
	assert.NoError(t, err)
	assert.Equal(t, name, gotName)

	// 2. Ошибка
	mockEvent.EXPECT().GetUserNameByID(ctx, userID).Return("", errors.New("get name error")).Times(1)
	gotName, err = s.GetUserNameByID(ctx, userID)
	assert.Error(t, err)
	assert.Empty(t, gotName)

	// 3. Пустое имя без ошибки
	mockEvent.EXPECT().GetUserNameByID(ctx, userID).Return("", nil).Times(1)
	gotName, err = s.GetUserNameByID(ctx, userID)
	assert.NoError(t, err)
	assert.Empty(t, gotName)
}

func TestEventService_GetEmailByUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvent := mock_events.NewMockEventService(ctrl)
	mockBooking := mock_events.NewMockBookingService(ctrl)
	mockNotifier := mock_events.NewMockNotifier(ctrl)
	s := events.NewService(mockEvent, mockBooking, mockNotifier)

	ctx := context.Background()
	userID := uuid.New()
	email := "test@example.com"

	// 1. Успешно
	mockEvent.EXPECT().GetEmailByUserID(ctx, userID).Return(email, nil).Times(1)
	gotEmail, err := s.GetEmailByUserID(ctx, userID)
	assert.NoError(t, err)
	assert.Equal(t, email, gotEmail)

	// 2. Ошибка
	mockEvent.EXPECT().GetEmailByUserID(ctx, userID).Return("", errors.New("error")).Times(1)
	gotEmail, err = s.GetEmailByUserID(ctx, userID)
	assert.Error(t, err)
	assert.Empty(t, gotEmail)

	// 3. Пустой email без ошибки
	mockEvent.EXPECT().GetEmailByUserID(ctx, userID).Return("", nil).Times(1)
	gotEmail, err = s.GetEmailByUserID(ctx, userID)
	assert.NoError(t, err)
	assert.Empty(t, gotEmail)
}

func TestEventService_UpdateUserEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvent := mock_events.NewMockEventService(ctrl)
	mockBooking := mock_events.NewMockBookingService(ctrl)
	mockNotifier := mock_events.NewMockNotifier(ctrl)
	s := events.NewService(mockEvent, mockBooking, mockNotifier)

	ctx := context.Background()
	userID := uuid.New()
	newEmail := "new@email.com"

	// 1. Успешно
	mockEvent.EXPECT().UpdateUserEmail(ctx, userID, newEmail).Return(nil).Times(1)
	err := s.UpdateUserEmail(ctx, userID, newEmail)
	assert.NoError(t, err)

	// 2. Ошибка
	mockEvent.EXPECT().UpdateUserEmail(ctx, userID, newEmail).Return(errors.New("update error")).Times(1)
	err = s.UpdateUserEmail(ctx, userID, newEmail)
	assert.Error(t, err)

	// 3. Пустой email (можно тестировать по делу, если метод позволяет)
	mockEvent.EXPECT().UpdateUserEmail(ctx, userID, "").Return(nil).Times(1)
	err = s.UpdateUserEmail(ctx, userID, "")
	assert.NoError(t, err)
}

func TestBookingService_BookSeat(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvent := mock_events.NewMockEventService(ctrl)
	mockBooking := mock_events.NewMockBookingService(ctrl)
	mockNotifier := mock_events.NewMockNotifier(ctrl)
	s := events.NewService(mockEvent, mockBooking, mockNotifier)

	ctx := context.Background()
	eventID := uuid.New()
	userID := uuid.New()
	seatID := 5
	bookingID := uuid.New()

	// 1. Успешное бронирование
	mockBooking.EXPECT().BookSeat(ctx, eventID, userID, &seatID).Return(bookingID, nil).Times(1)
	gotID, err := s.BookSeat(ctx, eventID, userID, &seatID)
	assert.NoError(t, err)
	assert.Equal(t, bookingID, gotID)

	// 2. Ошибка бронирования
	mockBooking.EXPECT().BookSeat(ctx, eventID, userID, &seatID).Return(uuid.Nil, errors.New("booking error")).Times(1)
	gotID, err = s.BookSeat(ctx, eventID, userID, &seatID)
	assert.Error(t, err)
	assert.Equal(t, uuid.Nil, gotID)

	// 3. Бронирование без указания места (seatID == nil)
	mockBooking.EXPECT().BookSeat(ctx, eventID, userID, nil).Return(bookingID, nil).Times(1)
	gotID, err = s.BookSeat(ctx, eventID, userID, nil)
	assert.NoError(t, err)
	assert.Equal(t, bookingID, gotID)
}

func TestBookingService_Confirm(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvent := mock_events.NewMockEventService(ctrl)
	mockBooking := mock_events.NewMockBookingService(ctrl)
	mockNotifier := mock_events.NewMockNotifier(ctrl)
	s := events.NewService(mockEvent, mockBooking, mockNotifier)

	ctx := context.Background()
	bookingID := uuid.New()
	userID := uuid.New()

	// 1. Успешное подтверждение
	mockBooking.EXPECT().Confirm(ctx, bookingID, userID).Return(nil).Times(1)
	err := s.Confirm(ctx, bookingID, userID)
	assert.NoError(t, err)

	// 2. Ошибка подтверждения
	mockBooking.EXPECT().Confirm(ctx, bookingID, userID).Return(errors.New("confirm error")).Times(1)
	err = s.Confirm(ctx, bookingID, userID)
	assert.Error(t, err)

	// 3. С другим UUID
	anotherBookingID := uuid.New()
	anotherUserID := uuid.New()
	mockBooking.EXPECT().Confirm(ctx, anotherBookingID, anotherUserID).Return(nil).Times(1)
	err = s.Confirm(ctx, anotherBookingID, anotherUserID)
	assert.NoError(t, err)
}

func TestBookingService_Cancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvent := mock_events.NewMockEventService(ctrl)
	mockBooking := mock_events.NewMockBookingService(ctrl)
	mockNotifier := mock_events.NewMockNotifier(ctrl)
	s := events.NewService(mockEvent, mockBooking, mockNotifier)

	ctx := context.Background()
	bookingID := uuid.New()
	userID := uuid.New()

	// 1. Успешная отмена
	mockBooking.EXPECT().Cancel(ctx, bookingID, userID).Return(nil).Times(1)
	err := s.Cancel(ctx, bookingID, userID)
	assert.NoError(t, err)

	// 2. Ошибка отмены
	mockBooking.EXPECT().Cancel(ctx, bookingID, userID).Return(errors.New("cancel error")).Times(1)
	err = s.Cancel(ctx, bookingID, userID)
	assert.Error(t, err)

	// 3. С другими UUID
	anotherBookingID := uuid.New()
	anotherUserID := uuid.New()
	mockBooking.EXPECT().Cancel(ctx, anotherBookingID, anotherUserID).Return(nil).Times(1)
	err = s.Cancel(ctx, anotherBookingID, anotherUserID)
	assert.NoError(t, err)
}

func TestBookingService_ListBookings(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvent := mock_events.NewMockEventService(ctrl)
	mockBooking := mock_events.NewMockBookingService(ctrl)
	mockNotifier := mock_events.NewMockNotifier(ctrl)
	s := events.NewService(mockEvent, mockBooking, mockNotifier)

	ctx := context.Background()
	eventID := uuid.New()
	bookings := []domain.Booking{{ID: uuid.New()}}

	// 1. Успешно
	mockBooking.EXPECT().ListBookings(ctx, eventID).Return(bookings, nil).Times(1)
	gotBookings, err := s.ListBookings(ctx, eventID)
	assert.NoError(t, err)
	assert.Equal(t, bookings, gotBookings)

	// 2. Ошибка
	mockBooking.EXPECT().ListBookings(ctx, eventID).Return(nil, errors.New("list error")).Times(1)
	gotBookings, err = s.ListBookings(ctx, eventID)
	assert.Error(t, err)
	assert.Nil(t, gotBookings)

	// 3. Пустой список
	mockBooking.EXPECT().ListBookings(ctx, eventID).Return([]domain.Booking{}, nil).Times(1)
	gotBookings, err = s.ListBookings(ctx, eventID)
	assert.NoError(t, err)
	assert.Empty(t, gotBookings)
}

func TestBookingService_CountBookedSeats(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvent := mock_events.NewMockEventService(ctrl)
	mockBooking := mock_events.NewMockBookingService(ctrl)
	mockNotifier := mock_events.NewMockNotifier(ctrl)
	s := events.NewService(mockEvent, mockBooking, mockNotifier)

	ctx := context.Background()
	eventID := uuid.New()

	// 1. Успешно
	mockBooking.EXPECT().CountBookedSeats(ctx, eventID).Return(42, nil).Times(1)
	count, err := s.CountBookedSeats(ctx, eventID)
	assert.NoError(t, err)
	assert.Equal(t, 42, count)

	// 2. Ошибка
	mockBooking.EXPECT().CountBookedSeats(ctx, eventID).Return(0, errors.New("count error")).Times(1)
	count, err = s.CountBookedSeats(ctx, eventID)
	assert.Error(t, err)
	assert.Equal(t, 0, count)

	// 3. 0 мест
	mockBooking.EXPECT().CountBookedSeats(ctx, eventID).Return(0, nil).Times(1)
	count, err = s.CountBookedSeats(ctx, eventID)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestBookingService_InsertBookingCancelledEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvent := mock_events.NewMockEventService(ctrl)
	mockBooking := mock_events.NewMockBookingService(ctrl)
	mockNotifier := mock_events.NewMockNotifier(ctrl)
	s := events.NewService(mockEvent, mockBooking, mockNotifier)

	ctx := context.Background()
	event := &domain.BookingCancelledEvent{}

	// 1. Успешно
	mockBooking.EXPECT().InsertBookingCancelledEvent(ctx, event).Return(nil).Times(1)
	err := s.InsertBookingCancelledEvent(ctx, event)
	assert.NoError(t, err)

	// 2. Ошибка
	mockBooking.EXPECT().InsertBookingCancelledEvent(ctx, event).Return(errors.New("insert error")).Times(1)
	err = s.InsertBookingCancelledEvent(ctx, event)
	assert.Error(t, err)

	// 3. Пограничный тест с пустым событием
	mockBooking.EXPECT().InsertBookingCancelledEvent(ctx, (*domain.BookingCancelledEvent)(nil)).Return(errors.New("nil event")).Times(1)
	err = s.InsertBookingCancelledEvent(ctx, nil)
	assert.Error(t, err)
}

func TestBookingService_GetContactByBookingID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvent := mock_events.NewMockEventService(ctrl)
	mockBooking := mock_events.NewMockBookingService(ctrl)
	mockNotifier := mock_events.NewMockNotifier(ctrl)
	s := events.NewService(mockEvent, mockBooking, mockNotifier)

	ctx := context.Background()
	bookingID := uuid.New()
	email := "user@example.com"
	telegramID := "123456"

	// 1. Успешно
	mockBooking.EXPECT().GetContactByBookingID(ctx, bookingID).Return(email, telegramID, nil).Times(1)
	em, tid, err := s.GetContactByBookingID(ctx, bookingID)
	assert.NoError(t, err)
	assert.Equal(t, email, em)
	assert.Equal(t, telegramID, tid)

	// 2. Ошибка
	mockBooking.EXPECT().GetContactByBookingID(ctx, bookingID).Return("", "", errors.New("error")).Times(1)
	em, tid, err = s.GetContactByBookingID(ctx, bookingID)
	assert.Error(t, err)
	assert.Empty(t, em)
	assert.Empty(t, tid)

	// 3. Пустые данные, без ошибки
	mockBooking.EXPECT().GetContactByBookingID(ctx, bookingID).Return("", "", nil).Times(1)
	em, tid, err = s.GetContactByBookingID(ctx, bookingID)
	assert.NoError(t, err)
	assert.Empty(t, em)
	assert.Empty(t, tid)
}
