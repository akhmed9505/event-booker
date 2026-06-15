// pkg/jwt/jwt_middleware.go (или можно добавить в token_manager.go)

package jwt

import (
	"context"
	"net/http"
	"strings"

	"github.com/wb-go/wbf/ginext"
)

// ... (Permission и rolePermissions) ...

// --- Mock TokenManager для примера ---
// В реальном коде это будет ваш настоящий TokenManager
type MockTokenManager struct{}

type ErrorBody struct {
	Message string `json:"message"`
}

func ProcessError(c *ginext.Context, msg string, code int) {
	body := ErrorBody{
		Message: msg,
	}
	c.AbortWithStatusJSON(code, body)
}
func ValidateTokenMiddleware(tokenManager TokenManager) ginext.HandlerFunc {
	return func(c *ginext.Context) {
		var tokenString string
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			tokenString = authHeader
		} else {
			cookieToken, err := c.Cookie("jwt_token")
			if err == nil && cookieToken != "" {
				tokenString = "Bearer " + cookieToken
			} else {
				tokenString = c.Query("access_token")
				if tokenString == "" {
					c.AbortWithStatusJSON(http.StatusUnauthorized, ginext.H{"error": "empty auth header, cookie or query"})
					return
				}
			}
		}

		headerParts := strings.Split(tokenString, " ")
		if len(headerParts) != 2 || strings.ToLower(headerParts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ginext.H{"error": "invalid auth header format"})
			return
		}
		token := headerParts[1]

		userInfo, err := tokenManager.Parse(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ginext.H{"error": "unauthorized: invalid or expired token"})
			return
		}
		if userInfo == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ginext.H{"error": "unauthorized: empty user info"})
			return
		}

		c.Set("userInfo", userInfo)
		ctx := context.WithValue(c.Request.Context(), "userInfo", userInfo)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}

}

// RequireRole - middleware для проверки наличия хотя бы одной из требуемых ролей
func RequireRole(requiredRoles ...Role) ginext.HandlerFunc {
	return func(c *ginext.Context) {
		// 1. Получаем UserInfo из контекста
		claims, exists := c.Get("userInfo") // Используем ключ "userInfo"
		if !exists {
			ProcessError(c, "authentication middleware not found or failed", http.StatusInternalServerError)
			return
		}
		userInfo, ok := claims.(*UserInfo) // Приводим к нашему типу
		if !ok {
			ProcessError(c, "internal server error: invalid user info type", http.StatusInternalServerError)
			return
		}

		// 2. Проверяем, есть ли у пользователя хотя бы одна из требуемых ролей
		hasRequiredRole := false
		for _, requiredRole := range requiredRoles {
			for _, userRole := range userInfo.Roles {
				if userRole == requiredRole {
					hasRequiredRole = true
					break
				}
			}
			if hasRequiredRole {
				break
			}
		}

		// 3. Если нужной роли нет, возвращаем ошибку
		if !hasRequiredRole {
			ProcessError(c, "access denied: insufficient permissions", http.StatusForbidden)
			return
		}

		// 4. Если роль есть, передаем управление дальше
		c.Next()
	}
}

// --- Helper function to get UserInfo from ginext context ---
// Эта функция может быть полезна в ваших обработчиках
func GetUserInfoFromContext(ctx context.Context) (*UserInfo, bool) {
	val := ctx.Value("userInfo")
	userInfo, ok := val.(*UserInfo)
	if !ok {
		return nil, false
	}
	return userInfo, true
}
