package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"text/template"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/websocket"
	_ "github.com/lib/pq"
)

var (
	DB        *sql.DB
	tc        map[string]*template.Template
	secretKey []byte

	addr = flag.String("addr", "localhost:8000", "server address")
)

func serveHome(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	if err := render(w, "index.html.tmpl"); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

func createRoom(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var params CreateRoomParams
	if err := json.Unmarshal(body, &params); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userId, ok := r.Context().Value(userIdKey).(int)
	if !ok {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	params.OwnerId = userId

	newRoom, err := CreateRoom(params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)

	resp, err := json.Marshal(newRoom)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(resp)
}

func serveWs(chatServer *ChatServer, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	username, ok := r.Context().Value(userIdKey).(int)
	if !ok {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	user, err := GetAccount(username)
	if err != nil {
		chatServer.log.Println(err)
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	upgrader := websocket.Upgrader{}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}

	client := NewClient(user, conn, chatServer, chatServer.log)
	chatServer.registerChan <- client

	go client.write()
	go client.read()
}

func main() {
	logger := log.New(os.Stderr, "", 0)
	flag.Parse()

	var err error
	tc, err = NewTemplateCache()
	if err != nil {
		logger.Fatalln("unable to create template cache:", err)
	}

	secretKey, err = decodeSigningSecret()
	if err != nil {
		logger.Fatal("get signing secret: %w", err)
	}

	DB, err = sql.Open("postgres", "host=localhost user=postgres password=postgres dbname=postgres sslmode=disable")
	if err != nil {
		logger.Fatal("db open:", err)
	}

	if err := DB.Ping(); err != nil {
		logger.Fatal("db ping:", err)
	}

	defer func() {
		if err := DB.Close(); err != nil {
			logger.Fatal("db close:", err)
		}
	}()

	chatServer := NewChatServer(logger)
	go chatServer.run()

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	mux.HandleFunc("/", authMiddleware(logger, http.HandlerFunc(serveHome)))
	mux.HandleFunc("/account/new", func(w http.ResponseWriter, r *http.Request) {
		createAccount(logger, w, r)
	})

	mux.Handle("/account/edit", authMiddleware(logger, func(w http.ResponseWriter, r *http.Request) {
		editAccount(logger, w, r)
	}))

	mux.Handle("POST /room/new", authMiddleware(logger, func(w http.ResponseWriter, r *http.Request) {
		createRoom(w, r)
	}))

	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		login(logger, w, r)
	})

	mux.Handle("/logout", authMiddleware(logger, func(w http.ResponseWriter, r *http.Request) {
		logout(w, r)
	}))

	mux.Handle("/ws", authMiddleware(logger, func(w http.ResponseWriter, r *http.Request) {
		serveWs(chatServer, w, r)
	}))

	h := handlers.CORS(
		handlers.MaxAge(3600),
		handlers.AllowedOrigins([]string{"http://localhost:8000"}),
		handlers.AllowedMethods([]string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions}),
		handlers.AllowedHeaders([]string{"Origin", "Content-Type", "Accept"}),
		handlers.AllowCredentials(),
	)(mux)

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
	chatServer.shutdown()

	logger.Println("shutdown complete")
}
