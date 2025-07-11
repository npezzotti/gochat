package server

import (
	"testing"
	"time"

	"github.com/npezzotti/go-chatroom/internal/database"
	"github.com/npezzotti/go-chatroom/internal/stats"
	"github.com/npezzotti/go-chatroom/internal/testutil"
	"github.com/npezzotti/go-chatroom/internal/types"
	"github.com/stretchr/testify/assert"
)

func Test_queueMessage(t *testing.T) {
	t.Run("successful queue", func(t *testing.T) {
		c := &Client{
			send: make(chan *ServerMessage, 1),
			log:  testutil.TestLogger(t),
		}

		res := c.queueMessage(&ServerMessage{})
		assert.True(t, res, "expected queueMessage to return true when channel is not full")

		select {
		case msg := <-c.send:
			assert.NotNil(t, msg, "expected a message to be sent to the client")
		default:
			t.Error("expected a message to be sent to the client, but none was sent")
		}
	})
	t.Run("channel full", func(t *testing.T) {
		c := &Client{
			send: make(chan *ServerMessage, 1),
			log:  testutil.TestLogger(t),
		}

		c.send <- &ServerMessage{} // Pre-fill the send channel to simulate a full channel
		res := c.queueMessage(&ServerMessage{})
		assert.False(t, res, "expected queueMessage to return false when channel is full")
	})
}

func Test_serializeMessage(t *testing.T) {
	// Test the serialization of a message
	message := &ServerMessage{
		BaseMessage: BaseMessage{
			Id:        1,
			Timestamp: Now(),
		},
		Response: &Response{
			ResponseCode: 200,
			Data:         "test data",
		},
	}

	// Ensure the timestamp is in the expected format
	expected := `{"id":1,"timestamp":"` + message.Timestamp.Format(time.RFC3339Nano) +
		`","response":{"response_code":200,"data":"test data"}}`

	bytes, err := serializeMessage(message)
	assert.NoError(t, err, "expected no error during serialization")
	assert.Equal(t, expected, string(bytes), "expected serialized message to match the expected format")
}

func Test_stopClient(t *testing.T) {
	c := &Client{
		stop: make(chan struct{}),
	}

	c.stopClient()

	select {
	case <-c.stop:
		// Channel is closed as expected
	default:
		t.Error("expected stop channel to be closed")
	}
}

func Test_leaveAllRooms(t *testing.T) {
	rooms := []*Room{
		{
			externalId: "room1",
			leaveChan:  make(chan *ClientMessage, 1),
		},
		{
			externalId: "room2",
			leaveChan:  make(chan *ClientMessage, 1),
		},
	}

	c := &Client{
		rooms: make(map[string]*Room),
	}

	for _, room := range rooms {
		c.addRoom(room)
	}

	c.leaveAllRooms()

	for _, room := range rooms {
		assert.Len(t, room.leaveChan, 1, "expected 1 leave message to be sent to room %s", room.externalId)

		select {
		case msg := <-room.leaveChan:
			assert.NotNil(t, msg, "expected leave message to be sent for room %s", room.externalId)
			assert.NotNil(t, msg.Leave, "expected leave message")
			assert.Equal(t, room.externalId, msg.Leave.RoomId, "expected leave message for room %s")
			assert.Equal(t, c.user.Id, msg.UserId, "expected leave message to include user ID %d", c.user.Id)
			assert.Equal(t, c, msg.client, "expected leave message to include client")
		default:
			t.Errorf("expected leave message to be sent for room %s, but it was not", room.externalId)
		}
	}
}

func Test_joinRoom(t *testing.T) {
	cs := newTestChatServer(t, &database.MockGoChatRepository{}, &stats.MockStatsUpdater{})
	t.Run("successful join", func(t *testing.T) {
		c := NewClient(types.User{
			Id:       1,
			Username: "testuser",
		}, nil, cs, testutil.TestLogger(t), &stats.MockStatsUpdater{})

		joinMsg := &ClientMessage{
			BaseMessage: BaseMessage{
				Id:        1,
				Timestamp: Now(),
			},
			Join: &Join{
				RoomId: "testroom",
			},
			UserId: c.user.Id,
			client: c,
		}

		c.joinRoom(joinMsg)

		select {
		case msg := <-cs.joinChan:
			assert.NotNil(t, msg, "expected message to be sent to chat server join channel")
			assert.NotNil(t, msg.Join, "expected join message to be sent to chat server join channel")
			assert.Equal(t, msg.Id, joinMsg.Id, "expected join message ID to match")
			assert.Equal(t, joinMsg.Join.RoomId, msg.Join.RoomId, "expected join message to have correct room ID")
			assert.Equal(t, c.user.Id, msg.UserId, "expected join message to have correct user ID")
			assert.Equal(t, c, msg.client, "expected join message to have correct client reference")
		default:
			t.Error("expected join message to be sent to chat server join channel, but it was not")
		}
	})

	t.Run("join room channel full", func(t *testing.T) {
		cs := newTestChatServer(t, &database.MockGoChatRepository{}, &stats.MockStatsUpdater{})
		cs.joinChan = make(chan *ClientMessage, 1) // Limit the channel to one message for this test

		c := NewClient(types.User{
			Id:       1,
			Username: "testuser",
		}, nil, cs, testutil.TestLogger(t), &stats.MockStatsUpdater{})

		// Fill the join channel to simulate a full channel
		c.chatServer.joinChan <- &ClientMessage{}

		joinMsg := &ClientMessage{
			BaseMessage: BaseMessage{
				Id:        1,
				Timestamp: Now(),
			},
			Join: &Join{
				RoomId: "testroom",
			},
			UserId: c.user.Id,
			client: c,
		}

		c.joinRoom(joinMsg)

		select {
		case msg := <-c.send:
			assert.NotNil(t, msg, "expected a message to be sent to the client")
			assert.NotNil(t, msg.Response, "expected response to be non-nil")
			assert.Equal(t, joinMsg.Id, msg.Id, "expected response ID to match join message ID")
			assert.Equal(t, 503, msg.Response.ResponseCode, "expected response code to be 503")
		default:
			t.Error("expected a message to be sent to the client, but none was sent")
		}
	})
}

func Test_leaveRoom(t *testing.T) {
	t.Run("leave room success", func(t *testing.T) {
		c := &Client{
			user: types.User{
				Id:       1,
				Username: "testuser",
			},
			rooms: make(map[string]*Room),
		}

		room := &Room{
			externalId: "testroom",
			leaveChan:  make(chan *ClientMessage, 1),
		}

		c.addRoom(room)

		c.leaveRoom(&ClientMessage{
			BaseMessage: BaseMessage{
				Id:        1,
				Timestamp: Now(),
			},
			Leave: &Leave{
				RoomId: room.externalId,
			},
			UserId: c.user.Id,
			client: c,
		})

		select {
		case msg := <-room.leaveChan:
			assert.NotNil(t, msg, "expected message to be sent to room leave channel")
			assert.NotNil(t, msg.Leave, "expected leave message")
			assert.Equal(t, 1, msg.Id, "expected leave message id to match")
			assert.Equal(t, room.externalId, msg.Leave.RoomId, "expected leave message to have correct room id")
			assert.Equal(t, c.user.Id, msg.UserId, "expected leave message to have correct user ID")
			assert.Equal(t, c, msg.client, "expected leave message to have correct client reference")
		default:
			t.Error("expected message to be sent to chat server leave channel")
		}
	})

	t.Run("leave room not found", func(t *testing.T) {
		c := &Client{
			user: types.User{
				Id:       1,
				Username: "testuser",
			},
			rooms: make(map[string]*Room),
			send:  make(chan *ServerMessage, 1),
		}

		c.leaveRoom(&ClientMessage{
			BaseMessage: BaseMessage{
				Id:        1,
				Timestamp: Now(),
			},
			Leave: &Leave{
				RoomId: "notfound",
			},
			UserId: c.user.Id,
			client: c,
		})

		select {
		case msg := <-c.send:
			assert.NotNil(t, msg, "expected a message to be sent to the client")
			assert.NotNil(t, msg.Response, "expected response to be non-nil")
			assert.Equal(t, 1, msg.Id, "expected response id to match leave message id")
			assert.Equal(t, 404, msg.Response.ResponseCode, "expected response code to be 404")
		default:
			t.Error("expected a message to be sent to the client, but none was sent")
		}
	})

	t.Run("room unavailable", func(t *testing.T) {
		room := &Room{
			externalId: "unavailable",
			leaveChan:  make(chan *ClientMessage, 1),
		}

		room.leaveChan <- &ClientMessage{} // Pre-fill the leave channel to simulate a full channel

		c := &Client{
			user: types.User{
				Id:       1,
				Username: "testuser",
			},
			rooms: make(map[string]*Room),
			send:  make(chan *ServerMessage, 1),
			log:   testutil.TestLogger(t),
		}

		// Add the room to the client
		c.addRoom(room)
		c.leaveRoom(&ClientMessage{
			BaseMessage: BaseMessage{
				Id:        1,
				Timestamp: Now(),
			},
			Leave: &Leave{
				RoomId: room.externalId,
			},
			UserId: c.user.Id,
			client: c,
		})

		select {
		case msg := <-c.send:
			assert.NotNil(t, msg, "expected a message to be sent to the client")
			assert.NotNil(t, msg.Response, "expected response to be non-nil")
			assert.Equal(t, 1, msg.Id, "expected response id to match leave message id")
			assert.Equal(t, 503, msg.Response.ResponseCode, "expected response code to be 503")
		default:
			t.Error("expected a message to be sent to the client, but none was sent")
		}
	})
}

func Test_addRoom_delRoom_getRoom(t *testing.T) {
	c := &Client{
		rooms: make(map[string]*Room),
	}

	room := &Room{
		externalId: "testroom",
	}

	c.addRoom(room)
	r, ok := c.getRoom(room.externalId)
	assert.True(t, ok, "expected room to be found after adding")
	assert.NotNil(t, r, "expected room to not be nil after addin")
	assert.Equal(t, room.externalId, r.externalId, "expected room external id to match")

	c.delRoom(r.externalId)
	assert.NotContains(t, c.rooms, r.externalId, "expected room to be removed after deletion")
}
