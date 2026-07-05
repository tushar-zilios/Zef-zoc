package routes

import (
	"context"
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
	"time"

	"zoc/src/internal/logger"
	"zoc/src/internal/utils"

	"github.com/go-chi/chi/v5/middleware"
)

func handlerLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		logger.LogHandler("%s %s %s from %s - %d %dB in %s",
			r.Method, r.URL.Path, r.Proto, r.RemoteAddr,
			ww.Status(), ww.BytesWritten(), time.Since(start))
	})
}

func JWTMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rawToken string

		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				utils.WriteError(w, http.StatusUnauthorized, "Authorization header must be in format Bearer <token>")
				return
			}
			rawToken = parts[1]
		} else {
			rawToken = r.URL.Query().Get("token")
		}

		if rawToken == "" {
			utils.WriteError(w, http.StatusUnauthorized, "Authorization required")
			return
		}

		claims, err := utils.VerifyToken(rawToken)
		if err != nil {
			utils.WriteError(w, http.StatusUnauthorized, "Invalid or expired token")
			return
		}
		ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
		ctx = context.WithValue(ctx, "email", claims.Email)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// InternalServiceMiddleware guards service-to-service routes (e.g. called by
// Zef-zoc-collab) with a shared secret header instead of a user JWT.
func InternalServiceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret := os.Getenv("INTERNAL_SERVICE_SECRET")
		provided := r.Header.Get("X-Internal-Service-Key")
		if secret == "" || provided == "" || subtle.ConstantTimeCompare([]byte(secret), []byte(provided)) != 1 {
			utils.WriteError(w, http.StatusUnauthorized, "Invalid or missing internal service key")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func conditionalLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		middleware.Logger(next).ServeHTTP(w, r)
	})
}
