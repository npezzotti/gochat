package main

import (
	"time"

	"github.com/npezzotti/go-chatroom/db"
)

const (
	createAccountQuery = "INSERT INTO accounts (username, email, password_hash, created_at) " +
		"VALUES ($1, $2, $3, $4) RETURNING id, username, email"
	updateAccountQuery = "UPDATE accounts SET username = $2, password_hash = $3, updated_at = $4 " +
		"WHERE id = $1 RETURNING id, username, email"
	getAccountByIdQuery = "SELECT id, username, email FROM accounts " +
		"WHERE id = $1 LIMIT 1"
	getAccountByEmailQuery = "SELECT id, username, email, password_hash FROM accounts " +
		"WHERE email = $1 LIMIT 1"
	createRoomQuery = "INSERT INTO rooms (name, description, owner_id, created_at) " +
		"VALUES ($1, $2, $3, $4) RETURNING id, name, description, owner_id"
)

type CreateAccountParams struct {
	Username     string
	EmailAddress string
	PasswordHash string
}

type UpdateAccountParams struct {
	User         User
	Username     string
	PasswordHash string
}

type CreateRoomParams struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	OwnerId     int    `json:"owner_id"`
}

func CreateAccount(accountParams CreateAccountParams) (User, error) {
	res := DB.QueryRow(
		createAccountQuery,
		accountParams.Username,
		accountParams.EmailAddress,
		accountParams.PasswordHash,
		time.Now(),
	)

	var u User
	err := res.Scan(
		&u.Id,
		&u.Username,
		&u.EmailAddress,
	)

	return u, err
}

func UpdateAccount(accountParams UpdateAccountParams) (User, error) {
	res := DB.QueryRow(
		updateAccountQuery,
		accountParams.User.Id,
		accountParams.Username,
		accountParams.PasswordHash,
		time.Now(),
	)

	var u User
	err := res.Scan(
		&u.Id,
		&u.Username,
		&u.EmailAddress,
	)

	return u, err
}

func GetAccount(id int) (User, error) {
	row := DB.QueryRow(
		getAccountByIdQuery,
		id,
	)

	var user User
	err := row.Scan(
		&user.Id,
		&user.Username,
		&user.EmailAddress,
	)

	return user, err
}

func GetAccountByEmail(email string) (User, error) {
	row := DB.QueryRow(
		getAccountByEmailQuery,
		email,
	)
	var user User
	err := row.Scan(
		&user.Id,
		&user.Username,
		&user.EmailAddress,
		&user.PasswordHash,
	)

	return user, err
}

func GetRoomById(id int) (db.Room, error) {
	row := DB.QueryRow(
		"SELECT id, name, description FROM rooms "+
			"WHERE id = $1 LIMIT 1",
		id,
	)

	var room db.Room
	err := row.Scan(
		&room.Id,
		&room.Name,
		&room.Description,
	)

	return room, err
}

func CreateRoom(params CreateRoomParams) (db.Room, error) {
	res := DB.QueryRow(
		createRoomQuery,
		params.Name,
		params.Description,
		params.OwnerId,
		time.Now(),
	)

	var room db.Room
	err := res.Scan(
		&room.Id,
		&room.Name,
		&room.Description,
		&room.Owner,
		&room.CreatedAt,
	)

	return room, err
}
