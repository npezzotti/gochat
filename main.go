package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
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

func UserId(ctx context.Context) (int, bool) {
	userId, ok := ctx.Value(userIdKey).(int)

	return userId, ok
}

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
		http.Error(w, "read: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var params CreateRoomParams
	if err := json.Unmarshal(body, &params); err != nil {
		http.Error(w, "unmarshal json: "+err.Error(), http.StatusBadRequest)
		return
	}

	userId, ok := UserId(r.Context())
	if !ok {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	params.OwnerId = userId

	newRoom, err := CreateRoom(params)
	if err != nil {
		http.Error(w, "CreateRoom: "+err.Error(), http.StatusBadRequest)
		return
	}

	room := &Room{
		Id:          newRoom.Id,
		Name:        newRoom.Name,
		Description: newRoom.Description,
	}

	w.WriteHeader(http.StatusCreated)

	resp, err := json.Marshal(room)
	if err != nil {
		http.Error(w, "marshal json: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(resp)
}

func getRoom(w http.ResponseWriter, r *http.Request) {
	roomIdStr := r.URL.Query().Get("id")
	if roomIdStr == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	roomId, err := strconv.Atoi(roomIdStr)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	dbRoom, err := GetRoomById(roomId)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	dbSubs, err := GetSubscribersForRoom(roomId)
	var subscribers []User
	for _, dbSub := range dbSubs {
		var u User
		u.Id = dbSub.Id
		u.Username = dbSub.Username

		subscribers = append(subscribers, u)
	}

	room := &Room{
		Id:          dbRoom.Id,
		Name:        dbRoom.Name,
		Description: dbRoom.Description,
		Subscribers: subscribers,
	}

	roomResp, err := json.Marshal(room)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Write(roomResp)
}

func deleteRoom(cs *ChatServer, w http.ResponseWriter, r *http.Request) {
	roomIdStr := r.URL.Query().Get("id")
	if roomIdStr == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	roomId, err := strconv.Atoi(roomIdStr)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	err = DeleteRoom(roomId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Println("deleted the room")
	cs.rmRoom <- roomId

	fmt.Println("done")

	w.WriteHeader(http.StatusNoContent)
}

func getUsersRooms(w http.ResponseWriter, r *http.Request) {
	userId, ok := UserId(r.Context())
	if !ok {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	dbRooms, err := ListSubscriptions(userId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var rooms []Room
	for _, dbRoom := range dbRooms {
		rooms = append(rooms, Room{
			Id:          dbRoom.Id,
			Name:        dbRoom.Name,
			Description: dbRoom.Description,
		})
	}

	resp, err := json.Marshal(rooms)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(resp)
}

func subscribeRoom(w http.ResponseWriter, r *http.Request) {
	roomIdStr := r.URL.Query().Get("room_id")
	if roomIdStr == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	roomId, err := strconv.Atoi(roomIdStr)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	userId, ok := UserId(r.Context())
	if !ok {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	user, err := GetAccount(userId)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	dbRoom, err := GetRoomById(roomId)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	dbSub, err := CreateSubscription(user.Id, dbRoom.Id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dbSubs, err := GetSubscribersForRoom(roomId)
	var subscribers []User
	for _, dbSub := range dbSubs {
		var u User
		u.Id = dbSub.Id
		u.Username = dbSub.Username

		subscribers = append(subscribers, u)
	}

	sub := Subscription{
		Id:   dbSub.Id,
		User: user,
		Room: &Room{
			Id:          dbRoom.Id,
			Name:        dbRoom.Name,
			Description: dbRoom.Description,
			Subscribers: subscribers,
		},
	}

	resp, err := json.Marshal(sub)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write(resp)
}

func (cs *ChatServer) unsubscribeRoom(w http.ResponseWriter, r *http.Request) {
	roomIdStr := r.URL.Query().Get("room_id")
	if roomIdStr == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	roomId, err := strconv.Atoi(roomIdStr)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	userId, ok := UserId(r.Context())
	if !ok {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	err = DeleteSubscription(userId, roomId)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func getMessages(w http.ResponseWriter, r *http.Request) {
	roomIdStr := r.URL.Query().Get("room_id")
	if roomIdStr == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	roomId, err := strconv.Atoi(roomIdStr)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	var before, after int

	beforeStr := r.URL.Query().Get("before")
	if beforeStr != "" {
		before, err = strconv.Atoi(beforeStr)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	afterStr := r.URL.Query().Get("after")
	if afterStr != "" {
		after, err = strconv.Atoi(afterStr)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	messages, err := MessageGetAll(roomId, before, after, 10)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	var userMessages []Message

	for _, msg := range messages {
		msg := Message{
			Id:      msg.Id,
			SeqId:   msg.SeqId,
			UserId:  msg.UserId,
			Content: msg.Content,
		}

		userMessages = append(userMessages, msg)
	}

	messagesResp, err := json.Marshal(userMessages)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Write(messagesResp)
}

func serveWs(chatServer *ChatServer, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	username, ok := UserId(r.Context())
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

	mux.Handle("GET /room/delete", authMiddleware(logger, func(w http.ResponseWriter, r *http.Request) {
		deleteRoom(chatServer, w, r)
	}))

	mux.Handle("GET /room", authMiddleware(logger, http.HandlerFunc(getRoom)))
	mux.Handle("GET /subscriptions", authMiddleware(logger, func(w http.ResponseWriter, r *http.Request) {
		getUsersRooms(w, r)
	}))

	mux.Handle("POST /subscriptions", authMiddleware(logger, http.HandlerFunc(subscribeRoom)))
	mux.Handle("DELETE /subscriptions", authMiddleware(logger, http.HandlerFunc(chatServer.unsubscribeRoom)))
	mux.Handle("GET /messages", authMiddleware(logger, http.HandlerFunc(getMessages)))
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
