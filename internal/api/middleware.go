package api

import (
	"fmt"
	"net/http"
	"runtime/debug"
)

func (s *GoChatApp) errorHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				var panicError error
				switch e := err.(type) {
				case error:
					panicError = e
				default:
					panicError = fmt.Errorf("%v", e)
				}
				s.log.Printf("panic: %v", panicError)
				debug.PrintStack()
				w.Header().Set("Connection", "close")
				return
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func (s *GoChatApp) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenCookie, err := r.Cookie(tokenCookieKey)
		if err != nil {
			errResp := NewUnauthorizedError()
			s.writeJson(w, errResp.StatusCode, errResp)
			return
		}

		tokenString := tokenCookie.Value
		userId, err := s.extractUserIdFromToken(tokenString)
		if err != nil {
			s.log.Printf("failed to extract user id from token: %v", err)
			errResp := NewUnauthorizedError()
			s.writeJson(w, errResp.StatusCode, errResp)
			return
		}

		ctx := WithUserId(r.Context(), userId)
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")

		next(w, r.WithContext(ctx))
	}
}
