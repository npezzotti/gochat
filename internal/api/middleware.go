package api

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime/debug"
)

func (s *GoChatApp) errorHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				s.log.Println("panic:", err)
				debug.PrintStack()

				errResp := NewInternalServerError(nil)
				jsonResp, err := json.Marshal(errResp)
				if err != nil {
					s.log.Println("Error marshalling JSON:", err)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}

				w.Header().Set("Connection", "close")
				w.WriteHeader(errResp.StatusCode)
				w.Write(jsonResp)
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
			s.log.Println("failed to extract user id from token:", err)
			errResp := NewUnauthorizedError()
			s.writeJson(w, errResp.StatusCode, errResp)
			return
		}

		ctx := context.WithValue(r.Context(), userIdKey, userId)
		w.Header().Add("Cache-Control", "no-store, no-cache, must-revalidate, private")

		next(w, r.WithContext(ctx))
	}
}
