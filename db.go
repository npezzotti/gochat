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
	createRoomQuery = "INSERT INTO rooms (name, description, owner_id, created_at, updated_at) " +
		"VALUES ($1, $2, $3, $4, $5) RETURNING id, name, description, owner_id, created_at, updated_at"
	deleteRoomQuery = "DELETE FROM rooms WHERE id = $1"
	createSubQuery  = "INSERT INTO subscriptions (account_id, room_id, created_at, updated_at) VALUES ($1, $2, $3, $4) RETURNING id, account_id, room_id"
	getSubQuery     = "SELECT id FROM subscriptions WHERE account_id = $1 AND room_id = $2 LIMIT 1"
	listSubQuery    = "SELECT r.id, r.name, r.description FROM subscriptions s JOIN rooms r ON r.id = s.room_id WHERE s.account_id = $1"
	deleteSubQuery  = "DELETE FROM subscriptions WHERE account_id = $1 AND room_id = $2"
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
	OwnerId     int    `json:"-"`
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
		"SELECT id, name, description, seq_id FROM rooms "+
			"WHERE id = $1 LIMIT 1",
		id,
	)

	var room db.Room
	err := row.Scan(
		&room.Id,
		&room.Name,
		&room.Description,
		&room.SeqId,
	)

	return room, err
}

func CreateRoom(params CreateRoomParams) (db.Room, error) {
	tx, err := DB.Begin()
	if err != nil {
		return db.Room{}, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()
	res := tx.QueryRow(
		createRoomQuery,
		params.Name,
		params.Description,
		params.OwnerId,
		time.Now(),
		time.Now(),
	)

	var room db.Room
	err = res.Scan(
		&room.Id,
		&room.Name,
		&room.Description,
		&room.OwnerId,
		&room.CreatedAt,
		&room.UpdatedAt,
	)

	_, err = tx.Exec(
		createSubQuery,
		params.OwnerId,
		room.Id,
		time.Now(),
		time.Now(),
	)
	if err != nil {
		return db.Room{}, err
	}

	if err = tx.Commit(); err != nil {
		return db.Room{}, err
	}

	return room, err
}

func DeleteRoom(id int) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	_, err = tx.Exec("DELETE FROM subscriptions WHERE room_id = $1", id)
	if err != nil {
		return err
	}

	_, err = tx.Exec(deleteRoomQuery, id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func CreateSubscription(userId, roomId int) (db.Subscription, error) {
	res := DB.QueryRow(
		createSubQuery,
		userId,
		roomId,
		time.Now(),
		time.Now(),
	)

	var sub db.Subscription
	err := res.Scan(
		&sub.Id,
		&sub.AccountId,
		&sub.RoomId,
	)

	return sub, err
}

func SubscriptionExists(account_id, room_id int) bool {
	res := DB.QueryRow(
		getSubQuery,
		account_id,
		room_id,
	)

	var sub db.Subscription
	err := res.Scan(
		&sub.Id,
	)

	return err == nil
}

func ListSubscriptions(account_id int) ([]db.Room, error) {
	rows, err := DB.Query(
		listSubQuery,
		account_id,
	)

	if err != nil {
		return nil, err
	}

	var rooms []db.Room
	for rows.Next() {
		var room db.Room
		if err = rows.Scan(&room.Id, &room.Name, &room.Description); err != nil {
			break
		}

		rooms = append(rooms, room)
	}
	return rooms, err
}

func DeleteSubscription(accountId, roomId int) error {
	_, err := DB.Exec(
		deleteSubQuery,
		accountId,
		roomId,
	)

	return err
}

func MessageCreate(msg db.UserMessage) error {
	err := RoomUpdateOnMessage(msg)
	if err != nil {
		return err
	}

	_, err = DB.Exec(
		"INSERT INTO messages (seq_id, room_id, user_id, content, created_at, updated_at) "+
			"VALUES ($1, $2, $3, $4, $5, $6)",
		msg.SeqId,
		msg.RoomId,
		msg.UserId,
		msg.Content,
		time.Now(),
		time.Now(),
	)

	return err
}

func RoomUpdateOnMessage(msg db.UserMessage) error {
	_, err := DB.Exec("UPDATE rooms SET seq_id = $1 WHERE id = $2", msg.SeqId, msg.RoomId)

	return err
}

func GetSubscribersForRoom(roomId int) ([]db.User, error) {
	rows, err := DB.Query("SELECT a.id, a.username FROM subscriptions AS s JOIN accounts AS a ON s.account_id = a.id WHERE s.room_id = $1", roomId)

	var subs = make([]db.User, 0)
	for rows.Next() {
		var sub db.User
		if err = rows.Scan(&sub.Id, &sub.Username); err != nil {
			break
		}

		subs = append(subs, sub)
	}

	return subs, err
}

func MessageGetAll(roomId, since, before, limit int) ([]db.UserMessage, error) {
	var upper, lower int
	if before == 0 {
		upper = 1<<31 - 1
	}

	if limit <= 0 {
		limit = 20
	}

	rows, err := DB.Query(
		"SELECT id, seq_id, room_id, user_id, content FROM messages "+
			"WHERE room_id = $1 AND seq_id BETWEEN $2 AND $3 ORDER BY seq_id DESC LIMIT $4",
		roomId,
		lower,
		upper,
		limit,
	)

	if err != nil {
		return nil, err
	}

	var messages = make([]db.UserMessage, 0, limit)
	for rows.Next() {
		var msg db.UserMessage
		if err = rows.Scan(&msg.Id, &msg.SeqId, &msg.RoomId, &msg.UserId, &msg.Content); err != nil {
			break
		}

		messages = append(messages, msg)
	}
	return messages, err
}
