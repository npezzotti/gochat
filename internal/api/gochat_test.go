package api

import (
	"net/http"
	"testing"

	"github.com/npezzotti/go-chatroom/internal/config"
	"github.com/npezzotti/go-chatroom/internal/database"
	"github.com/npezzotti/go-chatroom/internal/server"
	"github.com/npezzotti/go-chatroom/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestNewGoChatApp(t *testing.T) {
	mux := http.NewServeMux()
	logger := testutil.TestLogger(t)
	cs := &server.ChatServer{}
	db := &database.MockGoChatRepository{}
	cfg := &config.Config{
		ServerAddr:     "localhost:8080",
		DatabaseDSN:    "dsn",
		SigningKey:     []byte("secret"),
		AllowedOrigins: []string{"http://localhost:3000"},
	}

	app := NewGoChatApp(mux, logger, cs, db, nil, cfg)

	assert.NotNil(t, app, "expected app to be initialized")
	assert.NotNil(t, app.mux, "expected mux to be initialized")
	assert.NotNil(t, app.log, "expected logger to be set")
	assert.NotNil(t, app.db, "expected db to be set")
	assert.NotNil(t, app.cs, "expected chat server to be set")
	assert.Equal(t, app.log, logger, "expected logger to be set")
	assert.Equal(t, app.db, db, "expected db to be set")
	assert.Equal(t, app.cs, cs, "expected chat server to be set")
	assert.Equal(t, app.signingKey, cfg.SigningKey, "expected signing key to be set")
	assert.Equal(t, app.mux.Addr, cfg.ServerAddr, "expected server address to match config")
}
