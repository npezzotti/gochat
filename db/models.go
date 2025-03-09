package db

import "time"

type Room struct {
	Id          int
	Owner       User
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt    time.Time
}

type User struct {
	Id           int
	Username     string
	EmailAddress string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
