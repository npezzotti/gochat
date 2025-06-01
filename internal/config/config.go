package config

import (
	"encoding/base64"
	"fmt"
)

type Config struct {
	DatabaseDSN string
	ServerAddr  string
	SigningKey  []byte
}

func decodeSigningSecret(base64Secret string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(base64Secret)
}

func NewConfig(serverAddr, databaseDSN, base64Secret string) (*Config, error) {
	if serverAddr == "" {
		return nil, fmt.Errorf("server address cannot be empty")
	}
	if databaseDSN == "" {
		return nil, fmt.Errorf("database DSN cannot be empty")
	}
	if base64Secret == "" {
		return nil, fmt.Errorf("signing secret cannot be empty")
	}

	// Decode the base64 encoded signing secret
	signingKey, err := decodeSigningSecret(base64Secret)
	if err != nil {
		return nil, fmt.Errorf("decode signing secret: %w", err)
	}

	return &Config{
		DatabaseDSN: databaseDSN,
		ServerAddr:  ":8000",
		SigningKey:  signingKey,
	}, nil
}
