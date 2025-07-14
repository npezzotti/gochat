package api

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/npezzotti/go-chatroom/internal/config"
	"github.com/npezzotti/go-chatroom/internal/testutil"
	"github.com/npezzotti/go-chatroom/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestErrorHandler_PanicRecovery(t *testing.T) {
	buf := &bytes.Buffer{}
	app := &GoChatApp{
		log: testutil.TestLogger(t),
	}

	app.log.SetOutput(buf)

	// handler that panics
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(errors.New("test panic"))
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	handler := app.errorHandler(panicHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, "close", rr.Header().Get("Connection"))
	assert.Contains(t, buf.String(), "panic: test panic")
}

func Test_errorHandler_NoPanic(t *testing.T) {
	app := &GoChatApp{}

	// simple handler that does not panic
	called := false
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	handler := app.errorHandler(okHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "ok", rr.Body.String())
	assert.True(t, called, "expected handler to be called")
}

func Test_authMiddleware_ValidToken(t *testing.T) {
	app := NewGoChatApp(
		http.NewServeMux(),
		testutil.TestLogger(t),
		nil,
		nil,
		nil,
		&config.Config{
			SigningKey: []byte("test-signing-key"),
		},
	)

	buf := &bytes.Buffer{}
	app.log.SetOutput(buf)

	tokenHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := UserId(r.Context())
		if !ok {
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	t.Run("valid token", func(t *testing.T) {
		now := time.Now().UTC()
		u := types.User{
			Id:           1,
			Username:     "test",
			EmailAddress: "test@example.com	",
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		token, err := app.createJwtForSession(u, defaultJwtExpiration)
		if err != nil {
			t.Fatalf("failed to create jwt token: %v", err)
		}

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		tokenCookie := createJwtCookie(token, defaultJwtExpiration)
		req.AddCookie(tokenCookie)
		handler := app.authMiddleware(tokenHandler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "ok", rr.Body.String())
		assert.Equal(t, "no-store, no-cache, must-revalidate, private", rr.Header().Get("Cache-Control"))
	})

	t.Run("missing token", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		handler := app.authMiddleware(tokenHandler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("invalid token", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		// Add an invalid token cookie
		req.AddCookie(&http.Cookie{
			Name:  tokenCookieKey,
			Value: "invalid-token",
		})
		handler := app.authMiddleware(tokenHandler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.Contains(t, buf.String(), "failed to extract user id from token")
	})
}
