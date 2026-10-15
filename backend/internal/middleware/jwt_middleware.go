package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"muara.bi/core/internal/service"
	"muara.bi/core/pkg/httputil"
)

type contextKey string

const (
	UserIDKey         contextKey = "userID"
	UserEmailKey      contextKey = "userEmail"
	UserAttributesKey contextKey = "userAttributes"
)

func JWTMiddleware(secretKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				httputil.WriteUnauthorizedResponse(w, r, "Missing or invalid Authorization header")
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			claims := &service.JWTClaims{}

			token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(secretKey), nil
			})

			if err != nil || !token.Valid {
				httputil.WriteUnauthorizedResponse(w, r, "Invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, UserEmailKey, claims.Email)
			ctx = context.WithValue(ctx, UserAttributesKey, claims.Attributes)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserIDFromContext(ctx context.Context) string {
	if val, ok := ctx.Value(UserIDKey).(string); ok {
		return val
	}
	return ""
}

func GetUserAttributesFromContext(ctx context.Context) json.RawMessage {
	if val, ok := ctx.Value(UserAttributesKey).(json.RawMessage); ok {
		return val
	}
	return json.RawMessage("{}")
}
