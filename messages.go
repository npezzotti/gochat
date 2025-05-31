package main

import (
	"net/http"
	"time"
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
	RoomId   int    `json:"room_id"`
	Content  string `json:"content"`
	Username string `json:"username"`
	SeqId    int    `json:"seq_id"`
}

type Join struct {
	RoomId int `json:"room_id"`
}

type Leave struct {
	RoomId int `json:"room_id"`
}

type ServerMessage struct {
	BaseMessage
	Response     *Response     `json:"response,omitempty"`
	Message      *Message      `json:"message,omitempty"`
	Notification *Notification `json:"notification,omitempty"`
	SkipClient   *Client       `json:"-"`
}

type Message struct {
	SeqId     int       `json:"seq_id"`
	RoomId    int       `json:"room_id"`
	UserId    int       `json:"user_id"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type Response struct {
	ResponseCode MessageStatusCode `json:"response_code"`
	Error        string            `json:"error,omitempty"`
	Data         map[string]any    `json:"data,omitempty"`
}

type Notification struct {
	Presence           *Presence           `json:"presence,omitempty"`
	SubscriptionChange *SubscriptionChange `json:"subscription_change,omitempty"`
	RoomDeleted        *RoomDeleted        `json:"room_deleted,omitempty"`
}

type Presence struct {
	Present bool `json:"present"`
	UserId  int  `json:"user_id"`
	RoomId  int  `json:"room_id"`
}

type SubscriptionChange struct {
	RoomId     int  `json:"room_id"`
	Subscribed bool `json:"subscribed"`
	User       User `json:"user"`
}

type RoomDeleted struct {
	RoomId int `json:"room_id"`
}

type MessageStatusCode int

const (
	ResponseCodeNotFound      MessageStatusCode = http.StatusNotFound
	ResponseCodeInternalError MessageStatusCode = http.StatusInternalServerError
	ResponseCodeOK            MessageStatusCode = http.StatusOK
)

func ErrRoomNotFound(id int) *ServerMessage {
	return &ServerMessage{
		BaseMessage: BaseMessage{
			Id:        id,
			Timestamp: time.Now().Round(time.Millisecond),
		},
		Response: &Response{
			ResponseCode: ResponseCodeNotFound,
			Error:        "room not found",
		},
	}
}
