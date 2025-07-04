package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserId(t *testing.T) {
	tcases := []struct {
		name     string
		ctx      context.Context
		userId   int
		expected bool
	}{
		{
			name:     "no user ID",
			ctx:      context.Background(),
			expected: false,
		},
		{
			name:     "user ID set",
			ctx:      WithUserId(context.Background(), 42),
			userId:   42,
			expected: true,
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			userId, ok := UserId(tc.ctx)
			assert.Equal(t, tc.expected, ok, "expected UserId to return %v", tc.expected)
			assert.Equal(t, tc.userId, userId, "expected UserId to return %d", tc.userId)
		})
	}
}
