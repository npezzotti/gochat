package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
	"github.com/npezzotti/go-chatroom/internal/database"
	"github.com/npezzotti/go-chatroom/internal/server"
	"github.com/npezzotti/go-chatroom/internal/types"
	"github.com/teris-io/shortid"
)

type Subscription struct {
	Id   int          `json:"id"`
	User User         `json:"user"`
	Room *server.Room `json:"room"`
}

func writeJson(l *log.Logger, w http.ResponseWriter, statusCode int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		l.Printf("JSON encoding error: %v", err)
	}
}

func CreateRoom(l *log.Logger, w http.ResponseWriter, r *http.Request) {
	var params database.CreateRoomParams
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

	newRoom, err := database.DB.CreateRoom(params)
	if err != nil {
		errResp := NewBadRequestError()
		writeJson(l, w, errResp.Code, errResp)
		return
	}

	subs, err := database.DB.GetSubscribersForRoom(newRoom.Id)
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

	room := &server.Room{
		Id:          newRoom.Id,
		ExternalId:  newRoom.ExternalId,
		Name:        newRoom.Name,
		Description: newRoom.Description,
	}

	writeJson(l, w, http.StatusCreated, room)
}

func GetRoom(l *log.Logger, w http.ResponseWriter, r *http.Request) {
	externalId := r.URL.Query().Get("id")
	if externalId == "" {
		errResp := NewBadRequestError()
		writeJson(l, w, errResp.Code, errResp)
		return
	}

	dbRoom, err := database.DB.GetRoomByExternalID(externalId)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(l, w, errResp.Code, errResp)
		return
	}

	dbSubs, err := database.DB.GetSubscribersForRoom(dbRoom.Id)
	var subscribers []types.User
	for _, dbSub := range dbSubs {
		var u types.User
		u.Id = dbSub.Id
		u.Username = dbSub.Username

		subscribers = append(subscribers, u)
	}

	room := &server.Room{
		Id:          dbRoom.Id,
		ExternalId:  dbRoom.ExternalId,
		Name:        dbRoom.Name,
		Description: dbRoom.Description,
		Subscribers: subscribers,
	}

	writeJson(l, w, http.StatusOK, room)
}

func DeleteRoom(cs *server.ChatServer, w http.ResponseWriter, r *http.Request) {
	externalId := r.URL.Query().Get("id")
	if externalId == "" {
		errResp := NewBadRequestError()
		writeJson(cs.Log, w, errResp.Code, errResp)
		return
	}

	room, err := database.DB.GetRoomByExternalID(externalId)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(cs.Log, w, errResp.Code, errResp)
		return
	}

	err = database.DB.DeleteRoom(room.Id)
	if err != nil {
		cs.Log.Println("delete room:", err)
		errResp := NewInternalServerError(err)
		writeJson(cs.Log, w, errResp.Code, errResp)
		return
	}

	cs.RmRoomChan <- room.Id
	writeJson(cs.Log, w, http.StatusNoContent, nil)
}

func GetUsersRooms(l *log.Logger, w http.ResponseWriter, r *http.Request) {
	userId, ok := UserId(r.Context())
	if !ok {
		errResp := NewUnauthorizedError()
		writeJson(l, w, errResp.Code, errResp)
		return
	}

	dbRooms, err := database.DB.ListSubscriptions(userId)
	if err != nil {
		l.Println("list subscriptions:", err)
		errResp := NewInternalServerError(err)
		writeJson(l, w, errResp.Code, errResp)
		return
	}

	var rooms []server.Room
	for _, dbRoom := range dbRooms {
		rooms = append(rooms, server.Room{
			Id:          dbRoom.Id,
			ExternalId:  dbRoom.ExternalId,
			Name:        dbRoom.Name,
			Description: dbRoom.Description,
		})
	}

	writeJson(l, w, http.StatusOK, rooms)
}

func SubscribeRoom(cs *server.ChatServer, w http.ResponseWriter, r *http.Request) {
	roomExternalId := r.URL.Query().Get("room_id")
	if roomExternalId == "" {
		errResp := NewBadRequestError()
		writeJson(cs.Log, w, errResp.Code, errResp)
		return
	}

	userId, ok := UserId(r.Context())
	if !ok {
		errResp := NewUnauthorizedError()
		writeJson(cs.Log, w, errResp.Code, errResp)
		return
	}

	user, err := database.DB.GetAccount(userId)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(cs.Log, w, errResp.Code, errResp)
		return
	}

	room, err := database.DB.GetRoomByExternalID(roomExternalId)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(cs.Log, w, errResp.Code, errResp)
		return
	}

	dbSub, err := database.DB.CreateSubscription(user.Id, room.Id)
	if err != nil {
		errResp := NewBadRequestError()
		writeJson(cs.Log, w, errResp.Code, errResp)
		return
	}

	cs.SubChan <- server.SubReq{
		SubType: server.SubReqTypeSubscribe,
		User:    types.User{Id: user.Id, Username: user.Username},
		RoomId:  room.Id,
	}

	dbSubs, err := database.DB.GetSubscribersForRoom(room.Id)
	if err != nil {
		errResp := NewInternalServerError(err)
		writeJson(cs.Log, w, errResp.Code, errResp)
		return
	}

	var subscribers []types.User
	for _, dbSub := range dbSubs {
		var u types.User
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
		Room: &server.Room{
			Id:          room.Id,
			ExternalId:  room.ExternalId,
			Name:        room.Name,
			Description: room.Description,
		},
	}

	writeJson(cs.Log, w, http.StatusCreated, sub)
}

func UnsubscribeRoom(cs *server.ChatServer, w http.ResponseWriter, r *http.Request) {
	externalId := r.URL.Query().Get("room_id")
	if externalId == "" {
		errResp := NewBadRequestError()
		writeJson(cs.Log, w, errResp.Code, errResp)
		return
	}

	userId, ok := UserId(r.Context())
	if !ok {
		errResp := NewUnauthorizedError()
		writeJson(cs.Log, w, errResp.Code, errResp)
		return
	}

	user, err := database.DB.GetAccount(userId)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(cs.Log, w, errResp.Code, errResp)
		return
	}

	room, err := database.DB.GetRoomByExternalID(externalId)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(cs.Log, w, errResp.Code, errResp)
		return
	}

	err = database.DB.DeleteSubscription(userId, room.Id)
	if err != nil {
		cs.Log.Println("delete subscription:", err)
		errResp := NewInternalServerError(err)
		writeJson(cs.Log, w, errResp.Code, errResp)
		return
	}

	cs.SubChan <- server.SubReq{
		SubType: server.SubReqTypeUnsubscribe,
		User:    types.User{Id: user.Id, Username: user.Username},
		RoomId:  room.Id,
	}

	writeJson(cs.Log, w, http.StatusNoContent, nil)
}

func GetMessages(l *log.Logger, w http.ResponseWriter, r *http.Request) {
	externalId := r.URL.Query().Get("room_id")
	if externalId == "" {
		errResp := NewBadRequestError()
		writeJson(l, w, errResp.Code, errResp)
		return
	}

	room, err := database.DB.GetRoomByExternalID(externalId)
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

	messages, err := database.DB.MessageGetAll(room.Id, after, before, limit)
	if err != nil {
		errResp := NewInternalServerError(err)
		writeJson(l, w, errResp.Code, errResp)
		return
	}

	var userMessages []server.Message

	for _, msg := range messages {
		msg := server.Message{
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

func ServeWs(chatServer *server.ChatServer, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	username, ok := UserId(r.Context())
	if !ok {
		errResp := NewUnauthorizedError()
		writeJson(chatServer.Log, w, errResp.Code, errResp)
		return
	}

	user, err := database.DB.GetAccount(username)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(chatServer.Log, w, errResp.Code, errResp)
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		chatServer.Log.Println("error upgrading connection:", err)
		return
	}

	client := server.NewClient(types.User{
		Id:           user.Id,
		Username:     user.Username,
		EmailAddress: user.EmailAddress,
	}, conn, chatServer, chatServer.Log)
	chatServer.RegisterChan <- client

	go client.Write()
	go client.Read()
}
