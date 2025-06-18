package database

import "time"

type Room struct {
	Id            int
	Name          string
	ExternalId    string
	Description   string
	SeqId         int
	OwnerId       int
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Subscriptions []Subscription
}

type User struct {
	Id           int
	Username     string
	EmailAddress string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Subscription struct {
	Id            int
	LastReadSeqId int
	Room          Room
	AccountId     int
	Username      string
	RoomId        int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Message struct {
	Id        int
	SeqId     int
	RoomId    int
	UserId    int
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CreateAccountParams struct {
	Username     string
	EmailAddress string
	PasswordHash string
}

type UpdateAccountParams struct {
	UserId       int
	Username     string
	PasswordHash string
}

type CreateRoomParams struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	OwnerId     int    `json:"-"`
	ExternalId  string `json:"external_id"`
}
