package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/npezzotti/go-chatroom/internal/database"
	"github.com/npezzotti/go-chatroom/internal/server"
	"github.com/npezzotti/go-chatroom/internal/types"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type UpdateAccountRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type CreateRoomRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *GoChatApp) writeJson(w http.ResponseWriter, statusCode int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if v == nil {
		return
	}

	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.log.Printf("json encode: %v", err)
	}
}

func (s *GoChatApp) createAccount(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errResp := NewBadRequestError()
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	if req.Username == "" || req.Email == "" || req.Password == "" {
		errResp := NewBadRequestError()
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	pwdHash, err := hashPassword(req.Password)
	if err != nil {
		errResp := NewInternalServerError(err)
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	params := database.CreateAccountParams{
		Username:     r.Form.Get("username"),
		EmailAddress: r.Form.Get("email"),
		PasswordHash: pwdHash,
	}

	newUser, err := s.db.CreateAccount(params)
	if err != nil {
		errResp := NewInternalServerError(err)
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	s.writeJson(w, http.StatusCreated, types.User{
		Id:           newUser.Id,
		Username:     newUser.Username,
		EmailAddress: newUser.EmailAddress,
		CreatedAt:    newUser.CreatedAt,
		UpdatedAt:    newUser.UpdatedAt,
	})
}

func (s *GoChatApp) account(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		userId, ok := UserId(r.Context())
		if !ok {
			errResp := NewUnauthorizedError()
			s.writeJson(w, errResp.StatusCode, errResp)
			return
		}

		user, err := s.db.GetAccountById(userId)
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

		u := types.User{
			Id:           user.Id,
			Username:     user.Username,
			EmailAddress: user.EmailAddress,
			CreatedAt:    user.CreatedAt,
			UpdatedAt:    user.UpdatedAt,
		}

		s.writeJson(w, http.StatusOK, u)
	case http.MethodPut:
		userId, ok := UserId(r.Context())
		if !ok {
			errResp := NewUnauthorizedError()
			s.writeJson(w, errResp.StatusCode, errResp)
			return
		}

		curUser, err := s.db.GetAccountById(userId)
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

		var updateAccountReq UpdateAccountRequest
		err = json.NewDecoder(r.Body).Decode(&updateAccountReq)
		if err != nil {
			errResp := NewBadRequestError()
			s.writeJson(w, errResp.StatusCode, errResp)
			return
		}

		if updateAccountReq.Username == "" || updateAccountReq.Password == "" {
			errResp := NewBadRequestError()
			s.writeJson(w, errResp.StatusCode, errResp)
			return
		}

		pwdHash, err := hashPassword(updateAccountReq.Password)
		if err != nil {
			errResp := NewInternalServerError(err)
			s.writeJson(w, errResp.StatusCode, errResp)
			return
		}

		params := database.UpdateAccountParams{
			UserId:       curUser.Id,
			Username:     updateAccountReq.Username,
			PasswordHash: pwdHash,
		}

		dbUser, err := s.db.UpdateAccount(params)
		if err != nil {
			errResp := NewInternalServerError(err)
			s.writeJson(w, errResp.StatusCode, errResp)
			return
		}

		userResp := types.User{
			Id:           dbUser.Id,
			Username:     dbUser.Username,
			EmailAddress: dbUser.EmailAddress,
			CreatedAt:    dbUser.CreatedAt,
			UpdatedAt:    dbUser.UpdatedAt,
		}

		s.writeJson(w, http.StatusOK, userResp)
	default:
		errResp := NewMethodNotAllowedError()
		s.writeJson(w, errResp.StatusCode, errResp)
	}
}

func (s *GoChatApp) session(w http.ResponseWriter, r *http.Request) {
	userId, ok := UserId(r.Context())
	if !ok {
		errResp := NewUnauthorizedError()
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	user, err := s.db.GetAccountById(userId)
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

	u := types.User{
		Id:           user.Id,
		Username:     user.Username,
		EmailAddress: user.EmailAddress,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}

	s.writeJson(w, http.StatusOK, u)
}

func (s *GoChatApp) login(w http.ResponseWriter, r *http.Request) {
	var lr LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&lr); err != nil {
		errResp := NewBadRequestError()
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	if lr.Email == "" || lr.Password == "" {
		errResp := NewBadRequestError()
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	dbUser, err := s.db.GetAccountByEmail(lr.Email)
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

	if !verifyPassword(dbUser.PasswordHash, lr.Password) {
		errResp := NewUnauthorizedError()
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	u := types.User{
		Id:           dbUser.Id,
		Username:     dbUser.Username,
		EmailAddress: dbUser.EmailAddress,
		CreatedAt:    dbUser.CreatedAt,
		UpdatedAt:    dbUser.UpdatedAt,
	}

	token, err := s.createJwtForSession(u, defaultJwtExpiration)
	if err != nil {
		errResp := NewInternalServerError(err)
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	http.SetCookie(w, createJwtCookie(token, defaultJwtExpiration))

	s.writeJson(w, http.StatusOK, u)
}

func (s *GoChatApp) logout(w http.ResponseWriter, _ *http.Request) {
	// instruct browser to delete cookie by overwriting it with an expired token
	http.SetCookie(w, createJwtCookie("", time.Duration(time.Unix(0, 0).Unix())))
	w.WriteHeader(http.StatusNoContent)
}

func (s *GoChatApp) createRoom(w http.ResponseWriter, r *http.Request) {
	var createRoomReq CreateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&createRoomReq); err != nil {
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

	sid, err := s.generateShortId()
	if err != nil {
		s.log.Print("generateShortId:", err)
		errResp := NewInternalServerError(err)
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	params := database.CreateRoomParams{
		Name:        createRoomReq.Name,
		Description: createRoomReq.Description,
		OwnerId:     userId,
		ExternalId:  sid,
	}

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
		OwnerId:     newRoom.OwnerId,
		CreatedAt:   newRoom.CreatedAt,
		UpdatedAt:   newRoom.UpdatedAt,
	}

	s.writeJson(w, http.StatusCreated, room)
}

func (s *GoChatApp) deleteRoom(w http.ResponseWriter, r *http.Request) {
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

	room, err := s.db.GetRoomByExternalId(externalId)
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

	if err := s.cs.UnloadRoom(r.Context(), room.ExternalId, true); err != nil {
		s.log.Println("delete room from chat server:", err)
		errResp := NewInternalServerError(err)
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}
	s.writeJson(w, http.StatusNoContent, nil)
}

func (s *GoChatApp) getUsersSubscriptions(w http.ResponseWriter, r *http.Request) {
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

func (s *GoChatApp) getMessages(w http.ResponseWriter, r *http.Request) {
	externalId := r.URL.Query().Get("room_id")
	if externalId == "" {
		errResp := NewBadRequestError()
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	room, err := s.db.GetRoomByExternalId(externalId)
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

	messages, err := s.db.GetMessages(room.Id, after, before, limit)
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

func (s *GoChatApp) serveWs(w http.ResponseWriter, r *http.Request) {
	id, ok := UserId(r.Context())
	if !ok {
		errResp := NewUnauthorizedError()
		s.writeJson(w, errResp.StatusCode, errResp)
		return
	}

	user, err := s.db.GetAccountById(id)
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

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			// only allow connections from allowed origins
			origin := r.Header.Get("Origin")
			if origin == "" {
				// if no origin header, allow the request
				return true
			}

			return slices.Contains(s.allowedOrigins, origin)
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
	}, conn, s.cs, s.log, s.stats)

	s.cs.RegisterClient(client)
	go client.Write()
	go client.Read()
}
