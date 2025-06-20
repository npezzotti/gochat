package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
	"github.com/npezzotti/go-chatroom/internal/database"
	"github.com/npezzotti/go-chatroom/internal/server"
	"github.com/npezzotti/go-chatroom/internal/types"
	"github.com/teris-io/shortid"
)

func (s *Server) writeJson(w http.ResponseWriter, statusCode int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.log.Printf("json encode: %v", err)
	}
}

func (s *Server) createRoom(w http.ResponseWriter, r *http.Request) {
	var params database.CreateRoomParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		errResp := NewBadRequestError()
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	userId, ok := UserId(r.Context())
	if !ok {
		errResp := NewUnauthorizedError()
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	sid, err := shortid.Generate()
	if err != nil {
		s.log.Print("generate shortid:", err)
		errResp := NewInternalServerError(err)
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	params.ExternalId = sid
	params.OwnerId = userId

	newRoom, err := s.db.CreateRoom(params)
	if err != nil {
		errResp := NewBadRequestError()
		s.writeJson(w, errResp.StatusCode, errResp)
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

	s.writeJson(w, http.StatusCreated, room)
}

func (s *Server) getRoom(w http.ResponseWriter, r *http.Request) {
	externalId := r.URL.Query().Get("id")
	if externalId == "" {
		errResp := NewBadRequestError()
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	dbRoom, err := s.db.GetRoomByExternalID(externalId)
	if err != nil {
		var errResp *ApiError
		if errors.Is(err, sql.ErrNoRows) {
			errResp = NewNotFoundError()
		} else {
			errResp = NewInternalServerError(err)
		}
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	dbSubs, err := s.db.GetSubscribersForRoom(dbRoom.Id)
	if err != nil {
		s.log.Println("get subscribers for room:", err)
		errResp := NewInternalServerError(err)
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}
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

	s.writeJson(w, http.StatusOK, room)
}

func (s *Server) deleteRoom(w http.ResponseWriter, r *http.Request) {
	userId, ok := UserId(r.Context())
	if !ok {
		errResp := NewUnauthorizedError()
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	externalId := r.URL.Query().Get("id")
	if externalId == "" {
		errResp := NewBadRequestError()
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	room, err := s.db.GetRoomByExternalID(externalId)
	if err != nil {
		var errResp *ApiError
		if errors.Is(err, sql.ErrNoRows) {
			errResp = NewNotFoundError()
		} else {
			errResp = NewInternalServerError(err)
		}
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	// Check if the user is the owner of the room
	if room.OwnerId != userId {
		errResp := NewForbiddenError()
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	err = s.db.DeleteRoom(room.Id)
	if err != nil {
		s.log.Println("delete room:", err)
		errResp := NewInternalServerError(err)
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	s.cs.DelRoomChan <- room.ExternalId
	s.writeJson(w, http.StatusNoContent, nil)
}

func (s *Server) getUsersSubscriptions(w http.ResponseWriter, r *http.Request) {
	userId, ok := UserId(r.Context())
	if !ok {
		errResp := NewUnauthorizedError()
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	dbSubs, err := s.db.ListSubscriptions(userId)
	if err != nil {
		s.log.Println("list subscriptions:", err)
		errResp := NewInternalServerError(err)
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	var subs []types.Subscription
	for _, dbSub := range dbSubs {
		subs = append(subs, types.Subscription{
			Id:            dbSub.Id,
			LastReadSeqId: dbSub.LastReadSeqId,
			Room: types.Room{
				Id:          dbSub.Room.Id,
				ExternalId:  dbSub.Room.ExternalId,
				Name:        dbSub.Room.Name,
				Description: dbSub.Room.Description,
				SeqId:       dbSub.Room.SeqId,
				CreatedAt:   dbSub.Room.CreatedAt,
				UpdatedAt:   dbSub.Room.UpdatedAt,
			},
			CreatedAt: dbSub.CreatedAt,
			UpdatedAt: dbSub.UpdatedAt,
		})
	}

	s.writeJson(w, http.StatusOK, subs)
}

func (s *Server) getMessages(w http.ResponseWriter, r *http.Request) {
	externalId := r.URL.Query().Get("room_id")
	if externalId == "" {
		errResp := NewBadRequestError()
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	room, err := s.db.GetRoomByExternalID(externalId)
	if err != nil {
		var errResp *ApiError
		if errors.Is(err, sql.ErrNoRows) {
			errResp = NewNotFoundError()
		} else {
			errResp = NewInternalServerError(err)
		}
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	var before, after, limit int

	beforeStr := r.URL.Query().Get("before")
	if beforeStr != "" {
		before, err = strconv.Atoi(beforeStr)
		if err != nil {
			errResp := NewBadRequestError()
			s.writeJson(w, errResp.StatusCode, errResp)
			return
		}
	}

	afterStr := r.URL.Query().Get("after")
	if afterStr != "" {
		after, err = strconv.Atoi(afterStr)
		if err != nil {
			errResp := NewBadRequestError()
			s.writeJson(w, errResp.StatusCode, errResp)
			return
		}
	}

	limitStr := r.URL.Query().Get("limit")
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			errResp := NewBadRequestError()
			s.writeJson(w, errResp.StatusCode, errResp)
			return
		}
	}

	messages, err := s.db.MessageGetAll(room.Id, after, before, limit)
	if err != nil {
		errResp := NewInternalServerError(err)
		s.writeJson(w, errResp.StatusCode, errResp)
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

	s.writeJson(w, http.StatusOK, userMessages)
}

func (s *Server) serveWs(w http.ResponseWriter, r *http.Request) {
	username, ok := UserId(r.Context())
	if !ok {
		errResp := NewUnauthorizedError()
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	user, err := s.db.GetAccount(username)
	if err != nil {
		errResp := NewNotFoundError()
		s.writeJson(w, errResp.StatusCode, errResp)
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
