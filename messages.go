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
	Publish *MessagePublish `json:"publish,omitempty"`
	Join    *MessageJoin    `json:"join,omitempty"`
	Leave   *MessageLeave   `json:"leave,omitempty"`
	UserId  int             `json:"-"`
	RoomId  int             `json:"room_id"`
	client  *Client         `json:"-"`
}

type MessagePublish struct {
	Content  string `json:"content"`
	Username string `json:"username"`
	SeqId    int    `json:"seq_id"`
}

type MessageJoin struct {
	RoomId int `json:"room_id"`
}

type MessageLeave struct {
	RoomId int `json:"room_id"`
}

type ServerMessage struct {
	BaseMessage
	Response     *Response     `json:"response,omitempty"`
	Message      *Message      `json:"message,omitempty"`
	Notification *Notification `json:"notification,omitempty"`
	RoomId       int           `json:"room_id,omitempty"`
	UserId       int           `json:"user_id,omitempty"`
}

type Message struct {
	SeqId    int    `json:"seq_id"`
	RoomId   int    `json:"room_id"`
	Username string `json:"username"`
	Content  string `json:"content"`
}

type Response struct {
	Status    int            `json:"status"`
	ErrorCode int            `json:"error_code,omitempty"`
	Error     string         `json:"error,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
}

type Notification struct {
	Presence           *Presence           `json:"presence,omitempty"`
	SubscriptionChange *SubscriptionChange `json:"subscription_change,omitempty"`
	RoomDeleted        *RoomDeleted        `json:"room_deleted,omitempty"`
}

type Presence struct {
	Present bool `json:"present"`
}

type SubscriptionChange struct {
	Subscribed bool `json:"subscribed"`
	User       User `json:"user"`
}

type RoomDeleted struct {
	RoomId int `json:"room_id"`
}

const (
	ErrCodeNotFound = http.StatusNotFound
)
