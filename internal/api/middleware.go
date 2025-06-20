package api

import (
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
