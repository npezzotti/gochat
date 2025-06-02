package config

import "testing"

func TestNewConfig(t *testing.T) {
	serverAddr := "localhost:8080"
	databaseDSN := "host=localhost user=postgres password=postgres dbname=postgres sslmode=disable"
	base64Secret := "c29tZV9zZWNyZXQ="
	allowedOrigins := []string{"http://localhost:3000"}

	config, err := NewConfig(serverAddr, databaseDSN, base64Secret, allowedOrigins)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if config.ServerAddr != serverAddr {
		t.Errorf("expected server address %s, got %s", serverAddr, config.ServerAddr)
	}
	if config.DatabaseDSN != databaseDSN {
		t.Errorf("expected database DSN %s, got %s", databaseDSN, config.DatabaseDSN)
	}
	if string(config.SigningKey) != "some_secret" {
		t.Errorf("expected signing key %s, got %s", "some_secret", string(config.SigningKey))
	}
	if len(config.AllowedOrigins) != 1 || config.AllowedOrigins[0] != "http://localhost:3000" {
		t.Errorf("expected allowed origins %v, got %v", []string{"http://localhost:3000"}, config.AllowedOrigins)
	}
}

func Test_decodeSigningKey(t *testing.T) {
	base64Secret := "c29tZV9zZWNyZXQ="
	expectedKey := []byte("some_secret")
	key, err := decodeSigningSecret(base64Secret)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(key) != string(expectedKey) {
		t.Errorf("expected signing key %s, got %s", expectedKey, key)
	}
	if _, err := decodeSigningSecret("invalid_base64"); err == nil {
		t.Error("expected error for invalid base64, got nil")
	}
	if _, err := decodeSigningSecret(""); err == nil {
		t.Error("expected error for empty base64 string, got nil")
	}
}
