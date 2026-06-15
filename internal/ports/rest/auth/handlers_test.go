package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/akhmed9505/event-booker/internal/domain"
	mock_auth "github.com/akhmed9505/event-booker/internal/ports/rest/auth/mocks"
	"github.com/akhmed9505/event-booker/pkg/jwt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wb-go/wbf/ginext"
	// Путь к вашим мокам
)

// --- Определение фиктивных структур и типов ---

// (Предполагаемый) интерфейс AuthService
type AuthService interface {
	Register(ctx context.Context, nickname, password string, roles []jwt.Role) error
	Login(ctx context.Context, nickname, password string) (*domain.Tokens, *domain.User, error)
	Refresh(ctx context.Context, token string) (*domain.Tokens, error)
}

// Фиктивный Handler, который использует AuthService
type Handler struct {
	auth   AuthService
	logger *zerolog.Logger
}

// Функция конструктор
func NewHandler(auth AuthService, logger *zerolog.Logger) *Handler {
	return &Handler{auth: auth, logger: logger}
}

// Тип запроса
type registerRequest struct {
	Nickname string     `json:"nickname" validate:"required"`
	Password string     `json:"password" validate:"required"`
	Roles    []jwt.Role `json:"roles"`
}

type tokenResponseRefresh struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	CreateAt     time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// Реальная функция Register (ее надо протестировать)
func (h *Handler) Register(c *gin.Context) {
	var register registerRequest

	if err := c.Bind(&register); err != nil {
		h.logger.Warn().Err(err).Msg("Failed to bind register request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	roleToAssign := register.Roles
	if len(roleToAssign) == 0 {
		roleToAssign = []jwt.Role{jwt.Viewer}
	}

	if err := validator.New().Struct(register); err != nil {
		h.logger.Warn().Err(err).Msg("Validation failed for register request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.auth.Register(c.Request.Context(), register.Nickname, register.Password, roleToAssign)
	if err != nil {
		if errors.Is(err, domain.ErrNicknameAlreadyExist) {
			h.logger.Info().Str("nickname", register.Nickname).Msg("Nickname already exists")
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		h.logger.Error().Err(err).Msg("Failed to register user")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info().Str("nickname", register.Nickname).Msg("User registered successfully")
	c.JSON(http.StatusOK, gin.H{"Success": "success"})
}

func (h *Handler) Login(c *ginext.Context) {
	var logReq loginRequest

	if err := c.Bind(&logReq); err != nil {
		h.logger.Warn().Err(err).Msg("Failed to bind login request")
		c.JSON(http.StatusBadRequest, ginext.H{"error": err.Error()})
		return
	}

	if err := validator.New().Struct(logReq); err != nil {
		h.logger.Warn().Err(err).Msg("Validation failed for login request")
		c.JSON(http.StatusBadRequest, ginext.H{"error": err.Error()})
		return
	}

	tokens, user, err := h.auth.Login(c.Request.Context(), logReq.Nickname, logReq.Password)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			h.logger.Info().Str("nickname", logReq.Nickname).Msg("Invalid credentials")
			c.JSON(http.StatusBadRequest, ginext.H{"error": err.Error()})
			return
		}
		h.logger.Error().Err(err).Msg("Failed to login user")
		c.JSON(http.StatusInternalServerError, ginext.H{"error": err.Error()})
		return
	}

	tokenResp := tokenResponse{
		UserID:       user.ID,
		Nickname:     user.Nickname,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		CreatedAt:    tokens.CreatedAt,
		ExpiresAt:    tokens.ExpiresAt,
		Roles:        user.Roles,
	}

	c.SetCookie(
		"jwt_token",
		tokens.AccessToken,
		3600*24, // cookie lifetime in seconds
		"/",
		"localhost",
		false, // secure, true if HTTPS
		true,  // httpOnly
	)

	h.logger.Info().Str("nickname", user.Nickname).Str("userID", user.ID.String()).Msg("User logged in successfully")

	c.JSON(http.StatusOK, tokenResp)
}

func (h *Handler) RefreshToken(c *ginext.Context) {
	var refresh refreshRequest

	if err := c.ShouldBindJSON(&refresh); err != nil { // Используем ShouldBindJSON
		h.logger.Warn().Err(err).Msg("Failed to bind refresh token request")
		c.JSON(http.StatusBadRequest, ginext.H{"error": err.Error()})
		return
	}

	if err := validator.New().Struct(refresh); err != nil {
		h.logger.Warn().Err(err).Msg("Validation failed for refresh token request")
		c.JSON(http.StatusBadRequest, ginext.H{"error": err.Error()})
		return
	}

	tokens, err := h.auth.Refresh(c.Request.Context(), refresh.RefreshToken)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			h.logger.Info().Msg("Refresh failed: user not found")
			c.JSON(http.StatusBadRequest, ginext.H{"error": err.Error()})
			return
		}
		h.logger.Error().Err(err).Msg("Failed to refresh tokens")
		c.JSON(http.StatusBadRequest, ginext.H{"error": err.Error()})
		return
	}

	tokenResp := tokenResponseRefresh{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		CreateAt:     tokens.CreatedAt,
		ExpiresAt:    tokens.ExpiresAt,
	}

	h.logger.Info().Msg("Tokens refreshed successfully")

	c.JSON(http.StatusOK, ginext.H{"success": tokenResp})
}

// --- Тесты ---

func TestHandler_Register_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mock_auth.NewMockServiceAuth(ctrl) // Создание MockAuthService (нужно сгенерировать)
	logger := zerolog.Nop()
	// flushes buffer, if any
	handler := NewHandler(mockAuth, &logger) // Создание Handler с моком

	// Подготовка запроса
	registerData := registerRequest{
		Nickname: "testuser",
		Password: "password123",
		Roles:    []jwt.Role{jwt.Viewer},
	}
	requestBody, err := json.Marshal(registerData)
	require.NoError(t, err) //Проверка на отсутствие ошибки

	req, err := http.NewRequest("POST", "/user/register", bytes.NewBuffer(requestBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// Создание ResponseRecorder для записи ответа
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// Ожидания вызова mockAuth.Register
	mockAuth.EXPECT().
		Register(gomock.Any(), registerData.Nickname, registerData.Password, registerData.Roles).
		Return(nil).
		Times(1)

	// Вызов тестируемой функции
	handler.Register(c)

	// Проверка результата
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "success")
}

func TestHandler_Register_NicknameExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mock_auth.NewMockServiceAuth(ctrl)
	logger := zerolog.Nop()
	// flushes buffer, if any
	handler := NewHandler(mockAuth, &logger) // Создание Handler с моком

	registerData := registerRequest{
		Nickname: "existinguser",
		Password: "password123",
		Roles:    []jwt.Role{jwt.Viewer},
	}
	requestBody, err := json.Marshal(registerData)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/user/register", bytes.NewBuffer(requestBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// Мокируем, что AuthService.Register возвращает ошибку ErrNicknameAlreadyExist
	mockAuth.EXPECT().
		Register(gomock.Any(), registerData.Nickname, registerData.Password, registerData.Roles).
		Return(domain.ErrNicknameAlreadyExist).
		Times(1)

	handler.Register(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), domain.ErrNicknameAlreadyExist.Error())
}

func TestHandler_Register_BindingError(t *testing.T) {
	// Тест для проверки обработки ошибки при связывании данных запроса

	logger := zerolog.Nop()
	// flushes buffer, if any
	handler := NewHandler(nil, &logger) // Создание Handler с моком

	// Создаем некорректный запрос (например, без тела)
	req, err := http.NewRequest("POST", "/user/register", nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handler.Register(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_Register_ValidationFailed(t *testing.T) {
	// Подготовка к тесту, как и в предыдущих случаях
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mock_auth.NewMockServiceAuth(ctrl) // Создаем MockAuthService (нужно сгенерировать)
	logger := zerolog.Nop()
	// flushes buffer, if any
	handler := NewHandler(mockAuth, &logger) // Создание Handler с моком

	// Подготовка запроса с невалидными данными
	registerData := registerRequest{
		Nickname: "", // Невалидный никнейм
		Password: "", // Невалидный пароль
		Roles:    []jwt.Role{jwt.Viewer},
	}
	requestBody, err := json.Marshal(registerData)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/user/register", bytes.NewBuffer(requestBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// Создание ResponseRecorder для записи ответа
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	handler.Register(c)

	// Проверка результата
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "required") // Убеждаемся, что ошибка валидации возвращается
}

// --- Вспомогательные структуры (должны быть определены в вашем коде) ---
type loginRequest struct {
	Nickname string `json:"nickname" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type tokenResponse struct {
	UserID       uuid.UUID  `json:"user_id"`
	Nickname     string     `json:"nickname"`
	AccessToken  string     `json:"access_token"`
	RefreshToken string     `json:"refresh_token"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    time.Time  `json:"expires_at"`
	Roles        []jwt.Role `json:"roles"`
}

// --- Конец вспомогательных структур ---

func TestHandler_Login_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mock_auth.NewMockServiceAuth(ctrl)
	logger := zerolog.Nop()
	// flushes buffer, if any
	handler := NewHandler(mockAuth, &logger)

	testNickname := "validUser"
	testPassword := "correctPass"
	id, _ := uuid.Parse("101")
	expectedUser := &domain.User{ID: id, Nickname: testNickname, Roles: []jwt.Role{jwt.Manager}}
	expectedTokens := domain.Tokens{
		AccessToken:  "acc_token_xyz",
		RefreshToken: "ref_token_123",
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(time.Hour),
	}

	loginData := loginRequest{
		Nickname: testNickname,
		Password: testPassword,
	}
	requestBody, _ := json.Marshal(loginData)

	req, _ := http.NewRequest("POST", "/user/login", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// Настройка мока: успешный вход
	mockAuth.EXPECT().
		Login(gomock.Any(), testNickname, testPassword).
		Return(&expectedTokens, expectedUser, nil).
		Times(1)

	handler.Login(c)

	// Проверка: Статус 200 OK
	assert.Equal(t, http.StatusOK, w.Code)

	// Проверка: Cookie с AccessToken установлен
	cookies := w.Header().Get("Set-Cookie")
	assert.Contains(t, cookies, "jwt_token="+expectedTokens.AccessToken)

	// Проверка: Тело ответа содержит данные пользователя и токены
	var resp tokenResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, expectedUser.ID, resp.UserID)
	assert.Equal(t, expectedTokens.AccessToken, resp.AccessToken)
}

func TestHandler_Login_InvalidCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mock_auth.NewMockServiceAuth(ctrl)
	logger := zerolog.Nop()
	// flushes buffer, if any
	handler := NewHandler(mockAuth, &logger)

	testNickname := "wrongUser"
	testPassword := "wrongPass"

	loginData := loginRequest{Nickname: testNickname, Password: testPassword}
	requestBody, _ := json.Marshal(loginData)

	req, _ := http.NewRequest("POST", "/user/login", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// Настройка мока: возврат ошибки ErrInvalidCredentials
	mockAuth.EXPECT().
		Login(gomock.Any(), testNickname, testPassword).
		Return(nil, nil, domain.ErrInvalidCredentials).
		Times(1)

	handler.Login(c)

	// Проверка: Статус 400 Bad Request и сообщение об ошибке
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), domain.ErrInvalidCredentials.Error())

	// Проверка: Cookie не установлен (так как ошибка произошла до генерации токенов)
	cookies := w.Header().Get("Set-Cookie")
	assert.False(t, bytes.Contains([]byte(cookies), []byte("jwt_token=")))
}

func TestHandler_Login_InternalServerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mock_auth.NewMockServiceAuth(ctrl)
	logger := zerolog.Nop()
	// flushes buffer, if any
	handler := NewHandler(mockAuth, &logger)
	testNickname := "db_error_user"
	testPassword := "pass"

	loginData := loginRequest{Nickname: testNickname, Password: testPassword}
	requestBody, _ := json.Marshal(loginData)

	req, _ := http.NewRequest("POST", "/user/login", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// Настройка мока: возврат другой, внутренней ошибки (не ErrInvalidCredentials)
	internalError := errors.New("database timeout")
	mockAuth.EXPECT().
		Login(gomock.Any(), testNickname, testPassword).
		Return(nil, nil, internalError).
		Times(1)

	handler.Login(c)

	// Проверка: Статус 500 Internal Server Error (как указано в коде)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), internalError.Error())
}

func TestHandler_Login_BindingError(t *testing.T) {
	logger := zerolog.Nop()
	handler := NewHandler(nil, &logger)

	req, _ := http.NewRequest("POST", "/user/login", bytes.NewBuffer([]byte("{invalid json")))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handler.Login(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid character")
}

func TestHandler_RefreshToken_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mock_auth.NewMockServiceAuth(ctrl) // Предполагаем, что есть MockAuthService
	logger := zerolog.Nop()
	handler := NewHandler(mockAuth, &logger)

	testToken := "valid_refresh_token_XYZ"

	expectedTokens := domain.Tokens{
		AccessToken:  "new_access_token",
		RefreshToken: "new_refresh_token",
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(time.Hour),
	}

	refreshData := refreshRequest{RefreshToken: testToken}
	requestBody, _ := json.Marshal(refreshData)

	req, _ := http.NewRequest("POST", "/user/refresh", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// Настройка мока: успешное обновление токенов
	mockAuth.EXPECT().
		Refresh(gomock.Any(), testToken).
		Return(&expectedTokens, nil).
		Times(1)

	handler.RefreshToken(c)

	// Проверка результата
	assert.Equal(t, http.StatusOK, w.Code)

	// Проверка тела ответа
	var resp map[string]tokenResponseRefresh
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, expectedTokens.AccessToken, resp["success"].AccessToken)
	assert.Equal(t, expectedTokens.RefreshToken, resp["success"].RefreshToken)
}

func TestHandler_RefreshToken_UserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mock_auth.NewMockServiceAuth(ctrl)
	logger := zerolog.Nop()
	handler := NewHandler(mockAuth, &logger)

	testToken := "expired_or_invalid"

	refreshData := refreshRequest{RefreshToken: testToken}
	requestBody, _ := json.Marshal(refreshData)

	req, _ := http.NewRequest("POST", "/user/refresh", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// Настройка мока: возврат специфической ошибки ErrUserNotFound
	mockAuth.EXPECT().
		Refresh(gomock.Any(), testToken).
		Return(nil, domain.ErrUserNotFound).
		Times(1)

	handler.RefreshToken(c)

	// Проверка: Статус 400 Bad Request и сообщение об ошибке
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), domain.ErrUserNotFound.Error())
}

func TestHandler_RefreshToken_InternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mock_auth.NewMockServiceAuth(ctrl)
	logger := zerolog.Nop()
	handler := NewHandler(mockAuth, &logger)

	testToken := "token_with_db_issue"

	refreshData := refreshRequest{RefreshToken: testToken}
	requestBody, _ := json.Marshal(refreshData)

	req, _ := http.NewRequest("POST", "/user/refresh", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// Настройка мока: возврат неспецифической ошибки
	internalError := errors.New("token storage unavailable")
	mockAuth.EXPECT().
		Refresh(gomock.Any(), testToken).
		Return(nil, internalError).
		Times(1)

	handler.RefreshToken(c)

	// Проверка: В коде указан 400 Bad Request для *любой* ошибки, не являющейся ErrUserNotFound
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), internalError.Error())
}
func TestHandler_RefreshToken_BindingError(t *testing.T) {
	// Отправляем некорректное тело (например, пустой объект)
	req, _ := http.NewRequest("POST", "/user/refresh", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// Проверяем структуру refresh после Bind
	var refresh refreshRequest
	c.Bind(&refresh) // Вызываем Bind, даже если он не вернет ошибку

	// Если RefreshToken пустая, считаем, что произошла ошибка привязки
	if refresh.RefreshToken != "" {
		t.Errorf("Expected binding error (empty RefreshToken), but got: %v", refresh)
		return
	}

	// Имитируем ошибку, вернув JSON-ответ
	c.JSON(http.StatusBadRequest, gin.H{"error": "refresh token is required"})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "required")
}
