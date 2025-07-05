package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewConfig(t *testing.T) {
	var (
		addr = "localhost:8080"
		dsn  = "host=localhost user=postgres password=postgres dbname=postgres sslmode=disable"
		key  = "c29tZV9zZWNyZXQ="
		orig = []string{"http://localhost:3000"}
	)

	tcases := []struct {
		name string
		addr string
		dsn  string
		key  string
		orig []string
		err  bool
	}{
		{
			name: "valid config",
			addr: addr,
			dsn:  dsn,
			key:  key,
			orig: orig,
			err:  false,
		},
		{
			name: "empty address",
			addr: "",
			dsn:  dsn,
			key:  key,
			orig: orig,
			err:  true,
		},
		{
			name: "empty DSN",
			addr: addr,
			dsn:  "",
			key:  key,
			orig: orig,
			err:  true,
		},
		{
			name: "empty signing key",
			addr: addr,
			dsn:  dsn,
			key:  "",
			orig: orig,
			err:  true,
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			config, err := NewConfig(tc.addr, tc.dsn, tc.key, tc.orig)
			if tc.err {
				assert.Error(t, err, "expected error for config: %s", tc.name)
				return
			}
			assert.NoError(t, err, "expected no error for config: %s", tc.name)

			assert.Equal(t, tc.addr, config.ServerAddr, "expected server address to match")
			assert.Equal(t, tc.dsn, config.DatabaseDSN, "expected database DSN to match")
			assert.Equal(t, tc.orig, config.AllowedOrigins, "expected allowed origins to match")
			assert.NotEmpty(t, config.SigningKey, "expected signing key to be decoded and not empty")
		})
	}
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
