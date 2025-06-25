package server

import (
	"net/http"
	"testing"
	"time"

	"github.com/npezzotti/go-chatroom/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestGetUserId(t *testing.T) {
	t.Run("extracts id from UserId", func(t *testing.T) {
		cm := &ClientMessage{
			BaseMessage: BaseMessage{
				Id:        1,
				Timestamp: Now(),
			},
			UserId: 42,
		}

		res := cm.GetUserId()
		assert.Equal(t, 42, res, "expected UserId to be returned directly")
	})

	t.Run("extracts id from client", func(t *testing.T) {
		cm := &ClientMessage{
			BaseMessage: BaseMessage{
				Id:        1,
				Timestamp: Now(),
			},
			client: &Client{
				user: types.User{
					Id: 42,
				},
			},
		}

		res := cm.GetUserId()
		assert.Equal(t, 42, res, "expected UserId to be extracted from client user")
	})
}

func TestNoErrOk(t *testing.T) {
	expected := &ServerMessage{
		BaseMessage: BaseMessage{
			Id:        1,
			Timestamp: Now(),
		},
		Response: &Response{
			ResponseCode: http.StatusOK,
			Data: map[string]any{
				"testkey": "testvalue",
			},
		},
	}

	result := NoErrOK(1, map[string]any{
		"testkey": "testvalue",
	})

	assert.NotNil(t, result, "expected result to be non-nil")
	assert.NotNil(t, result.Response, "expected response to be non-nil")
	assert.Equal(t, expected.Id, result.Id, "expected Id to match")
	assert.WithinDuration(t, expected.Timestamp, result.Timestamp, time.Duration(time.Second), "expected Timestamp to be within 1 second")
	assert.Equal(t, expected.Response.ResponseCode, result.Response.ResponseCode, "expected ResponseCode to match")
	assert.Equal(t, expected.Response.Data, result.Response.Data, "expected Data to match")
}

func TestNoErrAccepted(t *testing.T) {
	expected := &ServerMessage{
		BaseMessage: BaseMessage{
			Id:        1,
			Timestamp: Now(),
		},
		Response: &Response{
			ResponseCode: http.StatusAccepted,
		},
	}

	result := NoErrAccepted(1)

	assert.NotNil(t, result, "expected result to be non-nil")
	assert.NotNil(t, result.Response, "expected response to be non-nil")
	assert.Equal(t, expected.Id, result.Id, "expected Id to match")
	assert.WithinDuration(t, expected.Timestamp, result.Timestamp, time.Duration(time.Second), "expected Timestamp to be within 1 seconds")
	assert.Equal(t, expected.Response.ResponseCode, result.Response.ResponseCode, "expected ResponseCode to match")
}

func TestErrRoomNotFound(t *testing.T) {
	expected := &ServerMessage{
		BaseMessage: BaseMessage{
			Id:        1,
			Timestamp: Now(),
		},
		Response: &Response{
			ResponseCode: http.StatusNotFound,
			Error:        "room not found",
		},
	}

	result := ErrRoomNotFound(1)

	assert.NotNil(t, result, "expected result to be non-nil")
	assert.NotNil(t, result.Response, "expected response to be non-nil")
	assert.Equal(t, expected.Id, result.Id, "expected Id to match")
	assert.WithinDuration(t, expected.Timestamp, result.Timestamp, time.Duration(time.Second), "expected Timestamp to be within 1 second")
	assert.Equal(t, expected.Response.ResponseCode, result.Response.ResponseCode, "expected ResponseCode to match")
	assert.Equal(t, expected.Response.Error, result.Response.Error, "expected Error message to match")
}

func TestErrSubscriptionNotFound(t *testing.T) {
	expected := &ServerMessage{
		BaseMessage: BaseMessage{
			Id:        1,
			Timestamp: Now(),
		},
		Response: &Response{
			ResponseCode: http.StatusNotFound,
			Error:        "subscription not found",
		},
	}

	result := ErrSubscriptionNotFound(1)

	assert.NotNil(t, result, "expected result to be non-nil")
	assert.NotNil(t, result.Response, "expected response to be non-nil")
	assert.Equal(t, expected.Id, result.Id, "expected Id to match")
	assert.WithinDuration(t, expected.Timestamp, result.Timestamp, time.Duration(time.Second), "expected Timestamp to be within 1 second")
	assert.Equal(t, expected.Response.ResponseCode, result.Response.ResponseCode, "expected ResponseCode to match")
	assert.Equal(t, expected.Response.Error, result.Response.Error, "expected Error message to match")
}

func TestErrInternalError(t *testing.T) {
	expected := &ServerMessage{
		BaseMessage: BaseMessage{
			Id:        1,
			Timestamp: Now(),
		},
		Response: &Response{
			ResponseCode: http.StatusInternalServerError,
			Error:        "internal server error",
		},
	}

	result := ErrInternalError(1)

	assert.NotNil(t, result, "expected result to be non-nil")
	assert.NotNil(t, result.Response, "expected response to be non-nil")
	assert.Equal(t, expected.Id, result.Id, "expected Id to match")
	assert.WithinDuration(t, expected.Timestamp, result.Timestamp, time.Duration(time.Second), "expected Timestamp to be within 1 second")
	assert.Equal(t, expected.Response.ResponseCode, result.Response.ResponseCode, "expected ResponseCode to match")
	assert.Equal(t, expected.Response.Error, result.Response.Error)
}

func TestErrServiceUnavailable(t *testing.T) {
	expected := &ServerMessage{
		BaseMessage: BaseMessage{
			Id:        1,
			Timestamp: Now(),
		},
		Response: &Response{
			ResponseCode: http.StatusServiceUnavailable,
			Error:        "service unavailable",
		},
	}

	result := ErrServiceUnavailable(1)

	assert.NotNil(t, result, "expected result to be non-nil")
	assert.NotNil(t, result.Response, "expected response to be non-nil")
	assert.Equal(t, expected.Id, result.Id, "expected Id to match")
	assert.WithinDuration(t, expected.Timestamp, result.Timestamp, time.Duration(time.Second), "expected Timestamp to be within 1 second")
	assert.Equal(t, expected.Response.ResponseCode, result.Response.ResponseCode, "expected ResponseCode to match")
	assert.Equal(t, expected.Response.Error, result.Response.Error)
}

func TestErrorInvalidMessage(t *testing.T) {
	expected := &ServerMessage{
		BaseMessage: BaseMessage{
			Timestamp: Now(),
		},
		Response: &Response{
			ResponseCode: http.StatusBadRequest,
			Error:        "invalid message format",
		},
	}

	result := ErrInvalidMessage(0)
	assert.NotNil(t, result, "expected result to be non-nil")
	assert.NotNil(t, result.Response, "expected response to be non-nil")
	assert.Equal(t, 0, result.Id, "expected Id to be zero")
	assert.WithinDuration(t, expected.Timestamp, result.Timestamp, time.Duration(time.Second), "expected Timestamp to be within 1 second")
	assert.Equal(t, expected.Response.ResponseCode, result.Response.ResponseCode, "expected ResponseCode to match")
	assert.Equal(t, expected.Response.Error, result.Response.Error, "expected Error message to match")

	// Additional test: when id > 0, it should be set
	expectedWithId := &ServerMessage{
		BaseMessage: BaseMessage{
			Id:        42,
			Timestamp: Now(),
		},
		Response: &Response{
			ResponseCode: http.StatusBadRequest,
			Error:        "invalid message format",
		},
	}
	resultWithId := ErrInvalidMessage(42)
	assert.NotNil(t, resultWithId, "expected result to be non-nil")
	assert.NotNil(t, resultWithId.Response, "expected response to be non-nil")
	assert.Equal(t, expectedWithId.Id, resultWithId.Id, "expected Id to match")
	assert.WithinDuration(t, expectedWithId.Timestamp, resultWithId.Timestamp, time.Duration(time.Second), "expected Timestamp to be within 1 second")
	assert.Equal(t, expectedWithId.Response.ResponseCode, resultWithId.Response.ResponseCode, "expected ResponseCode to match")
	assert.Equal(t, expectedWithId.Response.Error, resultWithId.Response.Error, "expected Error message to match")
}
