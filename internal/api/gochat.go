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
	"github.com/npezzotti/go-chatroom/internal/stats"
	"github.com/teris-io/shortid"
)

type GoChatApp struct {
	log             *log.Logger
	db              database.GoChatRepository
	mux             *http.Server
	cs              *server.ChatServer
	signingKey      []byte
	generateShortId func() (string, error)
	allowedOrigins  []string
	stats           stats.StatsProvider
}

func NewGoChatApp(mux *http.ServeMux, logger *log.Logger, cs *server.ChatServer, db database.GoChatRepository, stats stats.StatsProvider, cfg *config.Config) *GoChatApp {
	app := &GoChatApp{
		log:             logger,
		db:              db,
		cs:              cs,
		signingKey:      cfg.SigningKey,
		generateShortId: defaultGenerateShortId,
		allowedOrigins:  cfg.AllowedOrigins,
		stats:           stats,
	}

	fs := http.FileServer(http.Dir("./frontend/build"))
	mux.Handle("/", fs)

	mux.HandleFunc("GET /healthz", app.healthCheck)
	mux.HandleFunc("POST /api/auth/register", app.createAccount)
	mux.HandleFunc("POST /api/auth/login", app.login)
	mux.HandleFunc("GET /api/auth/session", app.authMiddleware(app.session))
	mux.Handle("GET /api/auth/logout", app.authMiddleware(app.logout))
	mux.Handle("/api/account", app.authMiddleware(app.account))
	mux.Handle("POST /api/rooms", app.authMiddleware(app.createRoom))
	mux.Handle("DELETE /api/rooms", app.authMiddleware(app.deleteRoom))
	mux.Handle("GET /api/subscriptions", app.authMiddleware(app.getUsersSubscriptions))
	mux.Handle("GET /api/messages", app.authMiddleware(app.getMessages))
	mux.Handle("GET /ws", app.authMiddleware(app.serveWs))

	h := handlers.CORS(
		handlers.MaxAge(3600),
		handlers.AllowedOrigins(app.allowedOrigins),
		handlers.AllowedMethods([]string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions}),
		handlers.AllowedHeaders([]string{"Origin", "Content-Type", "Accept"}),
		handlers.AllowCredentials(),
	)(mux)

	h = app.errorHandler(h)

	srv := &http.Server{
		Addr:    cfg.ServerAddr,
		Handler: h,
	}
	app.mux = srv

	return app
}

func (s *GoChatApp) Start() error {
	s.log.Printf("server running at %s\n", "http://"+s.mux.Addr)
	return s.mux.ListenAndServe()
}

func (s *GoChatApp) Shutdown(ctx context.Context) error {
	s.log.Println("shutting down api server...")
	if err := s.mux.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	return nil
}

func defaultGenerateShortId() (string, error) {
	return shortid.Generate()
}
