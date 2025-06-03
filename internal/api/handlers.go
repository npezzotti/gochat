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

func writeJson(l *log.Logger, w http.ResponseWriter, statusCode int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		l.Printf("JSON encoding error: %v", err)
	}
}

func (s *Server) createRoom(w http.ResponseWriter, r *http.Request) {
	var params database.CreateRoomParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		errResp := NewBadRequestError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	userId, ok := UserId(r.Context())
	if !ok {
		errResp := NewUnauthorizedError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	sid, err := shortid.Generate()
	if err != nil {
		s.log.Print("generate shortid:", err)
		errResp := NewInternalServerError(err)
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	params.ExternalId = sid
	params.OwnerId = userId

	newRoom, err := s.db.CreateRoom(params)
	if err != nil {
		errResp := NewBadRequestError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	room := &types.Room{
		Id:          newRoom.Id,
		ExternalId:  newRoom.ExternalId,
		Name:        newRoom.Name,
		Description: newRoom.Description,
		CreatedAt:   newRoom.CreatedAt,
		UpdatedAt:   newRoom.UpdatedAt,
	}

	writeJson(s.log, w, http.StatusCreated, room)
}

func (s *Server) getRoom(w http.ResponseWriter, r *http.Request) {
	externalId := r.URL.Query().Get("id")
	if externalId == "" {
		errResp := NewBadRequestError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	dbRoom, err := s.db.GetRoomByExternalID(externalId)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	dbSubs, err := s.db.GetSubscribersForRoom(dbRoom.Id)
	var subscribers []types.User
	for _, dbSub := range dbSubs {
		var u types.User
		u.Id = dbSub.Id
		u.Username = dbSub.Username

		subscribers = append(subscribers, u)
	}

	room := &types.Room{
		Id:          dbRoom.Id,
		ExternalId:  dbRoom.ExternalId,
		Name:        dbRoom.Name,
		Description: dbRoom.Description,
		Subscribers: subscribers,
		CreatedAt:   dbRoom.CreatedAt,
		UpdatedAt:   dbRoom.UpdatedAt,
	}

	writeJson(s.log, w, http.StatusOK, room)
}

func (s *Server) deleteRoom(w http.ResponseWriter, r *http.Request) {
	externalId := r.URL.Query().Get("id")
	if externalId == "" {
		errResp := NewBadRequestError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	room, err := s.db.GetRoomByExternalID(externalId)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	err = s.db.DeleteRoom(room.Id)
	if err != nil {
		s.log.Println("delete room:", err)
		errResp := NewInternalServerError(err)
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	s.cs.RmRoomChan <- room.Id
	writeJson(s.log, w, http.StatusNoContent, nil)
}

func (s *Server) getUsersRooms(w http.ResponseWriter, r *http.Request) {
	userId, ok := UserId(r.Context())
	if !ok {
		errResp := NewUnauthorizedError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	dbRooms, err := s.db.ListSubscriptions(userId)
	if err != nil {
		s.log.Println("list subscriptions:", err)
		errResp := NewInternalServerError(err)
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	var rooms []types.Room
	for _, dbRoom := range dbRooms {
		rooms = append(rooms, types.Room{
			Id:          dbRoom.Id,
			ExternalId:  dbRoom.ExternalId,
			Name:        dbRoom.Name,
			Description: dbRoom.Description,
			CreatedAt:   dbRoom.CreatedAt,
			UpdatedAt:   dbRoom.UpdatedAt,
		})
	}

	writeJson(s.log, w, http.StatusOK, rooms)
}

func (s *Server) subscribeRoom(w http.ResponseWriter, r *http.Request) {
	roomExternalId := r.URL.Query().Get("room_id")
	if roomExternalId == "" {
		errResp := NewBadRequestError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	userId, ok := UserId(r.Context())
	if !ok {
		errResp := NewUnauthorizedError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	user, err := s.db.GetAccount(userId)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	room, err := s.db.GetRoomByExternalID(roomExternalId)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	dbSub, err := s.db.CreateSubscription(user.Id, room.Id)
	if err != nil {
		errResp := NewBadRequestError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	s.cs.SubChan <- server.SubReq{
		SubType: server.SubReqTypeSubscribe,
		User:    types.User{Id: user.Id, Username: user.Username},
		RoomId:  room.Id,
	}

	sub := types.Subscription{
		Id: dbSub.Id,
		User: types.User{
			Id:           user.Id,
			Username:     user.Username,
			EmailAddress: user.EmailAddress,
			CreatedAt:    user.CreatedAt,
			UpdatedAt:    user.UpdatedAt,
		},
		Room: types.Room{
			Id:          room.Id,
			ExternalId:  room.ExternalId,
			Name:        room.Name,
			Description: room.Description,
			CreatedAt:   room.CreatedAt,
			UpdatedAt:   room.UpdatedAt,
		},
		CreatedAt: dbSub.CreatedAt,
		UpdatedAt: dbSub.UpdatedAt,
	}

	writeJson(s.log, w, http.StatusCreated, sub)
}

func (s *Server) unsubscribeRoom(w http.ResponseWriter, r *http.Request) {
	externalId := r.URL.Query().Get("room_id")
	if externalId == "" {
		errResp := NewBadRequestError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	userId, ok := UserId(r.Context())
	if !ok {
		errResp := NewUnauthorizedError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	user, err := s.db.GetAccount(userId)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	room, err := s.db.GetRoomByExternalID(externalId)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	err = s.db.DeleteSubscription(userId, room.Id)
	if err != nil {
		s.log.Println("delete subscription:", err)
		errResp := NewInternalServerError(err)
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	s.cs.SubChan <- server.SubReq{
		SubType: server.SubReqTypeUnsubscribe,
		User:    types.User{Id: user.Id, Username: user.Username},
		RoomId:  room.Id,
	}

	writeJson(s.log, w, http.StatusNoContent, nil)
}

func (s *Server) getMessages(w http.ResponseWriter, r *http.Request) {
	externalId := r.URL.Query().Get("room_id")
	if externalId == "" {
		errResp := NewBadRequestError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	room, err := s.db.GetRoomByExternalID(externalId)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	var before, after, limit int

	beforeStr := r.URL.Query().Get("before")
	if beforeStr != "" {
		before, err = strconv.Atoi(beforeStr)
		if err != nil {
			errResp := NewBadRequestError()
			writeJson(s.log, w, errResp.Code, errResp)
			return
		}
	}

	afterStr := r.URL.Query().Get("after")
	if afterStr != "" {
		after, err = strconv.Atoi(afterStr)
		if err != nil {
			errResp := NewBadRequestError()
			writeJson(s.log, w, errResp.Code, errResp)
			return
		}
	}

	limitStr := r.URL.Query().Get("limit")
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			errResp := NewBadRequestError()
			writeJson(s.log, w, errResp.Code, errResp)
			return
		}
	}

	messages, err := s.db.MessageGetAll(room.Id, after, before, limit)
	if err != nil {
		errResp := NewInternalServerError(err)
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	var userMessages []types.Message

	for _, msg := range messages {
		msg := types.Message{
			SeqId:     msg.SeqId,
			UserId:    msg.UserId,
			RoomId:    msg.RoomId,
			Content:   msg.Content,
			Timestamp: msg.CreatedAt,
		}

		userMessages = append(userMessages, msg)
	}

	writeJson(s.log, w, http.StatusOK, userMessages)
}

func (s *Server) serveWs(w http.ResponseWriter, r *http.Request) {
	username, ok := UserId(r.Context())
	if !ok {
		errResp := NewUnauthorizedError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	user, err := s.db.GetAccount(username)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Println("error upgrading connection:", err)
		return
	}

	client := server.NewClient(types.User{
		Id:           user.Id,
		Username:     user.Username,
		EmailAddress: user.EmailAddress,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}, conn, s.cs, s.log)
	s.cs.RegisterChan <- client

	go client.Write()
	go client.Read()
}
