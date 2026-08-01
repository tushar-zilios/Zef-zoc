package routes

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// VerifyToken (via JWTMiddleware) calls log.Fatal if JWT_SECRET is unset, which
// would kill the whole test binary rather than fail a single test. Make sure a
// secret is present before any test in this package runs.
func init() {
	if os.Getenv("JWT_SECRET") == "" {
		os.Setenv("JWT_SECRET", "test-secret-for-routes-tests")
	}
}

// TestInternalServiceMiddleware exercises the /internal/documents/{id}/ydoc
// route's shared-secret auth, which is what Zef-zoc-collab authenticates
// against via the X-Internal-Service-Key header (see InternalServiceMiddleware
// in middleware.go).
func TestInternalServiceMiddleware(t *testing.T) {
	const secret = "test-internal-secret"
	origSecret := os.Getenv("INTERNAL_SERVICE_SECRET")
	os.Setenv("INTERNAL_SERVICE_SECRET", secret)
	defer os.Setenv("INTERNAL_SERVICE_SECRET", origSecret)

	router := NewRouter()

	t.Run("missing header rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/internal/documents/doc-1/ydoc", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 with missing internal service key, got %d", rec.Code)
		}
	})

	t.Run("wrong header rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/internal/documents/doc-1/ydoc", nil)
		req.Header.Set("X-Internal-Service-Key", "wrong-secret")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 with wrong internal service key, got %d", rec.Code)
		}
	})

	t.Run("correct header accepted", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/internal/documents/doc-1/ydoc", nil)
		req.Header.Set("X-Internal-Service-Key", secret)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		// The handler treats "no stored state yet" (including a DB error when
		// no test DB is configured) as a normal 200 with an empty body, so this
		// path is reachable without a live database — it only proves auth passed.
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 past the auth gate with correct internal service key, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("empty configured secret always rejects", func(t *testing.T) {
		os.Setenv("INTERNAL_SERVICE_SECRET", "")
		defer os.Setenv("INTERNAL_SERVICE_SECRET", secret)

		req := httptest.NewRequest(http.MethodGet, "/internal/documents/doc-1/ydoc", nil)
		req.Header.Set("X-Internal-Service-Key", "")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 when INTERNAL_SERVICE_SECRET is unset, got %d", rec.Code)
		}
	})
}

func TestJWTMiddleware_RejectsMissingOrMalformedAuth(t *testing.T) {
	router := NewRouter()

	t.Run("no authorization header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/folders", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 with no Authorization header, got %d", rec.Code)
		}
	})

	t.Run("malformed authorization header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/folders", nil)
		req.Header.Set("Authorization", "NotBearer sometoken")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 with malformed Authorization header, got %d", rec.Code)
		}
	})

	t.Run("invalid bearer token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/folders", nil)
		req.Header.Set("Authorization", "Bearer not-a-real-jwt")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 with an invalid bearer token, got %d", rec.Code)
		}
	})
}

func TestHealthEndpoint(t *testing.T) {
	router := NewRouter()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected /health to return 200, got %d", rec.Code)
	}
}

func TestSharedRoute_NoAuthRequired(t *testing.T) {
	// /shared/{token} is intentionally unauthenticated (read-only via share
	// token, not a user JWT) — it should never be blocked by JWTMiddleware.
	router := NewRouter()
	req := httptest.NewRequest(http.MethodGet, "/shared/some-token", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("expected /shared/{token} to be reachable without a JWT, got 401")
	}
}
