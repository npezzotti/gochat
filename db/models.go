package db

import "time"

type Room struct {
	Id            int
	OwnerId       int
	Name          string
	ExternalId    string
	Description   string
	SeqId         int
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
	Id        int
	AccountId int
	Username  string
	RoomId    int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type UserMessage struct {
	Id        int
	SeqId     int
	RoomId    int
	UserId    int
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
}
