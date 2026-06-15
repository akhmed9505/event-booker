package eventbroker_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/akhmed9505/event-booker/internal/config"
	"github.com/akhmed9505/event-booker/internal/domain"
	eventbroker "github.com/akhmed9505/event-booker/internal/ports/rest/eventBroker"
	mock_eventbroker "github.com/akhmed9505/event-booker/internal/ports/rest/eventBroker/mocks"

	"github.com/akhmed9505/event-booker/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func setupRouter(t *testing.T) (*gin.Engine, *mock_eventbroker.MockEventHandler, *mock_eventbroker.MockBookingHandler, *mock_eventbroker.MockNotification, uuid.UUID) {
	ctrl := gomock.NewController(t)
	mockEventHandler := mock_eventbroker.NewMockEventHandler(ctrl)
	mockBookingHandler := mock_eventbroker.NewMockBookingHandler(ctrl)
	mockNotifier := mock_eventbroker.NewMockNotification(ctrl)
	logger := zerolog.Nop()
	cfg := &config.Config{}
	handler := eventbroker.NewHandler(cfg, logger, mockEventHandler, mockBookingHandler, mockNotifier)

	router := gin.New()

	userID := uuid.New()

	// Middleware для вставки userInfo (авторизация)
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "userInfo", &jwt.UserInfo{
			UserID: userID,
		}))
		c.Next()
	})

	// Роуты
	router.POST("/events", handler.Create)
	router.GET("/events/:id", handler.Get)
	router.POST("/events/:id/book", handler.BookSeat)
	router.POST("/bookings/:booking_id/confirm", handler.Confirm)
	router.POST("/bookings/:booking_id/cancel", handler.Cancel)
	router.GET("/profile", handler.ShowProfile)
	router.POST("/profile", handler.UpdateProfile)
	router.GET("/events/:id/book", handler.ShowBookingForm)
	router.GET("/events/:id/detail", handler.ShowEventDetail)
	router.GET("/events", handler.ShowEventList)

	return router, mockEventHandler, mockBookingHandler, mockNotifier, userID
}

func setupRouterNoAuth(t *testing.T, handler *eventbroker.Handler) *gin.Engine {
	router := gin.New()
	router.POST("/bookings/:booking_id/confirm", handler.Confirm)
	router.POST("/bookings/:booking_id/cancel", handler.Cancel)
	return router
}

func TestCreateBindError(t *testing.T) {
	router, _, _, _, _ := setupRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetEvent(t *testing.T) {
	router, mockEv, _, _, _ := setupRouter(t)
	id := uuid.New()

	t.Run("success", func(t *testing.T) {
		ev := &domain.Event{ID: id, Name: "Event1"}

		mockEv.EXPECT().Get(gomock.Any(), id).Return(ev, nil).Times(1)

		req := httptest.NewRequest(http.MethodGet, "/events/"+id.String(), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Event1")
	})

	t.Run("invalid id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/events/bad-uuid", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("not found", func(t *testing.T) {
		mockEv.EXPECT().Get(gomock.Any(), id).Return(nil, errors.New("not found")).Times(1)
		req := httptest.NewRequest(http.MethodGet, "/events/"+id.String(), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestBookSeat(t *testing.T) {
	router, _, mockBk, _, userID := setupRouter(t)

	eventID := uuid.New()
	seatID := 3

	formBody := "seat_id=3"
	req := httptest.NewRequest(http.MethodPost, "/events/"+eventID.String()+"/book", strings.NewReader(formBody))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	mockBk.EXPECT().
		BookSeat(gomock.Any(), eventID, userID, &seatID).
		Return(uuid.New(), nil).
		Times(1)

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusSeeOther, w.Code)
}

func TestConfirmBooking(t *testing.T) {
	bookingID := uuid.New()
	userID := uuid.New()

	ctrl := gomock.NewController(t)
	mockBk := mock_eventbroker.NewMockBookingHandler(ctrl)
	mockNotif := mock_eventbroker.NewMockNotification(ctrl)
	logger := zerolog.Nop()
	cfg := &config.Config{}
	handler := eventbroker.NewHandler(cfg, logger, nil, mockBk, mockNotif)

	t.Run("success", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "userInfo", &jwt.UserInfo{
				UserID: userID,
			}))
			c.Next()
		})
		router.POST("/bookings/:booking_id/confirm", handler.Confirm)

		mockBk.EXPECT().Confirm(gomock.Any(), bookingID, userID).Return(nil).Times(1)

		req := httptest.NewRequest(http.MethodPost, "/bookings/"+bookingID.String()+"/confirm", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("invalid booking id", func(t *testing.T) {
		router, _, _, _, _ := setupRouter(t)

		req := httptest.NewRequest(http.MethodPost, "/bookings/invalid/confirm", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("unauthorized", func(t *testing.T) {
		router := gin.New()
		router.POST("/bookings/:booking_id/confirm", handler.Confirm)

		req := httptest.NewRequest(http.MethodPost, "/bookings/"+bookingID.String()+"/confirm", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("service error", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "userInfo", &jwt.UserInfo{
				UserID: userID,
			}))
			c.Next()
		})
		router.POST("/bookings/:booking_id/confirm", handler.Confirm)

		mockBk.EXPECT().Confirm(gomock.Any(), bookingID, userID).Return(errors.New("fail")).Times(1)

		req := httptest.NewRequest(http.MethodPost, "/bookings/"+bookingID.String()+"/confirm", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestCancelBooking(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEv := mock_eventbroker.NewMockEventHandler(ctrl)
	mockBk := mock_eventbroker.NewMockBookingHandler(ctrl)
	mockNotif := mock_eventbroker.NewMockNotification(ctrl)

	logger := zerolog.Nop()
	cfg := &config.Config{}
	handler := eventbroker.NewHandler(cfg, logger, mockEv, mockBk, mockNotif)

	router := gin.New()
	userID := uuid.New()

	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "userInfo", &jwt.UserInfo{UserID: userID}))
		c.Next()
	})

	router.POST("/bookings/:booking_id/cancel", handler.Cancel)

	bookingID := uuid.New()

	t.Run("success with notification", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/bookings/"+bookingID.String()+"/cancel", nil)

		gomock.InOrder(
			mockBk.EXPECT().Cancel(gomock.Any(), bookingID, userID).Return(nil).Times(1),
			mockBk.EXPECT().GetContactByBookingID(gomock.Any(), bookingID).Return("email@example.com", "telegram-id", nil).Times(1),
			mockBk.EXPECT().InsertBookingCancelledEvent(gomock.Any(), gomock.Any()).Return(nil).Times(1),
			mockNotif.EXPECT().NotifyUserBookingCanceled(gomock.Any(), cfg, "email@example.com", "telegram-id", bookingID).Return(nil).Times(1),
		)

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("success without notification", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/bookings/"+bookingID.String()+"/cancel", nil)

		gomock.InOrder(
			mockBk.EXPECT().Cancel(gomock.Any(), bookingID, userID).Return(nil).Times(1),
			mockBk.EXPECT().GetContactByBookingID(gomock.Any(), bookingID).Return("", "", nil).AnyTimes(),
		)

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("invalid booking id", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/bookings/invalid/cancel", nil)

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("unauthorized", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockBk := mock_eventbroker.NewMockBookingHandler(ctrl)
		mockEv := mock_eventbroker.NewMockEventHandler(ctrl)
		mockNotif := mock_eventbroker.NewMockNotification(ctrl)

		handler := eventbroker.NewHandler(cfg, logger, mockEv, mockBk, mockNotif)

		router := gin.New()

		router.POST("/bookings/:booking_id/cancel", handler.Cancel)

		req := httptest.NewRequest(http.MethodPost, "/bookings/"+bookingID.String()+"/cancel", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("service error", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/bookings/"+bookingID.String()+"/cancel", nil)

		mockBk.EXPECT().Cancel(gomock.Any(), bookingID, userID).Return(errors.New("fail")).Times(1)

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
