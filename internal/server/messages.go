package server

import (
	"net/http"
	"time"

	"github.com/npezzotti/go-chatroom/internal/types"
)

type BaseMessage struct {
	Id        int       `json:"id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type ClientMessage struct {
	BaseMessage
	Publish *Publish `json:"publish,omitempty"`
	Join    *Join    `json:"join,omitempty"`
	Leave   *Leave   `json:"leave,omitempty"`
	UserId  int      `json:"-"`
	client  *Client  `json:"-"`
}

type Publish struct {
	RoomId   string `json:"room_id"`
	Content  string `json:"content"`
	Username string `json:"username"`
	SeqId    int    `json:"seq_id"`
}

type Join struct {
	RoomId string `json:"room_id"`
}

type Leave struct {
	Unsubscribe bool   `json:"unsubscribe,omitempty"`
	RoomId      string `json:"room_id"`
}

type ServerMessage struct {
	BaseMessage
	Response     *Response      `json:"response,omitempty"`
	Message      *types.Message `json:"message,omitempty"`
	Notification *Notification  `json:"notification,omitempty"`
	SkipClient   *Client        `json:"-"`
}

type Response struct {
	ResponseCode int            `json:"response_code"`
	Error        string         `json:"error,omitempty"`
	Data         map[string]any `json:"data,omitempty"`
}

type Notification struct {
	Presence           *Presence           `json:"presence,omitempty"`
	SubscriptionChange *SubscriptionChange `json:"subscription_change,omitempty"`
	RoomDeleted        *RoomDeleted        `json:"room_deleted,omitempty"`
}

type Presence struct {
	Present bool   `json:"present"`
	UserId  int    `json:"user_id"`
	RoomId  string `json:"room_id"`
}

type SubscriptionChange struct {
	RoomId     string     `json:"room_id"`
	Subscribed bool       `json:"subscribed"`
	User       types.User `json:"user"`
}

type RoomDeleted struct {
	RoomId string `json:"room_id"`
}

func NoErrOK(id int, data map[string]any) *ServerMessage {
	return &ServerMessage{
		BaseMessage: BaseMessage{
			Id:        id,
			Timestamp: Now(),
		},
		Response: &Response{
			ResponseCode: http.StatusOK,
			Data:         data,
		},
	}
}

func NoErrAccepted(id int) *ServerMessage {
	return &ServerMessage{
		BaseMessage: BaseMessage{
			Id:        id,
			Timestamp: Now(),
		},
		Response: &Response{
			ResponseCode: http.StatusAccepted,
		},
	}
}

func ErrRoomNotFound(id int) *ServerMessage {
	return &ServerMessage{
		BaseMessage: BaseMessage{
			Id:        id,
			Timestamp: Now(),
		},
		Response: &Response{
			ResponseCode: http.StatusNotFound,
			Error:        "room not found",
		},
	}
}

func ErrInternalError(id int) *ServerMessage {
	return &ServerMessage{
		BaseMessage: BaseMessage{
			Id:        id,
			Timestamp: Now(),
		},
		Response: &Response{
			ResponseCode: http.StatusInternalServerError,
			Error:        "internal server error",
		},
	}
}

func ErrServiceUnavailable(id int) *ServerMessage {
	return &ServerMessage{
		BaseMessage: BaseMessage{
			Id:        id,
			Timestamp: Now(),
		},
		Response: &Response{
			ResponseCode: http.StatusServiceUnavailable,
			Error:        "service unavailable",
		},
	}
}

func ErrInvalidMessage(id int) *ServerMessage {
	msg := &ServerMessage{
		BaseMessage: BaseMessage{
			Timestamp: Now(),
		},
		Response: &Response{
			ResponseCode: http.StatusBadRequest,
			Error:        "invalid message format",
		},
	}

	if id > 0 {
		msg.Id = id
	}
	return msg
}

func Now() time.Time {
	return time.Now().UTC().Round(time.Millisecond)
}
