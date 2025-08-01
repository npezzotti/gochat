package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate"
	"github.com/golang-migrate/migrate/database/postgres"
	_ "github.com/golang-migrate/migrate/source/file"
)

type PgGoChatRepository struct {
	conn *sql.DB
}

func NewPgGoChatRepository(dsn string) (*PgGoChatRepository, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &PgGoChatRepository{conn: db}, nil
}

func (db *PgGoChatRepository) Ping() error {
	if db.conn == nil {
		return fmt.Errorf("database connection is nil")
	}
	if err := db.conn.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	return nil
}

func (db *PgGoChatRepository) Migrate() error {
	driver, err := postgres.WithInstance(db.conn, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://internal/database/migrations",
		"postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}

	if err := m.Up(); err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("failed to apply migrations: %w", err)
		}
	}
	return nil
}

func (db *PgGoChatRepository) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

const (
	createSubQuery = "INSERT INTO subscriptions (account_id, room_id) VALUES ($1, $2) RETURNING id, account_id, room_id"
)

func (db *PgGoChatRepository) CreateAccount(accountParams CreateAccountParams) (User, error) {
	res := db.conn.QueryRow(
		"INSERT INTO accounts (username, email, password_hash) "+
			"VALUES ($1, $2, $3) RETURNING id, username, email, password_hash, created_at, updated_at",
		accountParams.Username,
		accountParams.EmailAddress,
		accountParams.PasswordHash,
	)

	var u User
	err := res.Scan(
		&u.Id,
		&u.Username,
		&u.EmailAddress,
		&u.PasswordHash,
		&u.CreatedAt,
		&u.UpdatedAt,
	)

	return u, err
}

func (db *PgGoChatRepository) UpdateAccount(accountParams UpdateAccountParams) (User, error) {
	res := db.conn.QueryRow(
		"UPDATE accounts SET username = $2, password_hash = $3, updated_at = $4 "+
			"WHERE id = $1 RETURNING id, username, email",
		accountParams.UserId,
		accountParams.Username,
		accountParams.PasswordHash,
		time.Now().UTC(),
	)

	var u User
	err := res.Scan(
		&u.Id,
		&u.Username,
		&u.EmailAddress,
	)

	return u, err
}

func (db *PgGoChatRepository) GetAccountById(id int) (User, error) {
	row := db.conn.QueryRow(
		"SELECT id, username, email FROM accounts "+
			"WHERE id = $1 LIMIT 1",
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

func (db *PgGoChatRepository) GetAccountByEmail(email string) (User, error) {
	row := db.conn.QueryRow(
		"SELECT id, username, email, password_hash FROM accounts "+
			"WHERE email = $1 LIMIT 1",
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

func (db *PgGoChatRepository) GetRoomByExternalId(externalId string) (Room, error) {
	row := db.conn.QueryRow(
		"SELECT id, name, external_id, description, seq_id, owner_id, created_at, updated_at FROM rooms "+
			"WHERE external_id = $1 LIMIT 1",
		externalId,
	)

	var room Room
	err := row.Scan(
		&room.Id,
		&room.Name,
		&room.ExternalId,
		&room.Description,
		&room.SeqId,
		&room.OwnerId,
		&room.CreatedAt,
		&room.UpdatedAt,
	)

	return room, err
}

func (db *PgGoChatRepository) GetRoomWithSubscribers(roomId int) (*Room, error) {
	query := `
		SELECT 
				r.id AS room_id,
				r.name AS room_name,
				r.external_id,
				r.description,
				r.seq_id,
				r.owner_id,
				r.created_at AS room_created_at,
				r.updated_at AS room_updated_at,
				s.id,
				s.account_id,
				a.username,
				s.created_at AS subscription_created_at,
				s.updated_at AS subscription_updated_at
		FROM rooms r
		LEFT JOIN subscriptions s ON r.id = s.room_id
		LEFT JOIN accounts a ON s.account_id = a.id
		WHERE r.id = $1;
`

	rows, err := db.conn.Query(query, roomId)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch room with subscribers: %w", err)
	}
	defer rows.Close()

	var room *Room
	for rows.Next() {
		var (
			roomId                int
			roomName              string
			externalId            string
			description           string
			seqId                 int
			ownerId               int
			roomCreatedAt         time.Time
			roomUpdatedAt         time.Time
			subscriptionId        sql.NullInt64
			accountId             sql.NullInt64
			username              sql.NullString
			subscriptionCreatedAt sql.NullTime
			subscriptionUpdatedAt sql.NullTime
		)

		err := rows.Scan(
			&roomId,
			&roomName,
			&externalId,
			&description,
			&seqId,
			&ownerId,
			&roomCreatedAt,
			&roomUpdatedAt,
			&subscriptionId,
			&accountId,
			&username,
			&subscriptionCreatedAt,
			&subscriptionUpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		if room == nil {
			room = &Room{
				Id:            roomId,
				Name:          roomName,
				ExternalId:    externalId,
				Description:   description,
				SeqId:         seqId,
				OwnerId:       ownerId,
				CreatedAt:     roomCreatedAt,
				UpdatedAt:     roomUpdatedAt,
				Subscriptions: make([]Subscription, 0),
			}
		}

		if accountId.Valid && username.Valid {
			room.Subscriptions = append(room.Subscriptions, Subscription{
				Id:        int(subscriptionId.Int64),
				AccountId: int(accountId.Int64),
				Username:  username.String,
				CreatedAt: subscriptionCreatedAt.Time,
				UpdatedAt: subscriptionUpdatedAt.Time,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	if room == nil {
		return nil, fmt.Errorf("room with id %d not found", roomId)
	}

	return room, nil
}

func (db *PgGoChatRepository) CreateRoom(params CreateRoomParams) (Room, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return Room{}, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()
	res := tx.QueryRow(
		"INSERT INTO rooms (name, external_id, description, owner_id) "+
			"VALUES ($1, $2, $3, $4) RETURNING id, name, external_id, description, owner_id, created_at, updated_at",
		params.Name,
		params.ExternalId,
		params.Description,
		params.OwnerId,
	)

	var room Room
	err = res.Scan(
		&room.Id,
		&room.Name,
		&room.ExternalId,
		&room.Description,
		&room.OwnerId,
		&room.CreatedAt,
		&room.UpdatedAt,
	)
	if err != nil {
		return Room{}, err
	}

	_, err = tx.Exec(
		createSubQuery,
		params.OwnerId,
		room.Id,
	)
	if err != nil {
		return Room{}, err
	}

	if err = tx.Commit(); err != nil {
		return Room{}, err
	}

	return room, err
}

func (db *PgGoChatRepository) DeleteRoom(id int) error {
	tx, err := db.conn.Begin()
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

	_, err = tx.Exec("DELETE FROM messages WHERE room_id = $1", id)
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM rooms WHERE id = $1", id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (db *PgGoChatRepository) CreateSubscription(userId, roomId int) (Subscription, error) {
	res := db.conn.QueryRow(
		createSubQuery,
		userId,
		roomId,
	)

	var sub Subscription
	err := res.Scan(
		&sub.Id,
		&sub.AccountId,
		&sub.RoomId,
	)

	return sub, err
}

func (db *PgGoChatRepository) SubscriptionExists(account_id, room_id int) bool {
	res := db.conn.QueryRow(
		"SELECT id FROM subscriptions WHERE account_id = $1 AND room_id = $2 LIMIT 1",
		account_id,
		room_id,
	)

	var sub Subscription
	err := res.Scan(
		&sub.Id,
	)

	return err == nil
}

func (db *PgGoChatRepository) ListSubscriptions(account_id int) ([]Subscription, error) {
	rows, err := db.conn.Query(
		"SELECT s.id, s.last_read_seq_id, s.created_at, s.updated_at, r.id AS room_id, r.external_id, "+
			"r.name, r.description, r.seq_id, r.created_at AS room_created_at, r.updated_at AS room_updated_at "+
			"FROM subscriptions s JOIN rooms r ON r.id = s.room_id WHERE s.account_id = $1",
		account_id,
	)

	if err != nil {
		return nil, err
	}

	var subs []Subscription
	for rows.Next() {
		var (
			sub  Subscription
			room Room
		)
		if err = rows.Scan(
			&sub.Id,
			&sub.LastReadSeqId,
			&sub.CreatedAt,
			&sub.UpdatedAt,
			&room.Id,
			&room.ExternalId,
			&room.Name,
			&room.Description,
			&room.SeqId,
			&room.CreatedAt,
			&room.UpdatedAt,
		); err != nil {
			break
		}

		sub.Room = room
		subs = append(subs, sub)
	}

	return subs, err
}

func (db *PgGoChatRepository) DeleteSubscription(accountId, roomId int) error {
	_, err := db.conn.Exec(
		"DELETE FROM subscriptions WHERE account_id = $1 AND room_id = $2",
		accountId,
		roomId,
	)

	return err
}

func (db *PgGoChatRepository) UpdateLastReadSeqId(userId, roomId, seqId int) error {
	_, err := db.conn.Exec(
		"UPDATE subscriptions SET last_read_seq_id = $1, updated_at = $2 "+
			"WHERE account_id = $3 AND room_id = $4",
		seqId,
		time.Now().UTC(),
		userId,
		roomId,
	)

	return err
}

func (db *PgGoChatRepository) CreateMessage(msg Message) error {
	tx, err := db.conn.Begin()
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	if err = db.UpdateRoomOnMessage(msg); err != nil {
		return fmt.Errorf("failed to update room on message: %w", err)
	}
	if _, err = db.conn.Exec(
		"INSERT INTO messages (seq_id, room_id, user_id, content, created_at, updated_at) "+
			"VALUES ($1, $2, $3, $4, $5, $6)",
		msg.SeqId,
		msg.RoomId,
		msg.UserId,
		msg.Content,
		msg.CreatedAt,
		msg.CreatedAt,
	); err != nil {
		return fmt.Errorf("failed to insert message: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return err
}

func (db *PgGoChatRepository) UpdateRoomOnMessage(msg Message) error {
	_, err := db.conn.Exec("UPDATE rooms SET seq_id = $1 WHERE id = $2", msg.SeqId, msg.RoomId)

	return err
}

func (db *PgGoChatRepository) GetSubscribersByRoomId(roomId int) ([]User, error) {
	rows, err := db.conn.Query(
		"SELECT a.id, a.username FROM subscriptions AS s "+
			"JOIN accounts AS a ON s.account_id = a.id WHERE s.room_id = $1",
		roomId,
	)

	var subs = make([]User, 0)
	for rows.Next() {
		var sub User
		if err = rows.Scan(&sub.Id, &sub.Username); err != nil {
			break
		}

		subs = append(subs, sub)
	}

	return subs, err
}

func (db *PgGoChatRepository) GetMessages(roomId, since, before, limit int) ([]Message, error) {
	var upper, lower int = 1<<31 - 1, 0
	if before > 0 {
		upper = before - 1
	}

	if since > 0 {
		lower = since
	}

	if limit <= 0 {
		limit = 20
	}

	rows, err := db.conn.Query(
		"SELECT id, seq_id, room_id, user_id, content, created_at FROM messages "+
			"WHERE room_id = $1 AND seq_id BETWEEN $2 AND $3 ORDER BY seq_id DESC LIMIT $4",
		roomId,
		lower,
		upper,
		limit,
	)

	if err != nil {
		return nil, err
	}

	var messages = make([]Message, 0, limit)
	for rows.Next() {
		var msg Message
		if err = rows.Scan(&msg.Id, &msg.SeqId, &msg.RoomId, &msg.UserId, &msg.Content, &msg.CreatedAt); err != nil {
			break
		}

		messages = append(messages, msg)
	}
	return messages, err
}
