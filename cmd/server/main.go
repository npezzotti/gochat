package main

import (
	"context"
	"encoding/base64"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/handlers"
	_ "github.com/lib/pq"
	"github.com/npezzotti/go-chatroom/internal/api"
	"github.com/npezzotti/go-chatroom/internal/database"
	"github.com/npezzotti/go-chatroom/internal/server"
)

var (
	addr = flag.String("addr", "localhost:8000", "server address")
)

func decodeSigningSecret() ([]byte, error) {
	return base64.StdEncoding.DecodeString("wT0phFUusHZIrDhL9bUKPUhwaxKhpi/SaI6PtgB+MgU=")
}

func main() {
	logger := log.New(os.Stderr, "", 0)
	flag.Parse()

	var err error
	api.SecretKey, err = decodeSigningSecret()
	if err != nil {
		logger.Fatal("get signing secret: %w", err)
	}

	database.DB, err = database.NewDatabaseConnection("host=localhost user=postgres password=postgres dbname=postgres sslmode=disable")
	if err != nil {
		logger.Fatal("db open:", err)
	}

	defer func() {
		if err := database.DB.Close(); err != nil {
			logger.Fatal("db close:", err)
		}
	}()

	chatServer, err := server.NewChatServer(logger)
	if err != nil {
		logger.Fatal("new chat server:", err)
	}

	go chatServer.Run()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/register", func(w http.ResponseWriter, r *http.Request) {
		api.CreateAccount(logger, w, r)
	})
	mux.HandleFunc("POST /api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		api.Login(logger, w, r)
	})
	mux.HandleFunc("GET /api/auth/session", api.AuthMiddleware(logger, func(w http.ResponseWriter, r *http.Request) {
		api.Session(logger, w, r)
	}))
	mux.Handle("GET /api/auth/logout", api.AuthMiddleware(logger, func(w http.ResponseWriter, r *http.Request) {
		api.Logout(w, r)
	}))
	mux.Handle("/api/account", api.AuthMiddleware(logger, func(w http.ResponseWriter, r *http.Request) {
		api.Account(logger, w, r)
	}))
	mux.Handle("POST /api/rooms", api.AuthMiddleware(logger, func(w http.ResponseWriter, r *http.Request) {
		api.CreateRoom(logger, w, r)
	}))
	mux.Handle("DELETE /api/rooms", api.AuthMiddleware(logger, func(w http.ResponseWriter, r *http.Request) {
		api.DeleteRoom(chatServer, w, r)
	}))
	mux.Handle("GET /api/rooms", api.AuthMiddleware(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.GetRoom(logger, w, r)
	})))
	mux.Handle("GET /api/subscriptions", api.AuthMiddleware(logger, func(w http.ResponseWriter, r *http.Request) {
		api.GetUsersRooms(logger, w, r)
	}))

	mux.Handle("POST /api/subscriptions", api.AuthMiddleware(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.SubscribeRoom(chatServer, w, r)
	})))
	mux.Handle("DELETE /api/subscriptions", api.AuthMiddleware(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.UnsubscribeRoom(chatServer, w, r)
	})))
	mux.Handle("GET /api/messages", api.AuthMiddleware(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.GetMessages(logger, w, r)
	})))

	mux.Handle("/ws", api.AuthMiddleware(logger, func(w http.ResponseWriter, r *http.Request) {
		api.ServeWs(chatServer, w, r)
	}))

	h := handlers.CORS(
		handlers.MaxAge(3600),
		handlers.AllowedOrigins([]string{"http://localhost:8000", "http://localhost:3000"}),
		handlers.AllowedMethods([]string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions}),
		handlers.AllowedHeaders([]string{"Origin", "Content-Type", "Accept"}),
		handlers.AllowCredentials(),
	)(mux)

	h = api.ErrorHandler(logger, h)

	srv := http.Server{
		Addr:    *addr,
		Handler: h,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Printf("starting server on %s\n", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigs:
		logger.Printf("received signal: %s\n", sig)
	case err := <-errCh:
		logger.Println("server:", err)
	}

	logger.Println("stopping server")

	shutDownCtx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := srv.Shutdown(shutDownCtx); err != nil {
		logger.Fatalln("shutdown:", err)
	}
	logger.Println("stopped server")

	logger.Println("shutting down chat server")
	chatServer.Shutdown()

	logger.Println("shutdown complete")
}
