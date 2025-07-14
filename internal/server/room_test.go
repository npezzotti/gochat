package server

import (
	"database/sql"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/npezzotti/go-chatroom/internal/database"
	"github.com/npezzotti/go-chatroom/internal/stats"
	"github.com/npezzotti/go-chatroom/internal/testutil"
	"github.com/npezzotti/go-chatroom/internal/types"
	"github.com/stretchr/testify/assert"
)

func Test_addClient_getClient_deleteClient(t *testing.T) {
	room := &Room{
		externalId: "test-room",
		clients:    make(map[*Client]struct{}),
		userMap:    make(map[int]map[*Client]struct{}),
	}

	c := &Client{user: types.User{Id: 1, Username: "testuser"}, rooms: make(map[string]*Room)}
	room.addClient(c)
	assert.Lenf(t, room.clients, 1, "expected 1 client after adding, got %d", len(room.clients))
	assert.Contains(t, room.clients, c, "expected room.clients to contain client")
	assert.Containsf(t, room.userMap, c.user.Id, "expected userMap to contain entry for user ID %d", c.user.Id)

	retrievedClient, ok := room.getClient(c)
	assert.True(t, ok, "expected to retrieve client by user ID")
	assert.Equal(t, c, retrievedClient, "expected retrieved client to match added client")

	room.deleteClient(c)
	assert.Lenf(t, room.clients, 0, "expected 0 clients after deletion, got %d", len(room.clients))
	assert.NotContainsf(t, room.userMap, c.user.Id, "expected userMap not to contain entry for user ID %d after deletion", c.user.Id)
	assert.NotContains(t, room.clients, c, "expected room.clients to not contain client after deletion")
}

func Test_handleRoomTimeout(t *testing.T) {
	t.Run("successfully unloads room", func(t *testing.T) {
		room := &Room{
			externalId: "test-room",
			cs:         newTestChatServer(t, &database.MockGoChatRepository{}, &stats.MockStatsUpdater{}),
			log:        testutil.TestLogger(t),
		}

		room.handleRoomTimeout()
		select {
		case req := <-room.cs.unloadRoomChan:
			assert.Equal(t, "test-room", req.roomId, "expected room ID to match")
			assert.Equal(t, false, req.deleted, "expected deleted flag to be false")
		default:
			t.Error("timeout: handleRoomTimeout did not send unload request")
		}
	})

	t.Run("unload channel is full", func(t *testing.T) {
		room := &Room{
			externalId: "test-room",
			cs:         newTestChatServer(t, &database.MockGoChatRepository{}, &stats.MockStatsUpdater{}),
			log:        testutil.TestLogger(t),
			killTimer:  time.NewTimer(time.Duration(0)),
		}

		<-room.killTimer.C // Ensure the timer is stopped before testing

		room.cs.unloadRoomChan = make(chan unloadRoomRequest, 1)                            // Limit channel size to 1
		room.cs.unloadRoomChan <- unloadRoomRequest{roomId: "another-room", deleted: false} // Fill the channel

		room.handleRoomTimeout()
		assert.True(t, room.killTimer.Stop(), "expected kill timer to be started after failed unload request")
	})
}

func Test_handleRoomExit(t *testing.T) {
	t.Run("exit room with no clients", func(t *testing.T) {
		room := &Room{
			externalId: "testroom",
			clients:    make(map[*Client]struct{}),
			userMap:    make(map[int]map[*Client]struct{}),
			log:        testutil.TestLogger(t),
		}

		done := make(chan string)
		go room.handleRoomExit(exitReq{deleted: false, done: done})

		// No clients should be notified, but the done channel should still be called
		select {
		case id := <-done:
			if id != room.externalId {
				t.Errorf("expected room ID %s, got %s", room.externalId, id)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout: handleRoomExit did not complete")
		}
	})

	t.Run("exit room with clients", func(t *testing.T) {
		room := &Room{
			externalId: "testroom",
			clients:    make(map[*Client]struct{}),
			userMap:    make(map[int]map[*Client]struct{}),
			log:        testutil.TestLogger(t),
		}

		c := &Client{user: types.User{Id: 1, Username: "user1"}, send: make(chan *ServerMessage, 256), rooms: make(map[string]*Room), exitRoom: make(chan string)}
		room.addClient(c)

		done := make(chan string)
		go room.handleRoomExit(exitReq{deleted: false, done: done})

		// Client should receive exit notification
		select {
		case <-c.exitRoom:
			// client should receive exit notification
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout: client did not receive exit notification")
		}

		select {
		case id := <-done:
			if id != room.externalId {
				t.Errorf("expected room ID %s, got %s", room.externalId, id)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout: handleRoomExit did not complete")
		}
	})

	t.Run("exit room with presence notification", func(t *testing.T) {
		cs := newTestChatServer(t, &database.MockGoChatRepository{}, &stats.MockStatsUpdater{})
		room := &Room{
			externalId: "testroom",
			subscribers: []types.User{
				{Id: 1, Username: "testsubscriber"},
			},
			cs:  cs,
			log: testutil.TestLogger(t),
		}

		done := make(chan string)
		go room.handleRoomExit(exitReq{deleted: false, done: done})

		select {
		case id := <-done:
			if id != room.externalId {
				t.Errorf("expected room ID %s, got %s", room.externalId, id)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout: handleRoomExit did not complete")
		}

		// Check that the server broadcast channel receives a presence notification
		select {
		case msg := <-cs.broadcastChan:
			if msg == nil || msg.Notification == nil || msg.Notification.Presence == nil {
				t.Error("expected presence notification, got nil")
			} else if msg.Notification.Presence.RoomId != room.externalId {
				t.Errorf("expected presence notification for room %s, got %s", room.externalId, msg.Notification.Presence.RoomId)
			}
		default:
			t.Error("expected broadcast channel to receive presence notification, but did not")
		}
	})

	t.Run("exit room with deleted flag", func(t *testing.T) {
		room := &Room{
			externalId: "testroom",
			clients:    make(map[*Client]struct{}),
			userMap:    make(map[int]map[*Client]struct{}),
			log:        testutil.TestLogger(t),
		}

		c := &Client{send: make(chan *ServerMessage, 256), rooms: make(map[string]*Room), exitRoom: make(chan string)}
		room.addClient(c)

		done := make(chan string)
		go room.handleRoomExit(exitReq{deleted: true, done: done})

		select {
		case <-c.exitRoom:
			// client should receive exit notification
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout: client did not receive exit notification")
		}

		select {
		case id := <-done:
			if id != room.externalId {
				t.Errorf("expected room ID %s, got %s", room.externalId, id)
			}
		case <-time.After(200 * time.Millisecond):
			t.Error("timeout: handleRoomExit did not complete")
		}

		// Check that the client receives a room deleted notification
		select {
		case msg := <-c.send:
			if msg == nil || msg.Notification == nil || msg.Notification.RoomDeleted == nil {
				t.Error("expected room deleted notification, got nil")
			} else if msg.Notification.RoomDeleted.RoomId != room.externalId {
				t.Errorf("expected room deleted notification for room %s, got %s", room.externalId, msg.Notification.RoomDeleted.RoomId)
			}
		default:
			t.Error("expected client to receive room deleted notification, but did not")
		}
	})
}

func Test_handleLeave(t *testing.T) {
	t.Run("leave without unsubscribe", func(t *testing.T) {
		db := &database.MockGoChatRepository{}
		defer db.AssertExpectations(t)

		room := &Room{
			externalId: "testroom",
			clients:    make(map[*Client]struct{}),
			userMap:    make(map[int]map[*Client]struct{}),
			cs:         newTestChatServer(t, db, &stats.MockStatsUpdater{}),
			log:        testutil.TestLogger(t),
			killTimer:  time.NewTimer(idleRoomTimeout),
		}

		room.killTimer.Stop() // Stop the timer to prevent it from firing during the test

		c := &Client{
			user:     types.User{Id: 1, Username: "testuser"},
			send:     make(chan *ServerMessage, 256),
			rooms:    make(map[string]*Room),
			exitRoom: make(chan string),
		}
		room.addClient(c)
		c.addRoom(room)

		room.handleLeave(&ClientMessage{
			Leave: &Leave{
				RoomId:      room.externalId,
				Unsubscribe: false,
			},
			UserId: c.user.Id,
			client: c,
		})

		select {
		case msg := <-c.send:
			if msg == nil || msg.Response == nil {
				t.Errorf("expected response message, got nil")
			} else if msg.Response.ResponseCode != 200 {
				t.Errorf("expected response code 200, got %d", msg.Response.ResponseCode)
			}
		default:
			t.Error("expected client to receive response message, but did not")
		}

		assert.NotContains(t, room.clients, c, "expected client to be removed from room clients")
		assert.NotContains(t, c.rooms, room.externalId, "expected room to be removed from client's rooms")
		db.AssertNotCalled(t, "DeleteSubscription", c.user.Id, room.externalId, "expected no subscription deletion for leave without unsubscribe")
	})

	t.Run("leave without unsubscribe send presence notification", func(t *testing.T) {
		db := &database.MockGoChatRepository{}
		defer db.AssertExpectations(t)

		room := &Room{
			externalId: "testroom",
			clients:    make(map[*Client]struct{}),
			userMap:    make(map[int]map[*Client]struct{}),
			cs:         newTestChatServer(t, db, &stats.MockStatsUpdater{}),
			log:        testutil.TestLogger(t),
			killTimer:  time.NewTimer(idleRoomTimeout),
		}

		room.killTimer.Stop() // Stop the timer to prevent it from firing during the test

		c1 := &Client{
			user:     types.User{Id: 1, Username: "testuser"},
			send:     make(chan *ServerMessage, 256),
			rooms:    make(map[string]*Room),
			exitRoom: make(chan string),
		}

		room.addClient(c1)
		c1.addRoom(room)

		// Simulate another client in the room
		c2 := &Client{
			user:  types.User{Id: 2, Username: "anotheruser"},
			send:  make(chan *ServerMessage, 256),
			rooms: make(map[string]*Room),
		}
		room.addClient(c2)
		c2.addRoom(room)

		room.handleLeave(&ClientMessage{
			Leave: &Leave{
				RoomId:      room.externalId,
				Unsubscribe: false,
			},
			UserId: c1.user.Id,
			client: c1,
		})

		select {
		case msg := <-c2.send:
			if msg == nil || msg.Notification == nil || msg.Notification.Presence == nil {
				t.Errorf("expected response message, got nil")
			} else {
				assert.Equal(t, room.externalId, msg.Notification.Presence.RoomId, "expected presence notification for room %s, got %s", room.externalId, msg.Notification.Presence.RoomId)
				assert.Equal(t, c1.user.Id, msg.Notification.Presence.UserId, "expected presence notification for user %s, got %d", c1.user.Id, msg.Notification.Presence.UserId)
				assert.False(t, msg.Notification.Presence.Present, "expected presence to be false for leaving user")
			}
		default:
			t.Error("expected client to receive response message, but did not")
		}
	})

	t.Run("leave without unsubscribe not joined", func(t *testing.T) {
		db := &database.MockGoChatRepository{}
		defer db.AssertExpectations(t)

		room := &Room{
			externalId: "testroom",
			clients:    make(map[*Client]struct{}),
			userMap:    make(map[int]map[*Client]struct{}),
			cs:         newTestChatServer(t, db, &stats.MockStatsUpdater{}),
			log:        testutil.TestLogger(t),
			killTimer:  time.NewTimer(idleRoomTimeout),
		}

		room.killTimer.Stop()
		c := &Client{
			user:  types.User{Id: 1, Username: "testuser"},
			send:  make(chan *ServerMessage, 1),
			rooms: make(map[string]*Room),
		}

		assert.NotContains(t, room.clients, c, "expected client to not be added to room clients")

		room.handleLeave(&ClientMessage{
			Leave: &Leave{
				RoomId:      room.externalId,
				Unsubscribe: false,
			},
			UserId: c.user.Id,
			client: c,
		})

		select {
		case msg := <-c.send:
			if msg != nil && msg.Response != nil {
				assert.Equal(t, http.StatusNotFound, msg.Response.ResponseCode, "expected 404")
				assert.Equal(t, "room not found", msg.Response.Error, "expected room not found error message")
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout: client did not receive server response message")
		}
	})

	t.Run("leave with unsubscribe", func(t *testing.T) {
		db := &database.MockGoChatRepository{}
		defer db.AssertExpectations(t)

		room := &Room{
			id:         1,
			externalId: "testroom",
			subscribers: []types.User{
				{Id: 1, Username: "testuser"},
			},
			clients:   make(map[*Client]struct{}),
			userMap:   make(map[int]map[*Client]struct{}),
			db:        db,
			cs:        newTestChatServer(t, db, &stats.MockStatsUpdater{}),
			log:       testutil.TestLogger(t),
			killTimer: time.NewTimer(idleRoomTimeout),
		}

		room.killTimer.Stop()

		c := &Client{
			user:     types.User{Id: 1, Username: "testuser"},
			send:     make(chan *ServerMessage, 256),
			rooms:    make(map[string]*Room),
			exitRoom: make(chan string),
		}

		room.addClient(c)
		c.addRoom(room)

		db.On("DeleteSubscription", c.user.Id, room.id).Return(nil).Once()

		done := make(chan struct{})
		go func() {
			room.handleLeave(&ClientMessage{
				Leave: &Leave{
					RoomId:      room.externalId,
					Unsubscribe: true,
				},
				UserId: c.user.Id,
				client: c,
			})
			close(done)
		}()

		select {
		case <-c.exitRoom:
			// client should receive exit notification
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout: client did not receive exit notification")
		}

		select {
		case <-done:
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout: handleLeave did not complete")
		}

		select {
		case msg := <-c.send:
			if msg != nil && msg.Response != nil {
				assert.Equal(t, http.StatusOK, msg.Response.ResponseCode, "expected response code 200, got %d", msg.Response.ResponseCode)
			} else {
				t.Errorf("expected server response message, got nil")
			}
		default:
			t.Error("expected client to receive response message")
		}

		assert.NotContains(t, room.clients, c, "expected client to be removed from room clients")
		assert.NotContains(t, room.userMap, c.user.Id, "expected user for client to be removed from room's userMap")
		assert.NotContains(t, room.subscribers, c.user, "expected user to be removed from room subscribers")
	})

	t.Run("leave with unsubscribe and no subscription", func(t *testing.T) {
		db := &database.MockGoChatRepository{}
		defer db.AssertExpectations(t)

		room := &Room{
			id:        1,
			clients:   make(map[*Client]struct{}),
			userMap:   make(map[int]map[*Client]struct{}),
			db:        db,
			cs:        newTestChatServer(t, db, &stats.MockStatsUpdater{}),
			log:       testutil.TestLogger(t),
			killTimer: time.NewTimer(idleRoomTimeout),
		}

		room.killTimer.Stop()
		c := &Client{
			user:  types.User{Id: 1, Username: "testuser"},
			send:  make(chan *ServerMessage, 1),
			rooms: make(map[string]*Room),
		}

		room.addClient(c)
		c.addRoom(room)

		db.On("DeleteSubscription", c.user.Id, room.id).Return(sql.ErrNoRows).Once()

		room.handleLeave(&ClientMessage{
			BaseMessage: BaseMessage{
				Id:        1,
				Timestamp: Now(),
			},
			Leave: &Leave{
				RoomId:      room.externalId,
				Unsubscribe: true,
			},
			UserId: c.user.Id,
			client: c,
		})

		select {
		case msg := <-c.send:
			if msg != nil && msg.Response != nil {
				assert.Equal(t, http.StatusNotFound, msg.Response.ResponseCode)
				assert.Equal(t, "subscription not found", msg.Response.Error, "expected error message for subscription not found")
			} else {
				t.Error("expected server response message, got nil")
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout: client did not receive server response message")
		}
	})

	t.Run("leave with unsubscribe and database error", func(t *testing.T) {
		db := &database.MockGoChatRepository{}
		defer db.AssertExpectations(t)

		room := &Room{
			id:        1,
			clients:   make(map[*Client]struct{}),
			userMap:   make(map[int]map[*Client]struct{}),
			db:        db,
			cs:        newTestChatServer(t, db, &stats.MockStatsUpdater{}),
			log:       testutil.TestLogger(t),
			killTimer: time.NewTimer(idleRoomTimeout),
		}

		room.killTimer.Stop()
		c := &Client{
			user:  types.User{Id: 1, Username: "testuser"},
			send:  make(chan *ServerMessage, 1),
			rooms: make(map[string]*Room),
		}

		room.addClient(c)
		c.addRoom(room)

		db.On("DeleteSubscription", c.user.Id, room.id).Return(errors.New("db error")).Once()

		room.handleLeave(&ClientMessage{
			BaseMessage: BaseMessage{
				Id:        1,
				Timestamp: Now(),
			},
			Leave: &Leave{
				RoomId:      room.externalId,
				Unsubscribe: true,
			},
			UserId: c.user.Id,
			client: c,
		})

		select {
		case msg := <-c.send:
			if msg != nil && msg.Response != nil {
				assert.Equal(t, http.StatusInternalServerError, msg.Response.ResponseCode)
				assert.Equal(t, "internal server error", msg.Response.Error, "expected error message for DeleteSubscription failure")
			} else {
				t.Error("expected server response message, got nil")
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout: client did not receive server response message")
		}
	})
}

func Test_handleRead(t *testing.T) {
	t.Run("successful process of read message", func(t *testing.T) {
		db := &database.MockGoChatRepository{}
		defer db.AssertExpectations(t)

		client := &Client{
			send: make(chan *ServerMessage, 256),
		}

		msg := &ClientMessage{
			BaseMessage: BaseMessage{
				Id:        1,
				Timestamp: Now(),
			},
			Read: &Read{
				RoomId: "testroom",
				SeqId:  42,
			},
			UserId: 1,
			client: client,
		}

		room := &Room{
			id: 1,
			cs: newTestChatServer(t, db, &stats.MockStatsUpdater{}),
			db: db,
		}

		db.On("UpdateLastReadSeqId", msg.UserId, room.id, msg.Read.SeqId).Return(nil).Once()
		room.handleRead(msg)

		select {
		case response := <-client.send:
			assert.NotNil(t, response.Response, "expected response to be non-nil")
			assert.Equal(t, msg.Id, response.Id, "expected response ID to match request ID")
			assert.Equal(t, http.StatusOK, response.Response.ResponseCode, "expected response code 200")
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout: client did not receive response message")
		}
	})

	t.Run("failure with db error", func(t *testing.T) {
		db := &database.MockGoChatRepository{}
		defer db.AssertExpectations(t)

		client := &Client{
			send: make(chan *ServerMessage, 256),
		}

		msg := &ClientMessage{
			BaseMessage: BaseMessage{
				Id:        1,
				Timestamp: Now(),
			},
			Read: &Read{
				RoomId: "testroom",
				SeqId:  42,
			},
			UserId: 1,
			client: client,
		}

		room := &Room{
			id:  1,
			db:  db,
			cs:  newTestChatServer(t, db, &stats.MockStatsUpdater{}),
			log: testutil.TestLogger(t),
		}

		db.On("UpdateLastReadSeqId", msg.UserId, room.id, msg.Read.SeqId).Return(errors.New("db error")).Once()
		room.handleRead(msg)

		select {
		case response := <-client.send:
			assert.NotNil(t, response.Response, "expected response to be non-nil")
			assert.Equal(t, msg.Id, response.Id, "expected response ID to match request ID")
			assert.Equal(t, http.StatusInternalServerError, response.Response.ResponseCode, "expected response code 500")
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout: client did not receive response message")
		}
	})
}

func Test_handleJoin(t *testing.T) {
	t.Run("join subscription exists", func(t *testing.T) {
		db := &database.MockGoChatRepository{}
		defer db.AssertExpectations(t)

		cs := newTestChatServer(t, db, &stats.MockStatsUpdater{})

		room := &Room{
			id:         1,
			externalId: "testroom",
			clients:    make(map[*Client]struct{}),
			userMap:    make(map[int]map[*Client]struct{}),
			db:         db,
			cs:         cs,
			log:        testutil.TestLogger(t),
			killTimer:  time.NewTimer(idleRoomTimeout),
			subscribers: []types.User{
				{Id: 1, Username: "testuser"},
			},
		}

		room.killTimer.Stop()

		c := &Client{
			user:  types.User{Id: 1, Username: "testuser"},
			send:  make(chan *ServerMessage, 256),
			rooms: make(map[string]*Room),
		}

		now := Now()
		db.On("SubscriptionExists", 1, 1).Return(true, nil).Once()
		db.On("GetRoomWithSubscribers", 1).Return(&database.Room{
			Id:          1,
			Name:        "testroom",
			ExternalId:  "testroom",
			Description: "testroom description",
			SeqId:       0,
			OwnerId:     1,
			CreatedAt:   now,
			UpdatedAt:   now,
			Subscriptions: []database.Subscription{
				{
					Id:        1,
					AccountId: 1,
					Username:  "testuser",
					CreatedAt: now,
					UpdatedAt: now,
				},
			},
		}, nil).Once()

		room.handleJoin(&ClientMessage{
			BaseMessage: BaseMessage{
				Id:        1,
				Timestamp: Now(),
			},
			Join: &Join{
				RoomId: room.externalId,
			},
			UserId: c.user.Id,
			client: c,
		})

		assert.Contains(t, room.clients, c, "expected client to be added to room clients")
		assert.Contains(t, c.rooms, room.externalId, "expected room to be added to client's rooms")
		assert.Contains(t, room.userMap[c.user.Id], c, "expected user for client to be added to room's userMap")

		expectedData := types.Room{
			Id:          1,
			Name:        "testroom",
			ExternalId:  "testroom",
			Description: "testroom description",
			SeqId:       0,
			OwnerId:     1,
			Subscribers: []types.User{
				{
					Id:        1,
					Username:  "testuser",
					IsPresent: true,
				},
			},
			CreatedAt: now,
			UpdatedAt: now,
		}
		select {
		case msg := <-c.send:
			assert.NotNil(t, msg.Response, "expected response message")
			assert.Equal(t, 1, msg.Id, "expected response id to match client message id")
			assert.Equalf(t, http.StatusOK, msg.Response.ResponseCode, "expected response code %d", http.StatusOK)
			assert.NotNil(t, room.externalId, msg.Response.Data, "expected response data to be non-nil")
			assert.EqualValuesf(t, expectedData, msg.Response.Data, "expected data to be %+v, got %+v", expectedData, msg.Response.Data)
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout: client did not receive response message")
		}
	})

	t.Run("join initial client", func(t *testing.T) {
		db := &database.MockGoChatRepository{}
		defer db.AssertExpectations(t)

		cs := newTestChatServer(t, db, &stats.MockStatsUpdater{})

		room := &Room{
			id:         1,
			externalId: "testroom",
			clients:    make(map[*Client]struct{}),
			userMap:    make(map[int]map[*Client]struct{}),
			db:         db,
			cs:         cs,
			log:        testutil.TestLogger(t),
			killTimer:  time.NewTimer(idleRoomTimeout),
			subscribers: []types.User{
				{Id: 1, Username: "testuser"},
			},
		}

		room.killTimer.Stop()

		c := &Client{
			user:  types.User{Id: 1, Username: "testuser"},
			send:  make(chan *ServerMessage, 256),
			rooms: make(map[string]*Room),
		}

		now := Now()
		db.On("SubscriptionExists", 1, 1).Return(true, nil).Once()
		db.On("GetRoomWithSubscribers", 1).Return(&database.Room{
			Id:          1,
			Name:        "testroom",
			ExternalId:  "testroom",
			Description: "testroom description",
			SeqId:       0,
			OwnerId:     1,
			CreatedAt:   now,
			UpdatedAt:   now,
			Subscriptions: []database.Subscription{
				{
					Id:        1,
					AccountId: 1,
					Username:  "testuser",
					CreatedAt: now,
					UpdatedAt: now,
				},
			},
		}, nil).Once()

		clientMsg := &ClientMessage{
			BaseMessage: BaseMessage{
				Id:        1,
				Timestamp: Now(),
			},
			Join: &Join{
				RoomId: room.externalId,
			},
			UserId: c.user.Id,
			client: c,
		}

		room.handleJoin(clientMsg)
	})

	t.Run("join notifies other clients in room", func(t *testing.T) {
		db := &database.MockGoChatRepository{}
		defer db.AssertExpectations(t)

		room := &Room{
			id:         1,
			externalId: "testroom",
			clients:    make(map[*Client]struct{}),
			userMap:    make(map[int]map[*Client]struct{}),
			db:         db,
			cs:         newTestChatServer(t, db, &stats.MockStatsUpdater{}),
			log:        testutil.TestLogger(t),
			killTimer:  time.NewTimer(idleRoomTimeout),
			subscribers: []types.User{
				{Id: 1, Username: "testuser"},
			},
		}

		room.killTimer.Stop()

		c1 := &Client{
			user:  types.User{Id: 1, Username: "testuser"},
			send:  make(chan *ServerMessage, 1),
			rooms: make(map[string]*Room),
		}
		room.addClient(c1)
		c1.addRoom(room)

		c2 := &Client{
			user:  types.User{Id: 2, Username: "anotheruser"},
			send:  make(chan *ServerMessage, 1),
			rooms: make(map[string]*Room),
		}

		now := Now()
		db.On("SubscriptionExists", c2.user.Id, room.id).Return(true, nil).Once()
		db.On("GetRoomWithSubscribers", room.id).Return(&database.Room{
			Id:          room.id,
			Name:        room.externalId,
			ExternalId:  room.externalId,
			Description: "testroom description",
			SeqId:       0,
			OwnerId:     c1.user.Id,
			CreatedAt:   now,
			UpdatedAt:   now,
			Subscriptions: []database.Subscription{
				{
					Id:        c1.user.Id,
					AccountId: c1.user.Id,
					Username:  c1.user.Username,
					CreatedAt: now,
					UpdatedAt: now,
				},
				{
					Id:        c2.user.Id,
					AccountId: c2.user.Id,
					Username:  c2.user.Username,
					CreatedAt: now,
					UpdatedAt: now,
				},
			},
		}, nil).Once()

		room.handleJoin(&ClientMessage{
			BaseMessage: BaseMessage{
				Id:        1,
				Timestamp: Now(),
			},
			Join: &Join{
				RoomId: room.externalId,
			},
			UserId: c2.user.Id,
			client: c2,
		})

		assert.Contains(t, room.clients, c2, "expected c2 to be added to room clients")
		assert.Contains(t, c2.rooms, room.externalId, "expected room to be added to c2's rooms")
		assert.Contains(t, room.userMap[c2.user.Id], c2, "expected user for c2 to be added to room's userMap")

		select {
		case msg := <-c1.send:
			assert.NotNil(t, msg.Notification, "expected notification message")
			assert.NotNil(t, msg.Notification.Presence, "expected presence notification")
			assert.Equal(t, c2.user.Id, msg.Notification.Presence.UserId, "expected presence notification for user %d", c2.user.Id)
			assert.Equal(t, room.externalId, msg.Notification.Presence.RoomId, "expected presence notification for room %s", room.externalId)
			assert.True(t, msg.Notification.Presence.Present, "expected presence to be true for joining user")
			assert.Equal(t, c2, msg.SkipClient, "expected presence notification to skip client that just joined")
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout: c1 did not receive presence notification for c2 joining")
		}
	})

	t.Run("join with no subscription", func(t *testing.T) {
		db := &database.MockGoChatRepository{}
		defer db.AssertExpectations(t)

		room := &Room{
			id:         1,
			externalId: "testroom",
			clients:    make(map[*Client]struct{}, 0),
			userMap:    make(map[int]map[*Client]struct{}),
			db:         db,
			cs:         newTestChatServer(t, db, &stats.MockStatsUpdater{}),
			log:        testutil.TestLogger(t),
			killTimer:  time.NewTimer(idleRoomTimeout),
			subscribers: []types.User{
				{Id: 2},
			},
		}

		room.killTimer.Stop()

		c1 := &Client{
			user:  types.User{Id: 1, Username: "testuser"},
			send:  make(chan *ServerMessage, 256),
			rooms: make(map[string]*Room, 0),
		}

		// Create an active subscriber in the room
		c2 := &Client{
			user:  types.User{Id: 2, Username: "anotheruser"},
			send:  make(chan *ServerMessage, 256),
			rooms: make(map[string]*Room, 0),
		}
		room.addClient(c2)
		c2.addRoom(room)

		db.On("SubscriptionExists", c1.user.Id, room.id).Return(false, nil).Once()
		db.On("CreateSubscription", c1.user.Id, room.id).Return(database.Subscription{
			Id:        2,
			AccountId: c1.user.Id,
			RoomId:    room.id,
		}, nil).Once()

		db.On("GetRoomWithSubscribers", room.id).Return(&database.Room{
			Id:          room.id,
			Name:        room.externalId,
			ExternalId:  room.externalId,
			Description: "testroom description",
			SeqId:       0,
			OwnerId:     c2.user.Id,
			CreatedAt:   Now(),
			UpdatedAt:   Now(),
			Subscriptions: []database.Subscription{
				{
					Id:        1,
					AccountId: c2.user.Id,
					Username:  c2.user.Username,
					CreatedAt: Now(),
					UpdatedAt: Now(),
				},
				{
					Id:        2,
					AccountId: c1.user.Id,
					Username:  c1.user.Username,
					CreatedAt: Now(),
					UpdatedAt: Now(),
				},
			},
		}, nil).Once()

		room.handleJoin(&ClientMessage{
			BaseMessage: BaseMessage{
				Id:        1,
				Timestamp: Now(),
			},
			Join: &Join{
				RoomId: room.externalId,
			},
			UserId: c1.user.Id,
			client: c1,
		})

		assert.Contains(t, room.clients, c1, "expected client to be added to room clients")
		assert.Contains(t, c1.rooms, room.externalId, "expected room to be added to client's rooms")
		assert.Contains(t, room.userMap[c1.user.Id], c1, "expected user for client to be added to room's userMap")
		assert.Equalf(t, 2, len(room.subscribers), "expected room to have 2 subscribers, got %d", len(room.subscribers))
		assert.Containsf(t, room.subscribers, types.User{Id: c1.user.Id}, "expected user to be added to room subscribers, got %+v", room.subscribers)

		// Check that c2 receives a notification about c1 subscribing
		// It should not not receive a presence notification for the room since it is joined.
		assert.Equal(t, 1, len(c2.send), "expected c2 to receive 1 message after c1 joins")

		select {
		case msg := <-c2.send:
			assert.NotNil(t, msg, "expected broadcast message to be non-nil")
			assert.NotNil(t, msg.Notification, "expected notification message")
			assert.NotNil(t, msg.Notification.SubscriptionChange, "expected subscription change notification")
			assert.Equal(t, c1.user, msg.Notification.SubscriptionChange.User, "expected subscription change notification for user %+v", c1.user)
			assert.Equal(t, room.externalId, msg.Notification.SubscriptionChange.RoomId, "expected subscription change notification for room %s", room.externalId)
			assert.True(t, msg.Notification.SubscriptionChange.Subscribed, "expected subscription change notification to be true")
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout: c2 did not receive subscription change notification for c1 joining")
		}
	})

	t.Run("join existing subscription with db error", func(t *testing.T) {
		db := &database.MockGoChatRepository{}
		defer db.AssertExpectations(t)

		room := &Room{
			id:         1,
			externalId: "testroom",
			clients:    make(map[*Client]struct{}),
			userMap:    make(map[int]map[*Client]struct{}),
			db:         db,
			cs:         newTestChatServer(t, db, &stats.MockStatsUpdater{}),
			log:        testutil.TestLogger(t),
			killTimer:  time.NewTimer(idleRoomTimeout),
			subscribers: []types.User{
				{Id: 1, Username: "testuser"},
			},
		}

		room.killTimer.Stop()

		c := &Client{
			user:  types.User{Id: 1, Username: "testuser"},
			rooms: make(map[string]*Room),
			send:  make(chan *ServerMessage, 256),
		}

		db.On("SubscriptionExists", c.user.Id, room.id).Return(true).Once()
		db.On("GetRoomWithSubscribers", room.id).Return(nil, errors.New("db error")).Once()

		room.handleJoin(&ClientMessage{
			BaseMessage: BaseMessage{
				Id:        1,
				Timestamp: Now(),
			},
			Join: &Join{
				RoomId: room.externalId,
			},
			UserId: c.user.Id,
			client: c,
		})

		// Check that the killTimer was started
		assert.True(t, room.killTimer.Stop(), "expected room's killTimer to be started after join failure")

		// Check that the client was not added to the room
		assert.NotContains(t, room.clients, c, "expected client to not be added to room clients")
		assert.NotContains(t, c.rooms, room.externalId, "expected room to not be added to client's rooms")
		assert.NotContains(t, room.userMap, c.user.Id, "expected user for client to not be added to room's userMap")
		assert.Len(t, c.send, 1, "expected client to receive a response message")

		// Check that the client received an error response
		select {
		case msg := <-c.send:
			assert.NotNil(t, msg, "expected response messages")
			assert.NotNil(t, msg.Response, "expected response to be non-nil")
			assert.Equal(t, http.StatusInternalServerError, msg.Response.ResponseCode, "expected response code 500")
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout: client did not receive response message")
		}
	})

	t.Run("join with no subscription and db error", func(t *testing.T) {
		db := &database.MockGoChatRepository{}
		defer db.AssertExpectations(t)

		room := &Room{
			id:         1,
			externalId: "testroom",
			clients:    make(map[*Client]struct{}),
			userMap:    make(map[int]map[*Client]struct{}),
			db:         db,
			cs:         newTestChatServer(t, db, &stats.MockStatsUpdater{}),
			log:        testutil.TestLogger(t),
			killTimer:  time.NewTimer(idleRoomTimeout),
			subscribers: []types.User{
				{Id: 2, Username: "anotheruser"},
			},
		}

		room.killTimer.Stop()

		c := &Client{
			user:  types.User{Id: 1, Username: "testuser"},
			rooms: make(map[string]*Room),
			send:  make(chan *ServerMessage, 256),
		}

		db.On("SubscriptionExists", c.user.Id, room.id).Return(false, nil).Once()
		db.On("CreateSubscription", c.user.Id, room.id).Return(database.Subscription{}, errors.New("db error")).Once()

		room.handleJoin(&ClientMessage{
			BaseMessage: BaseMessage{
				Id:        1,
				Timestamp: Now(),
			},
			Join: &Join{
				RoomId: room.externalId,
			},
			UserId: c.user.Id,
			client: c,
		})

		// Check that the killTimer was started
		assert.True(t, room.killTimer.Stop(), "expected room's killTimer to be started after join failure")

		// Check that the client was not added to the room
		assert.NotContains(t, room.clients, c, "expected client to not be added to room clients")
		assert.NotContains(t, c.rooms, room.externalId, "expected room to not be added to client's rooms")
		assert.NotContains(t, room.userMap, c.user.Id, "expected user for client to not be added to room's userMap")

		assert.Len(t, c.send, 1, "expected client to receive a response message")

		// Check that the client received an error response
		select {
		case msg := <-c.send:
			assert.NotNil(t, msg, "expected response message")
			assert.NotNil(t, msg.Response, "expected response to be non-nil")
			assert.Equal(t, http.StatusInternalServerError, msg.Response.ResponseCode, "expected response code 500")
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout: client did not receive response message")
		}
	})

	t.Run("failed join to room with active clients", func(t *testing.T) {
		db := &database.MockGoChatRepository{}
		defer db.AssertExpectations(t)

		room := &Room{
			id:         1,
			externalId: "testroom",
			clients:    make(map[*Client]struct{}),
			userMap:    make(map[int]map[*Client]struct{}),
			db:         db,
			cs:         newTestChatServer(t, db, &stats.MockStatsUpdater{}),
			log:        testutil.TestLogger(t),
			killTimer:  time.NewTimer(idleRoomTimeout),
			subscribers: []types.User{
				{Id: 2, Username: "activeuser"},
			},
		}

		room.killTimer.Stop()

		activeClient := &Client{
			user:  types.User{Id: 2, Username: "activeuser"},
			rooms: make(map[string]*Room),
		}
		room.addClient(activeClient)
		activeClient.addRoom(room)

		c := &Client{
			user:  types.User{Id: 1, Username: "testuser"},
			rooms: make(map[string]*Room),
			send:  make(chan *ServerMessage, 256),
		}

		db.On("SubscriptionExists", c.user.Id, room.id).Return(true).Once()
		db.On("GetRoomWithSubscribers", room.id).Return(nil, errors.New("db error")).Once()

		room.handleJoin(&ClientMessage{
			BaseMessage: BaseMessage{
				Id:        1,
				Timestamp: Now(),
			},
			Join: &Join{
				RoomId: room.externalId,
			},
			UserId: c.user.Id,
			client: c,
		})

		// Check that the killTimer was started
		assert.False(t, room.killTimer.Stop(), "expected room's killTimer to have not been started")
	})
}

func Test_removeClientSession(t *testing.T) {
	t.Run("remove single client in room", func(t *testing.T) {
		room := &Room{
			externalId: "testroom",
			clients:    make(map[*Client]struct{}),
			userMap:    make(map[int]map[*Client]struct{}),
			log:        testutil.TestLogger(t),
			killTimer:  time.NewTimer(idleRoomTimeout),
		}

		room.killTimer.Stop()

		c := &Client{
			user:  types.User{Id: 1, Username: "testuser"},
			rooms: make(map[string]*Room),
		}

		room.addClient(c)
		c.addRoom(room)
		room.removeSession(c)

		assert.Len(t, room.clients, 0, "expected 0 clients in room after removal")
		assert.NotContains(t, room.clients, c, "expected client to be removed from room clients")
		assert.NotContains(t, c.rooms, room.externalId, "expected room to be removed from client's rooms")
		assert.NotContains(t, room.userMap, c.user.Id, "expected user for client to be removed from room's userMap")
		assert.True(t, room.killTimer.Stop(), "expected killTimer to have been in started after removing only client")
	})

	t.Run("remove client when multiple clients in room", func(t *testing.T) {
		room := &Room{
			externalId: "testroom",
			clients:    make(map[*Client]struct{}),
			userMap:    make(map[int]map[*Client]struct{}),
			log:        testutil.TestLogger(t),
			killTimer:  time.NewTimer(idleRoomTimeout),
		}

		room.killTimer.Stop()

		c1 := &Client{
			user:  types.User{Id: 1, Username: "testuser1"},
			rooms: make(map[string]*Room),
		}
		room.addClient(c1)
		c1.addRoom(room)

		c2 := &Client{
			user:  types.User{Id: 2, Username: "testuser2"},
			rooms: make(map[string]*Room),
		}
		room.addClient(c2)
		c2.addRoom(room)

		room.removeSession(c1)

		assert.Equal(t, 1, len(room.clients), "expected 1 client in room after removal")
		assert.NotContains(t, room.clients, c1, "expected c1 to be removed from room clients")
		assert.Contains(t, room.clients, c2, "expected c2 to remain in room clients")
		assert.Contains(t, c2.rooms, room.externalId, "expected room to remain in c2's rooms")
		assert.NotContains(t, c1.rooms, room.externalId, "expected room to be removed from c1's rooms")
		assert.NotContains(t, room.userMap[c1.user.Id], c1, "expected user for c1 to be removed from room's userMap")
		assert.Contains(t, room.userMap[c2.user.Id], c2, "expected user for c2 to remain in room's userMap")
		assert.False(t, room.killTimer.Stop(), "expected killTime to be in stopped state after removing c1")
	})
}

func Test_removeAllClientsForUser(t *testing.T) {
	room := &Room{
		externalId: "testroom",
		clients:    make(map[*Client]struct{}),
		userMap:    make(map[int]map[(*Client)]struct{}),
		log:        testutil.TestLogger(t),
		killTimer:  time.NewTimer(idleRoomTimeout),
	}

	room.killTimer.Stop()

	userToRemove := types.User{Id: 1, Username: "testuser"}
	var clients []*Client
	for range 3 { // Create three clients for the same user
		clients = append(clients, NewClient(userToRemove, nil, nil, nil, &stats.MockStatsUpdater{}))
	}

	// Add another user
	anotherClient := NewClient(types.User{Id: 2, Username: "anotheruser"}, nil, nil, nil, &stats.MockStatsUpdater{})
	clients = append(clients, anotherClient)

	for _, c := range clients {
		room.addClient(c)
		c.addRoom(room)
	}

	done := make(chan struct{})
	go func() {
		room.removeAllSessionsForUser(userToRemove.Id)
		close(done)
	}()

	for _, c := range clients {
		if c.user.Id == userToRemove.Id {
			//  All clients for this user should receive one exit notification
			select {
			case id := <-c.exitRoom:
				assert.Equal(t, id, room.externalId, "expected client to receive room id in exitRoom channel")
			case <-time.After(100 * time.Millisecond):
				t.Error("timeout: expected client to receive exit notification")
			}

			assert.Len(t, c.send, 0, "expected client to receive one exit notification")
		}
	}

	// Wait for the removeAllSessionsForUser to complete
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout: removeAllSessionsForUser did not complete")
	}

	// Check that the room's clients and userMap are empty
	assert.Equalf(t, 1, len(room.clients), "expected 1 client in room after removing all clients for user with id %d", userToRemove.Id)
	assert.Containsf(t, room.clients, anotherClient, "expected client for user with id %d to remain in room clients", anotherClient.user.Id)
	assert.NotContainsf(t, room.userMap, userToRemove.Id, "expected user map not to contain user with id %d after removal", userToRemove.Id)
	assert.Containsf(t, room.userMap, anotherClient.user.Id, "expected user map to contain user with id %d after removal", anotherClient.user.Id)

	// Check that all clients were removed from the room's clients
	for _, c := range clients {
		if c.user.Id == userToRemove.Id {
			assert.NotContains(t, room.clients, c, "expected client to be removed from room clients")
		} else {
			assert.Contains(t, room.clients, c, "expected another client to remain in room clients")
		}
	}
}

func Test_removeSubscriber(t *testing.T) {
	sub1 := types.User{Id: 1, Username: "testuser"}
	sub2 := types.User{Id: 2, Username: "anotheruser"}
	room := &Room{
		externalId:  "testroom",
		subscribers: []types.User{sub1, sub2},
		log:         testutil.TestLogger(t),
	}

	room.removeSubscriber(1)
	assert.Len(t, room.subscribers, 1, "expected 1 subscriber after removal")
	assert.NotContains(t, room.subscribers, 1, "expected subscriber with ID 1 to be removed")
	assert.Contains(t, room.subscribers, sub2, "expected subscriber with ID 2 to remain")
}

func Test_saveAndBroadcast(t *testing.T) {
	t.Run("save and broadcast message", func(t *testing.T) {
		db := &database.MockGoChatRepository{}
		defer db.AssertExpectations(t)

		cs := newTestChatServer(t, db, &stats.MockStatsUpdater{})

		room := &Room{
			id:         1,
			externalId: "testroom",
			clients:    make(map[*Client]struct{}),
			userMap:    make(map[int]map[*Client]struct{}),
			db:         db,
			cs:         cs,
			log:        testutil.TestLogger(t),
			seq_id:     0,
			killTimer:  time.NewTimer(idleRoomTimeout),
			subscribers: []types.User{
				{Id: 2, Username: "inactive-subscriber"},
			},
		}

		room.killTimer.Stop()

		c := &Client{
			user:  types.User{Id: 1, Username: "user1"},
			send:  make(chan *ServerMessage, 256),
			rooms: make(map[string]*Room),
			log:   room.log,
		}

		room.addClient(c)
		msg := &ClientMessage{
			BaseMessage: BaseMessage{
				Id:        1,
				Timestamp: Now(),
			},
			Publish: &Publish{
				RoomId:  room.externalId,
				Content: "Hello, world!",
			},
			UserId: c.user.Id,
			client: c,
		}

		db.On("CreateMessage", database.Message{
			SeqId:     1,
			RoomId:    room.id,
			UserId:    msg.client.user.Id,
			Content:   msg.Publish.Content,
			CreatedAt: msg.Timestamp,
		}).Return(nil).Once()

		room.saveAndBroadcast(msg)

		// The client should first receive an OK server response, then a server publish response.
		select {
		case resp := <-c.send:
			assert.NotNil(t, resp, "expected first message to be non-nil")
			assert.NotNil(t, resp.Response, "expected first message to be a server response")
			assert.Equal(t, http.StatusAccepted, resp.Response.ResponseCode, "expected first message to be accepted response")
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout: client did not receive first server response message")
		}

		select {
		case pub := <-c.send:
			assert.NotNil(t, pub, "expected second message to be non-nil")
			assert.NotNil(t, pub.Message, "expected second message to be a publish message")
			assert.Equal(t, msg.Publish.Content, pub.Message.Content, "expected published content to match")
			assert.Equal(t, c.user.Id, pub.Message.UserId, "expected published user id to match")
			assert.Equal(t, room.id, pub.Message.RoomId, "expected published room id to match")
			assert.Equal(t, room.seq_id, pub.Message.SeqId, "expected published seq_id to match")
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout: client did not receive publish message")
		}

		assert.Equal(t, 1, room.seq_id, "expected seq_id to be incremented after saving message")
	})

	t.Run("save and broadcast message with error", func(t *testing.T) {
		db := &database.MockGoChatRepository{}
		defer db.AssertExpectations(t)

		cs := newTestChatServer(t, db, &stats.MockStatsUpdater{})

		room := &Room{
			id:         1,
			externalId: "testroom",
			clients:    make(map[*Client]struct{}),
			userMap:    make(map[int]map[*Client]struct{}),
			db:         db,
			cs:         cs,
			log:        testutil.TestLogger(t),
			seq_id:     0,
			killTimer:  time.NewTimer(idleRoomTimeout),
			subscribers: []types.User{
				{Id: 2, Username: "inactive-subscriber"},
			},
		}

		room.killTimer.Stop()

		c := &Client{
			user:  types.User{Id: 1, Username: "user1"},
			send:  make(chan *ServerMessage, 256),
			rooms: make(map[string]*Room),
			log:   room.log,
		}

		room.addClient(c)

		msg := &ClientMessage{
			BaseMessage: BaseMessage{
				Id:        1,
				Timestamp: Now(),
			},
			Publish: &Publish{
				RoomId:  room.externalId,
				Content: "Hello, world!",
			},
			UserId: c.user.Id,
			client: c,
		}

		db.On("CreateMessage", database.Message{
			SeqId:     1,
			RoomId:    room.id,
			UserId:    msg.client.user.Id,
			Content:   msg.Publish.Content,
			CreatedAt: msg.Timestamp,
		}).Return(errors.New("db error")).Once()

		room.saveAndBroadcast(msg)

		// The client should receive an error response
		select {
		case resp := <-c.send:
			assert.NotNil(t, resp, "expected response message to be non-nil")
			assert.NotNil(t, resp.Response, "expected response message to be a server response")
			assert.Equal(t, http.StatusInternalServerError, resp.Response.ResponseCode, "expected response code to be 500")
			assert.Equal(t, "internal server error", resp.Response.Error, "expected error message to match")
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout: client did not receive server response message")
		}

		assert.Equal(t, 0, room.seq_id, "expected seq_id to remain unchanged after error")
	})
}

func Test_broadcast(t *testing.T) {
	r := &Room{
		externalId: "testroom",
		clients:    make(map[*Client]struct{}),
		userMap:    make(map[int]map[*Client]struct{}),
		log:        testutil.TestLogger(t),
	}

	c1 := &Client{user: types.User{Id: 1, Username: "user1"}, send: make(chan *ServerMessage, 256), rooms: make(map[string]*Room), log: r.log}
	c2 := &Client{user: types.User{Id: 2, Username: "user2"}, send: make(chan *ServerMessage, 256), rooms: make(map[string]*Room), log: r.log}

	r.addClient(c1)
	r.addClient(c2)

	t.Run("broadcast to all clients", func(t *testing.T) {
		msg := &ServerMessage{}

		r.broadcast(msg)

		select {
		case m := <-c1.send:
			if m != msg {
				t.Errorf("expected c1 to receive message, got %v", m)
			}
		default:
			t.Error("expected c1 to receive message, but did not")
		}

		select {
		case m := <-c2.send:
			if m != msg {
				t.Errorf("expected c2 to receive message, got %v", m)
			}
		default:
			t.Error("expected c2 to receive message, but did not")
		}
	})

	t.Run("skip client in broadcast", func(t *testing.T) {
		msg := &ServerMessage{SkipClient: c1}
		r.broadcast(msg)

		select {
		case <-c1.send:
			t.Error("expected client 1 to not receive message")
		default:
			// client 1 should not receive the message
		}

		select {
		case m := <-c2.send:
			if m != msg {
				t.Errorf("expected client 2 to receive message, got %v", m)
			}
		default:
			t.Error("expected client 2 to receive message, but did not")
		}
	})
}
