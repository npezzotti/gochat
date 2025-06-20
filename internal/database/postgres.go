package database

import (
	"database/sql"
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

func (db *PgGoChatRepository) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}
