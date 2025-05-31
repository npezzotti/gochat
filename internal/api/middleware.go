package api

import (
	"encoding/json"
	"log"
	"net/http"
	"runtime/debug"
)

func ErrorHandler(log *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Println("panic:", err)
				debug.PrintStack()

				errResp := NewInternalServerError(nil)
				jsonResp, err := json.Marshal(errResp)
				if err != nil {
					log.Println("Error marshalling JSON:", err)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}

				w.Header().Set("Connection", "close")
				w.WriteHeader(errResp.Code)
				w.Write(jsonResp)
				return
			}
		}()

		next.ServeHTTP(w, r)
	})
}
