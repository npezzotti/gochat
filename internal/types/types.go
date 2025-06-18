package types

import (
	"time"
)

type User struct {
	Id           int       `json:"id"`
	Username     string    `json:"username"`
	EmailAddress string    `json:"email_address,omitempty"`
	IsPresent    bool      `json:"is_present,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
}

type Room struct {
	Id          int       `json:"id"`
	Name        string    `json:"name"`
	ExternalId  string    `json:"external_id"`
	Description string    `json:"description"`
	SeqId       int       `json:"seq_id"`
	OwnerId     int       `json:"owner_id,omitempty"`
	Subscribers []User    `json:"subscribers,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Subscription struct {
	Id            int       `json:"id"`
	LastReadSeqId int       `json:"last_read_seq_id"`
	Room          Room      `json:"room"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Message struct {
	SeqId     int       `json:"seq_id"`
	RoomId    int       `json:"room_id"`
	UserId    int       `json:"user_id"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}
