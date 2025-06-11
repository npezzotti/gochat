package types

import (
	"time"
)

type User struct {
	Id           int       `json:"id"`
	Username     string    `json:"username"`
	EmailAddress string    `json:"email_address,omitempty"`
	Password     string    `json:"-"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
}

type Room struct {
	Id          int       `json:"id"`
	Name        string    `json:"name"`
	ExternalId  string    `json:"external_id"`
	Description string    `json:"description"`
	SeqId       int       `json:"seq_id"`
	Subscribers []User    `json:"subscribers"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

type Subscription struct {
	Id        int       `json:"id"`
	User      User      `json:"user"`
	Room      Room      `json:"room"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type Message struct {
	SeqId     int       `json:"seq_id"`
	RoomId    int       `json:"room_id"`
	UserId    int       `json:"user_id"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}
