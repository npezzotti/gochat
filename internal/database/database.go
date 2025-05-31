package database

import (
	"database/sql"
)

var DB *DBConn

type DBConn struct {
	conn *sql.DB
}

func NewDatabaseConnection(dsn string) (*DBConn, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &DBConn{conn: db}, nil
}

func (db *DBConn) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}
