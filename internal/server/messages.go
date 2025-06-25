package server

import (
	"net/http"
	"time"

	"github.com/npezzotti/go-chatroom/internal/types"
)

// BaseMessage is the common structure for all messages sent between the client and server.
// It contains an ID and a timestamp to help with message tracking and ordering.
// The ID is optional and can be used to correlate requests and responses.
// The Timestamp is always set to the current time when the message is created.
type BaseMessage struct {
	Id        int       `json:"id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// ClientMessage represents a message sent by the client to the server.
// It can contain a Publish, Join, or Leave action.
// The UserId field is used to identify the user sending the message.
type ClientMessage struct {
	BaseMessage
	Publish *Publish `json:"publish,omitempty"`
	Join    *Join    `json:"join,omitempty"`
	Leave   *Leave   `json:"leave,omitempty"`
	Read    *Read    `json:"read,omitempty"`
	UserId  int      `json:"-"`
	client  *Client  `json:"-"`
}

// Read represents a notification that the client has read messages in a room.
// It contains the room ID and the sequence ID of the last message read by the client.
// This is used to keep track of which messages the client has already processed.
type Read struct {
	RoomId string `json:"room_id"`
	SeqId  int    `json:"seq_id"`
}

// Publish represents a message that the client wants to publish to a room.
// It contains the room ID, content of the message, username of the sender, and a sequence ID.
type Publish struct {
	RoomId   string `json:"room_id"`
	Content  string `json:"content"`
	Username string `json:"username"`
	SeqId    int    `json:"seq_id"`
}

// Join represents a request from the client to join a room.
// It contains the room ID that the client wants to join.
type Join struct {
	RoomId string `json:"room_id"`
}

// Leave represents a request from the client to leave a room.
// It can optionally include an unsubscribe flag to indicate whether the client wants to unsubscribe from the room.
// The RoomId field specifies which room the client is leaving.
type Leave struct {
	Unsubscribe bool   `json:"unsubscribe,omitempty"`
	RoomId      string `json:"room_id"`
}

// ServerMessage represents a message sent from the server to the client.
// It can contain a Response, Message, or Notification.
type ServerMessage struct {
	BaseMessage
	Response     *Response      `json:"response,omitempty"`
	Message      *types.Message `json:"message,omitempty"`
	Notification *Notification  `json:"notification,omitempty"`
	// SkipClient (optional) is used to skip sending the message to a client (i.e the client that sent the original message).
	SkipClient *Client `json:"-"`
	// UserId (optional) field used to identify a user for whom the message is intended.
	UserId int `json:"-"`
}

// Response represents the response sent from the server to the client.
type Response struct {
	ResponseCode int    `json:"response_code"`
	Error        string `json:"error,omitempty"`
	Data         any    `json:"data,omitempty"`
}

// Notification represents a notification sent from the server to the client.
type Notification struct {
	Presence           *Presence            `json:"presence,omitempty"`
	Message            *MessageNotification `json:"message,omitempty"`
	SubscriptionChange *SubscriptionChange  `json:"subscription_change,omitempty"`
	RoomDeleted        *RoomDeleted         `json:"room_deleted,omitempty"`
}

// Presence represents the presence status of a user in a room.
type Presence struct {
	Present bool   `json:"present"`
	UserId  int    `json:"user_id,omitempty"`
	RoomId  string `json:"room_id,omitempty"`
}

type MessageNotification struct {
	RoomId string `json:"room_id"`
	SeqId  int    `json:"seq_id"`
}

type SubscriptionChange struct {
	RoomId     string     `json:"room_id"`
	Subscribed bool       `json:"subscribed"`
	User       types.User `json:"user"`
}

type RoomDeleted struct {
	RoomId string `json:"room_id"`
}

// GetUserId returns the UserId of the ClientMessage.
// If UserId is set to a positive value, it returns that value.
// If UserId is not set (i.e., it is 0), it attempts to extract the user ID from the associated Client.
// Otherwise, it returns 0.
func (cm *ClientMessage) GetUserId() int {
	if cm.UserId > 0 {
		return cm.UserId
	}
	if cm.client != nil {
		return cm.client.user.Id
	}

	return 0
}

func NoErrOK(id int, data any) *ServerMessage {
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

func ErrSubscriptionNotFound(id int) *ServerMessage {
	return &ServerMessage{
		BaseMessage: BaseMessage{
			Id:        id,
			Timestamp: Now(),
		},
		Response: &Response{
			ResponseCode: http.StatusNotFound,
			Error:        "subscription not found",
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
