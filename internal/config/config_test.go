package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewConfig(t *testing.T) {
	serverAddr := "localhost:8080"
	databaseDSN := "host=localhost user=postgres password=postgres dbname=postgres sslmode=disable"
	base64Secret := "c29tZV9zZWNyZXQ=" // "some_secret" in base64
	allowedOrigins := []string{"http://localhost:3000"}

	config, err := NewConfig(serverAddr, databaseDSN, base64Secret, allowedOrigins)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assert.Equal(t, serverAddr, config.ServerAddr, "expected server address to match")
	assert.Equal(t, databaseDSN, config.DatabaseDSN, "expected database DSN to match")
	assert.Equal(t, allowedOrigins, config.AllowedOrigins, "expected allowed origins to match")
	assert.NotEmpty(t, config.SigningKey, "expected signing key to be decoded and not empty")
	assert.Equal(t, "some_secret", string(config.SigningKey), "expected signing key to match")
	assert.Equal(t, 1, len(config.AllowedOrigins), "expected allowed origins to have one entry")
	assert.Equal(t, "http://localhost:3000", config.AllowedOrigins[0], "expected allowed origin to match")
	assert.Equal(t, []string{"http://localhost:3000"}, config.AllowedOrigins, "expected allowed origins to match")
}

func Test_decodeSigningKey(t *testing.T) {
	tcases := []struct {
		name         string
		base64Secret string
		expectedKey  []byte
		expectError  bool
	}{
		{
			name:         "valid base64 secret",
			base64Secret: "c29tZV9zZWNyZXQ=", //
			expectedKey:  []byte("some_secret"),
			expectError:  false,
		},
		{
			name:         "invalid base64 secret",
			base64Secret: "invalid_base64",
			expectedKey:  nil,
			expectError:  true,
		},
		{
			name:         "empty base64 secret",
			base64Secret: "",
			expectedKey:  nil,
			expectError:  true,
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			key, err := decodeSigningSecret(tc.base64Secret)
			if tc.expectError {
				assert.Error(t, err, "expected error for base64 secret: %s", tc.base64Secret)
			} else {
				assert.NoError(t, err, "expected no error for base64 secret: %s", tc.base64Secret)
				assert.Equal(t, tc.expectedKey, key, "expected decoded key to match for base64 secret: %s", tc.base64Secret)
			}
		})
	}
}
