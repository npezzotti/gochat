package db

import "time"

type Room struct {
	Id          int
	OwnerId     int
	Name        string
	Description string
	SeqId       int
	CreatedAt   time.Time
	UpdatedAt   time.Time
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
	RoomId    int
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
