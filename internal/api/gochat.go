package api

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/handlers"
	"github.com/npezzotti/go-chatroom/internal/config"
	"github.com/npezzotti/go-chatroom/internal/database"
	"github.com/npezzotti/go-chatroom/internal/server"
)

type GoChatApp struct {
	log        *log.Logger
	db         database.GoChatRepository
	mux        *http.Server
	cs         *server.ChatServer
	signingKey []byte
}

func NewGoChatApp(logger *log.Logger, cs *server.ChatServer, db *database.PgGoChatRepository, cfg *config.Config) *GoChatApp {
	s := &GoChatApp{
		log:        logger,
		db:         db,
		cs:         cs,
		signingKey: cfg.SigningKey,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/register", s.createAccount)
	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.HandleFunc("GET /api/auth/session", s.authMiddleware(s.session))
	mux.Handle("GET /api/auth/logout", s.authMiddleware(s.logout))
	mux.Handle("/api/account", s.authMiddleware(s.account))
	mux.Handle("POST /api/rooms", s.authMiddleware(s.createRoom))
	mux.Handle("DELETE /api/rooms", s.authMiddleware(s.deleteRoom))
	mux.Handle("GET /api/rooms", s.authMiddleware(s.getRoom))
	mux.Handle("GET /api/subscriptions", s.authMiddleware(s.getUsersSubscriptions))
	mux.Handle("GET /api/messages", s.authMiddleware(s.getMessages))
	mux.Handle("GET /ws", s.authMiddleware(s.serveWs))

	h := handlers.CORS(
		handlers.MaxAge(3600),
		handlers.AllowedOrigins(cfg.AllowedOrigins),
		handlers.AllowedMethods([]string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions}),
		handlers.AllowedHeaders([]string{"Origin", "Content-Type", "Accept"}),
		handlers.AllowCredentials(),
	)(mux)

	h = s.errorHandler(h)

	srv := &http.Server{
		Addr:    cfg.ServerAddr,
		Handler: h,
	}

	s.mux = srv
	return s
}

func (s *GoChatApp) Start() error {
	s.log.Printf("starting server on %s\n", s.mux.Addr)
	return s.mux.ListenAndServe()
}

func (s *GoChatApp) Shutdown(ctx context.Context) error {
	s.log.Println("shutting down HTTP server...")
	if err := s.mux.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	return nil
}
