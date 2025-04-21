package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
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
	"github.com/teris-io/shortid"
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

func writeJson(l *log.Logger, w http.ResponseWriter, statusCode int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		l.Printf("JSON encoding error: %v", err)
	}
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

func createRoom(l *log.Logger, w http.ResponseWriter, r *http.Request) {
	var params CreateRoomParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		errResp := NewBadRequestError()
		writeJson(l, w, errResp.Code, errResp)
		return
	}

	userId, ok := UserId(r.Context())
	if !ok {
		errResp := NewUnauthorizedError()
		writeJson(l, w, errResp.Code, errResp)
		return
	}

	sid, err := shortid.Generate()
	if err != nil {
		l.Print("generate shortid:", err)
		errResp := NewInternalServerError(err)
		writeJson(l, w, errResp.Code, errResp)
		return
	}

	params.ExternalId = sid
	params.OwnerId = userId

	newRoom, err := CreateRoom(params)
	if err != nil {
		errResp := NewBadRequestError()
		writeJson(l, w, errResp.Code, errResp)
		return
	}

	subs, err := GetSubscribersForRoom(newRoom.Id)
	if err != nil {
		errResp := NewInternalServerError(err)
		writeJson(l, w, errResp.Code, errResp)
		return
	}

	var subscribers []User
	for _, dbSub := range subs {
		var u User
		u.Id = dbSub.Id
		u.Username = dbSub.Username
		subscribers = append(subscribers, u)
	}

	room := &Room{
		Id:          newRoom.Id,
		ExternalId:  newRoom.ExternalId,
		Name:        newRoom.Name,
		Description: newRoom.Description,
		Subscribers: subscribers,
	}

	writeJson(l, w, http.StatusCreated, room)
}

func getRoom(l *log.Logger, w http.ResponseWriter, r *http.Request) {
	externalId := r.URL.Query().Get("id")
	if externalId == "" {
		errResp := NewBadRequestError()
		writeJson(l, w, errResp.Code, errResp)
		return
	}

	dbRoom, err := GetRoomByExternalID(externalId)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(l, w, errResp.Code, errResp)
		return
	}

	dbSubs, err := GetSubscribersForRoom(dbRoom.Id)
	var subscribers []User
	for _, dbSub := range dbSubs {
		var u User
		u.Id = dbSub.Id
		u.Username = dbSub.Username

		subscribers = append(subscribers, u)
	}

	room := &Room{
		Id:          dbRoom.Id,
		ExternalId:  dbRoom.ExternalId,
		Name:        dbRoom.Name,
		Description: dbRoom.Description,
		Subscribers: subscribers,
	}

	writeJson(l, w, http.StatusOK, room)
}

func deleteRoom(cs *ChatServer, w http.ResponseWriter, r *http.Request) {
	externalId := r.URL.Query().Get("id")
	if externalId == "" {
		errResp := NewBadRequestError()
		writeJson(cs.log, w, errResp.Code, errResp)
		return
	}

	room, err := GetRoomByExternalID(externalId)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(cs.log, w, errResp.Code, errResp)
		return
	}

	err = DeleteRoom(room.Id)
	if err != nil {
		cs.log.Println("delete room:", err)
		errResp := NewInternalServerError(err)
		writeJson(cs.log, w, errResp.Code, errResp)
		return
	}

	cs.rmRoomChan <- room.Id
	writeJson(cs.log, w, http.StatusNoContent, nil)
}

func getUsersRooms(l *log.Logger, w http.ResponseWriter, r *http.Request) {
	userId, ok := UserId(r.Context())
	if !ok {
		errResp := NewUnauthorizedError()
		writeJson(l, w, errResp.Code, errResp)
		return
	}

	dbRooms, err := ListSubscriptions(userId)
	if err != nil {
		l.Println("list subscriptions:", err)
		errResp := NewInternalServerError(err)
		writeJson(l, w, errResp.Code, errResp)
		return
	}

	var rooms []Room
	for _, dbRoom := range dbRooms {
		rooms = append(rooms, Room{
			Id:          dbRoom.Id,
			ExternalId:  dbRoom.ExternalId,
			Name:        dbRoom.Name,
			Description: dbRoom.Description,
		})
	}

	writeJson(l, w, http.StatusOK, rooms)
}

func (cs *ChatServer) subscribeRoom(w http.ResponseWriter, r *http.Request) {
	roomExternalId := r.URL.Query().Get("room_id")
	if roomExternalId == "" {
		errResp := NewBadRequestError()
		writeJson(cs.log, w, errResp.Code, errResp)
		return
	}

	userId, ok := UserId(r.Context())
	if !ok {
		errResp := NewUnauthorizedError()
		writeJson(cs.log, w, errResp.Code, errResp)
		return
	}

	user, err := GetAccount(userId)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(cs.log, w, errResp.Code, errResp)
		return
	}

	room, err := GetRoomByExternalID(roomExternalId)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(cs.log, w, errResp.Code, errResp)
		return
	}

	dbSub, err := CreateSubscription(user.Id, room.Id)
	if err != nil {
		errResp := NewBadRequestError()
		writeJson(cs.log, w, errResp.Code, errResp)
		return
	}

	cs.subChan <- subReq{
		subType: subReqTypeSubscribe,
		user:    User{Id: user.Id, Username: user.Username},
		roomId:  room.Id,
	}

	dbSubs, err := GetSubscribersForRoom(room.Id)
	if err != nil {
		errResp := NewInternalServerError(err)
		writeJson(cs.log, w, errResp.Code, errResp)
		return
	}

	var subscribers []User
	for _, dbSub := range dbSubs {
		var u User
		u.Id = dbSub.Id
		u.Username = dbSub.Username

		subscribers = append(subscribers, u)
	}

	sub := Subscription{
		Id: dbSub.Id,
		User: User{
			Id:           user.Id,
			Username:     user.Username,
			EmailAddress: user.EmailAddress,
		},
		Room: &Room{
			Id:          room.Id,
			ExternalId:  room.ExternalId,
			Name:        room.Name,
			Description: room.Description,
			Subscribers: subscribers,
		},
	}

	writeJson(cs.log, w, http.StatusCreated, sub)
}

func (cs *ChatServer) unsubscribeRoom(w http.ResponseWriter, r *http.Request) {
	externalId := r.URL.Query().Get("room_id")
	if externalId == "" {
		errResp := NewBadRequestError()
		writeJson(cs.log, w, errResp.Code, errResp)
		return
	}

	userId, ok := UserId(r.Context())
	if !ok {
		errResp := NewUnauthorizedError()
		writeJson(cs.log, w, errResp.Code, errResp)
		return
	}

	user, err := GetAccount(userId)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(cs.log, w, errResp.Code, errResp)
		return
	}

	room, err := GetRoomByExternalID(externalId)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(cs.log, w, errResp.Code, errResp)
		return
	}

	err = DeleteSubscription(userId, room.Id)
	if err != nil {
		cs.log.Println("delete subscription:", err)
		errResp := NewInternalServerError(err)
		writeJson(cs.log, w, errResp.Code, errResp)
		return
	}

	cs.subChan <- subReq{
		subType: subReqTypeUnsubscribe,
		user:    User{Id: user.Id, Username: user.Username},
		roomId:  room.Id,
	}

	writeJson(cs.log, w, http.StatusNoContent, nil)
}

func getMessages(l *log.Logger, w http.ResponseWriter, r *http.Request) {
	externalId := r.URL.Query().Get("room_id")
	if externalId == "" {
		errResp := NewBadRequestError()
		writeJson(l, w, errResp.Code, errResp)
		return
	}

	room, err := GetRoomByExternalID(externalId)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(l, w, errResp.Code, errResp)
		return
	}

	var before, after, limit int

	beforeStr := r.URL.Query().Get("before")
	if beforeStr != "" {
		before, err = strconv.Atoi(beforeStr)
		if err != nil {
			errResp := NewBadRequestError()
			writeJson(l, w, errResp.Code, errResp)
			return
		}
	}

	afterStr := r.URL.Query().Get("after")
	if afterStr != "" {
		after, err = strconv.Atoi(afterStr)
		if err != nil {
			errResp := NewBadRequestError()
			writeJson(l, w, errResp.Code, errResp)
			return
		}
	}

	limitStr := r.URL.Query().Get("limit")
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			errResp := NewBadRequestError()
			writeJson(l, w, errResp.Code, errResp)
			return
		}
	}

	messages, err := MessageGetAll(room.Id, after, before, limit)
	if err != nil {
		errResp := NewInternalServerError(err)
		writeJson(l, w, errResp.Code, errResp)
		return
	}

	var userMessages []Message

	for _, msg := range messages {
		msg := Message{
			Id:        msg.Id,
			SeqId:     msg.SeqId,
			UserId:    msg.UserId,
			RoomId:    msg.RoomId,
			Content:   msg.Content,
			Timestamp: msg.CreatedAt,
		}

		userMessages = append(userMessages, msg)
	}

	writeJson(l, w, http.StatusOK, userMessages)
}

func serveWs(chatServer *ChatServer, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	username, ok := UserId(r.Context())
	if !ok {
		errResp := NewUnauthorizedError()
		writeJson(chatServer.log, w, errResp.Code, errResp)
		return
	}

	user, err := GetAccount(username)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(chatServer.log, w, errResp.Code, errResp)
		return
	}

	upgrader := websocket.Upgrader{}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := NewClient(User{
		Id:           user.Id,
		Username:     user.Username,
		EmailAddress: user.EmailAddress,
	}, conn, chatServer, chatServer.log)
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

	chatServer, err := NewChatServer(logger)
	if err != nil {
		logger.Fatal("new chat server:", err)
	}
	go chatServer.run()

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	mux.HandleFunc("/", authMiddleware(logger, http.HandlerFunc(serveHome)))
	mux.HandleFunc("/account/new", func(w http.ResponseWriter, r *http.Request) {
		createAccount(logger, w, r)
	})

	mux.Handle("/account", authMiddleware(logger, func(w http.ResponseWriter, r *http.Request) {
		account(logger, w, r)
	}))

	mux.Handle("POST /rooms", authMiddleware(logger, func(w http.ResponseWriter, r *http.Request) {
		createRoom(logger, w, r)
	}))

	mux.Handle("DELETE /rooms", authMiddleware(logger, func(w http.ResponseWriter, r *http.Request) {
		deleteRoom(chatServer, w, r)
	}))

	mux.Handle("GET /rooms", authMiddleware(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		getRoom(logger, w, r)
	})))
	mux.Handle("GET /subscriptions", authMiddleware(logger, func(w http.ResponseWriter, r *http.Request) {
		getUsersRooms(logger, w, r)
	}))

	mux.Handle("POST /subscriptions", authMiddleware(logger, http.HandlerFunc(chatServer.subscribeRoom)))
	mux.Handle("DELETE /subscriptions", authMiddleware(logger, http.HandlerFunc(chatServer.unsubscribeRoom)))
	mux.Handle("GET /messages", authMiddleware(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		getMessages(logger, w, r)
	})))
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

	h = ErrorHandler(logger, h)

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
