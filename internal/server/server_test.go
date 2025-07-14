package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/npezzotti/go-chatroom/internal/database"
	"github.com/npezzotti/go-chatroom/internal/stats"
	"github.com/npezzotti/go-chatroom/internal/testutil"
	"github.com/npezzotti/go-chatroom/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// newTestChatServer creates a new ChatServer instance for testing purposes
func newTestChatServer(t *testing.T, db database.GoChatRepository, su *stats.MockStatsUpdater) *ChatServer {
	su.On("RegisterMetric", mock.Anything).Return(nil).Times(4)

	logger := testutil.TestLogger(t)
	cs, err := NewChatServer(logger, db, su)
	if err != nil {
		t.Fatalf("failed to create test ChatServer: %v", err)
	}
	return cs
}

func TestNewChatServer(t *testing.T) {
	db := &database.MockGoChatRepository{}
	defer db.AssertExpectations(t)

	su := &stats.MockStatsUpdater{}
	defer su.AssertExpectations(t)
	su.On("RegisterMetric", mock.Anything).Return(nil).Times(4)

	logger := testutil.TestLogger(t)
	cs, err := NewChatServer(logger, db, su)
	assert.NoError(t, err, "expected no error creating ChatServer")
	assert.NotNil(t, cs, "expected ChatServer to be non-nil")
	assert.Equal(t, logger, cs.log, "expected logger to be set")
	assert.Equal(t, db, cs.db, "expected database repository to be set")
	assert.NotNil(t, cs.joinChan, "expected joinChan to be initialized")
	assert.NotNil(t, cs.unloadRoomChan, "expected unloadRoomChan to be initialized")
	assert.NotNil(t, cs.broadcastChan, "expected broadcastChan to be initialized")
	assert.NotNil(t, cs.stop, "expected stop channel to be initialized")
	assert.NotNil(t, cs.clients, "expected clients map to be initialized")
	assert.NotNil(t, cs.userMap, "expected userMap to be initialized")
}

func TestChatServerShutdown(t *testing.T) {
	t.Run("successful shutdown", func(t *testing.T) {
		cs := newTestChatServer(t, &database.MockGoChatRepository{}, &stats.MockStatsUpdater{})

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		go func() {
			select {
			case req := <-cs.stop:
				assert.NotNil(t, req.done, "expected done channel in stop request")
				close(req.done) // Signal that shutdown is complete
			case <-time.After(100 * time.Millisecond):
				t.Error("expected signal on stop chan")
			}
		}()

		err := cs.Shutdown(ctx)
		assert.NoError(t, err, "expected successful shutdown without error")
	})

	t.Run("fails with context deadline exceeded", func(t *testing.T) {
		cs := newTestChatServer(t, &database.MockGoChatRepository{}, &stats.MockStatsUpdater{})

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		go func() {
			select {
			case <-cs.stop:
				// do not close cs.done to simulate a hang
			case <-time.After(100 * time.Millisecond):
				t.Error("expected signal on stop chan")
			}
		}()

		err := cs.Shutdown(ctx)
		assert.ErrorIs(t, err, context.DeadlineExceeded, "expected context deadline exceeded error, got %v", err)
	})
}

func TestChatServerShutdown_Integration(t *testing.T) {
	t.Run("successful shutdown with no rooms", func(t *testing.T) {
		cs := newTestChatServer(t, &database.MockGoChatRepository{}, &stats.MockStatsUpdater{})
		go cs.Run()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		err := cs.Shutdown(ctx)
		assert.NoError(t, err, "expected successful shutdown without error")
	})

	t.Run("successful shutdown with active rooms", func(t *testing.T) {
		su := &stats.MockStatsUpdater{}
		su.On("Incr", "NumActiveRooms").Once()
		su.On("Decr", "NumActiveRooms").Once()
		defer su.AssertExpectations(t)

		cs := newTestChatServer(t, &database.MockGoChatRepository{}, su)
		go cs.Run()

		// Create an active room to test shutdown behavior
		room := &Room{
			externalId: "testroom",
			exit:       make(chan exitReq, 1),
			log:        cs.log,
		}

		cs.addRoom(room.externalId, room)
		go room.start()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		err := cs.Shutdown(ctx)
		assert.NoError(t, err, "expected successful shutdown with active rooms")

		// Ensure the room is unloaded
		_, ok := cs.getRoom(room.externalId)
		assert.False(t, ok, "expected room to be unloaded after shutdown")
	})
}

func TestChatServer_addClient_removeClient(t *testing.T) {
	su := &stats.MockStatsUpdater{}
	su.On("Incr", "NumActiveClients").Once()
	su.On("Decr", "NumActiveClients").Once()
	defer su.AssertExpectations(t)

	cs := newTestChatServer(t, &database.MockGoChatRepository{}, su)
	user := types.User{Id: 1, Username: "testuser"}
	client := &Client{user: user}
	cs.addClient(client)
	assert.Len(t, cs.clients, 1, "expected 1 client after adding")
	assert.Contains(t, cs.clients, client, "expected client to be added to clients map")
	assert.Len(t, cs.userMap, 1, "expected userMap to have 1 entry")
	assert.Len(t, cs.userMap[user.Id], 1, "expected userMap to have 1 client for user")
	assert.Contains(t, cs.userMap[user.Id], client, "expected userMap to contain client")

	cs.removeClient(client)
	assert.Len(t, cs.clients, 0, "expected 0 client after removing")
	assert.NotContains(t, cs.clients, client, "expected client to be removed from clients map")
	assert.Nil(t, cs.userMap[user.Id], "expected userMap to not contain user after removing client")
	assert.Len(t, cs.userMap, 0, "expected userMap to be empty after removing client")
}

func Test_getClients(t *testing.T) {
	user := types.User{Id: 1, Username: "testuser"}
	tcases := []struct {
		name    string
		user    types.User
		clients []*Client
	}{
		{
			name: "single client",
			user: user,
			clients: []*Client{
				{user: user},
			},
		},
		{
			name: "multiple clients",
			user: user,
			clients: []*Client{
				{user: user},
				{user: user},
			},
		},
		{
			name:    "no clients",
			user:    user,
			clients: []*Client{},
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			su := &stats.MockStatsUpdater{}
			if len(tc.clients) > 0 {
				su.On("Incr", "NumActiveClients").Times(len(tc.clients))
			}
			defer su.AssertExpectations(t)

			cs := newTestChatServer(t, &database.MockGoChatRepository{}, su)

			for _, client := range tc.clients {
				cs.addClient(client)
			}

			clients := cs.getClients(user.Id)
			assert.Len(t, clients, len(tc.clients), "expected %d clients for user", len(tc.clients))

			for _, client := range tc.clients {
				assert.Contains(t, clients, client, "expected %v to be in clients list", client)
			}
		})
	}
}

func TestChatServer_addRoom_getRoom_removeRoom(t *testing.T) {
	su := &stats.MockStatsUpdater{}
	su.On("Incr", "NumActiveRooms").Once()
	su.On("Decr", "NumActiveRooms").Once()
	defer su.AssertExpectations(t)

	cs := newTestChatServer(t, &database.MockGoChatRepository{}, su)
	room := &Room{externalId: "testroom"}

	cs.addRoom("testroom", room)
	r, exists := cs.roomsMap.Load("testroom")
	assert.True(t, exists, "expected room to be added")
	assert.NotNil(t, r, "expected room to be non-nil")
	assert.Equal(t, room, r, "expected added room to match retrieved room")
	assert.Equal(t, 1, cs.numRooms, "expected numRooms to be 1 after adding room")

	got, ok := cs.getRoom("testroom")
	assert.True(t, ok, "expected room to be found")
	assert.Equal(t, room, got, "expected retrieved room to match added room")

	cs.removeRoom("testroom")
	_, ok = cs.getRoom("testroom")
	assert.False(t, ok, "expected room to be removed")
	assert.Equal(t, 0, cs.numRooms, "expected numRooms to be 0 after removing room")
}

func TestChatServer_handleBroadcast(t *testing.T) {
	t.Run("successful broadcast", func(t *testing.T) {
		su := &stats.MockStatsUpdater{}
		su.On("Incr", "NumActiveClients").Once()
		defer su.AssertExpectations(t)

		cs := newTestChatServer(t, &database.MockGoChatRepository{}, su)

		client := &Client{user: types.User{Id: 1, Username: "testuser"}, send: make(chan *ServerMessage, 1)}
		cs.addClient(client)

		msg := &ServerMessage{UserId: 1}
		cs.handleBroadcast(msg)
		assert.Len(t, client.send, 1, "expected 1 message to be queued to client")

		select {
		case clientMsg := <-client.send:
			assert.NotNil(t, clientMsg, "expected message to be queued to client")
			assert.Equal(t, clientMsg, msg, "expected messages to match")
		default:
			t.Error("expected message to be queued to client, but none was sent")
		}
	})

	t.Run("successful broadcast skip client", func(t *testing.T) {
		su := &stats.MockStatsUpdater{}
		su.On("Incr", "NumActiveClients").Twice()
		defer su.AssertExpectations(t)

		cs := newTestChatServer(t, &database.MockGoChatRepository{}, su)
		user := types.User{Id: 1, Username: "testuser"}

		client1 := &Client{user: user, send: make(chan *ServerMessage, 1)}
		client2 := &Client{user: user, send: make(chan *ServerMessage, 1)}
		cs.addClient(client1)
		cs.addClient(client2)

		msg := &ServerMessage{UserId: 1, SkipClient: client2}
		cs.handleBroadcast(msg)

		assert.Len(t, client1.send, 1, "expected 1 message to be queued to client1")
		assert.Len(t, client2.send, 0, "expected no messages to be queued to client2")

		select {
		case <-client1.send:
			// ok, message sent to client1
		default:
			t.Error("expected message to be queued to client1")
		}

		select {
		case <-client2.send:
			t.Error("expected message to be skipped for client2")
		default:
			// ok, no message sent to client2
		}
	})
}

func TestChatServerRegisterClient(t *testing.T) {
	t.Run("register client", func(t *testing.T) {
		db := &database.MockGoChatRepository{}
		defer db.AssertExpectations(t)

		db.On("ListSubscriptions", 1).Return([]database.Subscription{
			{Room: database.Room{ExternalId: "testroom"}},
		}, nil).Once()

		su := &stats.MockStatsUpdater{}
		su.On("Incr", "NumActiveRooms").Once()
		su.On("Incr", "NumActiveClients").Once()
		defer su.AssertExpectations(t)

		cs := newTestChatServer(t, db, su)
		room := &Room{externalId: "testroom"}

		// Simulate an active room a client is subscribed to
		cs.addRoom(room.externalId, room)

		// Register a client
		client := &Client{
			user: types.User{Id: 1, Username: "testuser"},
			send: make(chan *ServerMessage, 1),
		}

		cs.RegisterClient(client)
		assert.Len(t, cs.clients, 1, "expected 1 client after registration")
		assert.Contains(t, cs.clients, client, "expected client to be registered")

		// Check if the presence notification was queued
		select {
		case msg := <-client.send:
			assert.NotNil(t, msg, "expected message to be queued to client")
			assert.NotNil(t, msg.Notification, "expected notification message")
			assert.NotNil(t, msg.Notification.Presence, "expected presence message")
			assert.Equalf(t, room.externalId, msg.Notification.Presence.RoomId, "expected presence message for room %q", room.externalId)
			assert.True(t, msg.Notification.Presence.Present, "expected presence to be true")
		default:
			t.Error("expected presence notification to be queued to client")
		}
	})

	t.Run("register client with no subscriptions", func(t *testing.T) {
		db := &database.MockGoChatRepository{}
		defer db.AssertExpectations(t)

		db.On("ListSubscriptions", 1).Return([]database.Subscription{}, nil).Once()

		su := &stats.MockStatsUpdater{}
		su.On("Incr", "NumActiveClients").Once()
		defer su.AssertExpectations(t)

		cs := newTestChatServer(t, db, su)
		client := &Client{
			user: types.User{Id: 1, Username: "testuser"},
			send: make(chan *ServerMessage, 1),
		}

		cs.RegisterClient(client)
		assert.Len(t, cs.clients, 1, "expected 1 client after registration")
		assert.Contains(t, cs.clients, client, "expected client to be registered")

		select {
		case <-client.send:
			t.Error("expected no presence notification when no subscriptions")
		default:
		}
	})

	t.Run("register client with subscription to inactive room", func(t *testing.T) {
		db := &database.MockGoChatRepository{}
		defer db.AssertExpectations(t)

		db.On("ListSubscriptions", 1).Return([]database.Subscription{
			{Room: database.Room{ExternalId: "testroom"}},
		}, nil).Once()

		su := &stats.MockStatsUpdater{}
		su.On("Incr", "NumActiveClients").Once()
		defer su.AssertExpectations(t)

		cs := newTestChatServer(t, db, su)
		client := &Client{
			user: types.User{Id: 1, Username: "testuser"},
			send: make(chan *ServerMessage, 1),
		}

		cs.RegisterClient(client)
		assert.Len(t, cs.clients, 1, "expected 1 client after registration")
		assert.Contains(t, cs.clients, client, "expected client to be registered")

		select {
		case <-client.send:
			t.Error("expected no presence notification for room that is not active")
		default:
		}
	})

	t.Run("register client with db error listing subscriptions", func(t *testing.T) {
		db := &database.MockGoChatRepository{}
		defer db.AssertExpectations(t)

		db.On("ListSubscriptions", 1).Return([]database.Subscription{}, errors.New("db error")).Once()

		su := &stats.MockStatsUpdater{}
		su.On("Incr", "NumActiveClients").Once()
		defer su.AssertExpectations(t)

		cs := newTestChatServer(t, db, su)
		client := &Client{
			user: types.User{Id: 1, Username: "testuser"},
			send: make(chan *ServerMessage, 1),
		}

		cs.RegisterClient(client)
		assert.Len(t, cs.clients, 1, "expected 1 client after registration")
		assert.Contains(t, cs.clients, client, "expected client to be registered")

		select {
		case <-client.send:
			t.Error("expected no presence notification after db error")
		default:
		}
	})
}

func TestDeRegisterClient(t *testing.T) {
	su := &stats.MockStatsUpdater{}
	su.On("Incr", "NumActiveClients").Once()
	su.On("Decr", "NumActiveClients").Once()
	defer su.AssertExpectations(t)

	cs := newTestChatServer(t, &database.MockGoChatRepository{}, su)
	user := types.User{Id: 1, Username: "testuser"}

	client := &Client{
		user: user,
		send: make(chan *ServerMessage, 1),
	}

	cs.addClient(client)
	assert.Contains(t, cs.clients, client, "expected client to be registered")
	assert.Contains(t, cs.userMap, user.Id, "expected userMap to contain user ID")

	cs.DeRegisterClient(client)
	assert.NotContains(t, cs.clients, client, "expected client to be removed from clients map")
	assert.NotContains(t, cs.userMap, user.Id, "expected userMap to not contain user ID after removing client")
}

func TestUnloadRoom(t *testing.T) {
	tcases := []struct {
		name        string
		roomId      string
		deleted     bool
		expectedErr error
	}{
		{
			name:        "unload existing room",
			roomId:      "testroom",
			deleted:     false,
			expectedErr: nil,
		},
		{
			name:        "empty room id",
			deleted:     false,
			expectedErr: fmt.Errorf("roomId cannot be empty"),
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			cs := newTestChatServer(t, &database.MockGoChatRepository{}, &stats.MockStatsUpdater{})
			err := cs.UnloadRoom(context.Background(), tc.roomId, tc.deleted)
			if tc.expectedErr != nil {
				assert.Error(t, err, "expected error unloading room")
				assert.EqualError(t, err, tc.expectedErr.Error(), "expected error to match %v, got %v", tc.expectedErr, err)
				assert.Len(t, cs.unloadRoomChan, 0, "expected unloadRoomChan to have no messages")
			} else {
				assert.NoError(t, err, "expected no error unloading room")

				select {
				case msg := <-cs.unloadRoomChan:
					assert.NotNil(t, msg, "expected unload request to be sent")
					assert.Equal(t, msg.roomId, tc.roomId, "expected room id to match")
					assert.Equalf(t, msg.deleted, tc.deleted, "expected deleted to be %t", tc.deleted)
				default:
					t.Error("expected unload request to be sent, but none was received")
				}
			}
		})
	}

	t.Run("fails with context deadline exceeded", func(t *testing.T) {
		cs := newTestChatServer(t, &database.MockGoChatRepository{}, &stats.MockStatsUpdater{})
		cs.unloadRoomChan = make(chan unloadRoomRequest) // make unbuffered channel to simulate blocking
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
		defer cancel()

		// Wait for the context to expire before calling UnloadRoom
		<-ctx.Done()

		err := cs.UnloadRoom(ctx, "testroom", false)
		assert.Error(t, err, "expected error unloading room")
		assert.ErrorIsf(t, err, context.DeadlineExceeded, "expected context deadline exceeded, got %v", err)
		assert.Len(t, cs.unloadRoomChan, 0, "expected unloadRoomChan to have no messages")
	})
}

func TestChatServer_unloadAllRooms(t *testing.T) {
	numRooms := 3
	su := &stats.MockStatsUpdater{}
	su.On("Incr", "NumActiveRooms").Times(numRooms)
	su.On("Decr", "NumActiveRooms").Times(numRooms)
	defer su.AssertExpectations(t)

	cs := newTestChatServer(t, &database.MockGoChatRepository{}, su)

	// Create and add three test rooms
	rooms := make([]*Room, numRooms)
	for i := range numRooms {
		rooms[i] = &Room{
			externalId: "testroom" + strconv.Itoa(i+1),
			exit:       make(chan exitReq, 1),
			log:        cs.log,
		}
		cs.addRoom(rooms[i].externalId, rooms[i])
	}

	var wg sync.WaitGroup
	for _, room := range rooms {
		wg.Add(1)
		go func(r *Room) {
			defer wg.Done()
			select {
			case req := <-r.exit:
				assert.Falsef(t, req.deleted, "expected deleted to be false for room %q", r.externalId)
				req.done <- r.externalId
			case <-time.After(500 * time.Millisecond):
				t.Errorf("timeout waiting for exit request for room %q", r.externalId)
			}
		}(room)
	}

	cs.unloadAllRooms()
	wg.Wait()

	for _, room := range rooms {
		_, ok := cs.getRoom(room.externalId)
		assert.Falsef(t, ok, "expected room %q to be unloaded", room.externalId)
	}

	assert.Equal(t, 0, cs.numRooms, "expected numRooms to be 0 after unloading all rooms")
}

func TestChatServer_unloadAllRooms_Integration(t *testing.T) {
	numRooms := 3

	su := &stats.MockStatsUpdater{}
	su.On("Incr", "NumActiveRooms").Times(numRooms)
	su.On("Decr", "NumActiveRooms").Times(numRooms)
	defer su.AssertExpectations(t)

	cs := newTestChatServer(t, &database.MockGoChatRepository{}, su)

	// Start three test rooms
	rooms := make([]*Room, numRooms)
	for i := 1; i <= numRooms; i++ {
		rooms[i-1] = &Room{
			externalId: "testroom" + strconv.Itoa(i),
			exit:       make(chan exitReq, 1),
			log:        cs.log,
		}
	}

	for _, room := range rooms {
		cs.addRoom(room.externalId, room)
		go room.start()
	}

	cs.unloadAllRooms()

	for _, room := range rooms {
		_, ok := cs.roomsMap.Load(room.externalId)
		assert.False(t, ok, "expected room %q to be unloaded", room.externalId)
	}

	assert.Equal(t, 0, cs.numRooms, "expected numRoom to be 0 after unloading all rooms")
}

func TestChatServer_unloadRoom(t *testing.T) {
	tcases := []struct {
		name    string
		deleted bool
	}{
		{
			name:    "not deleted",
			deleted: false,
		},
		{
			name:    "deleted",
			deleted: true,
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			su := &stats.MockStatsUpdater{}
			su.On("Incr", "NumActiveRooms").Once()
			su.On("Decr", "NumActiveRooms").Once()
			defer su.AssertExpectations(t)

			cs := newTestChatServer(t, &database.MockGoChatRepository{}, su)
			room := &Room{
				externalId: "testroom",
				exit:       make(chan exitReq, 1),
				log:        cs.log,
			}

			// Add the room to the chat server
			cs.addRoom(room.externalId, room)

			// Prepare to receive the exitReq and close done to unblock unloadRoom
			done := make(chan struct{})
			go func(r *Room) {
				req := <-r.exit
				assert.Equalf(t, tc.deleted, req.deleted, "expected %t for deleted flag", tc.deleted)
				req.done <- r.externalId
				close(done)
			}(room)

			// Unload the room
			cs.unloadRoom(room.externalId, tc.deleted)

			// Wait for the goroutine to finish
			select {
			case <-done:
			case <-time.After(200 * time.Millisecond):
				t.Error("expected exit request to be sent to room and handled")
			}

			_, ok := cs.getRoom(room.externalId)
			assert.False(t, ok, "expected room %q to be unloaded", room.externalId)
		})
	}
}

func TestChatServer_handleJoinRoom(t *testing.T) {
	t.Run("join existing active room", func(t *testing.T) {
		su := &stats.MockStatsUpdater{}
		su.On("Incr", "NumActiveRooms").Once()
		defer su.AssertExpectations(t)

		cs := newTestChatServer(t, &database.MockGoChatRepository{}, su)
		room := &Room{
			externalId: "testroom",
			joinChan:   make(chan *ClientMessage, 1),
		}
		cs.addRoom(room.externalId, room)

		cs.handleJoinRoom(&ClientMessage{
			BaseMessage: BaseMessage{Id: 1, Timestamp: time.Now()},
			Join:        &Join{RoomId: "testroom"},
		})

		// Check if the room was loaded to the chat server
		_, ok := cs.getRoom(room.externalId)
		assert.True(t, ok, "expected room to be loaded")

		select {
		case <-room.joinChan:
			// ok, join message sent to room
		default:
			t.Error("expected join message to be sent to room")
		}
	})

	t.Run("join to active room fails when joinChan full", func(t *testing.T) {
		su := &stats.MockStatsUpdater{}
		su.On("Incr", "NumActiveRooms").Once()
		defer su.AssertExpectations(t)

		cs := newTestChatServer(t, &database.MockGoChatRepository{}, su)
		room := &Room{
			externalId: "fullroom",
			joinChan:   make(chan *ClientMessage, 1),
		}
		cs.addRoom("fullroom", room)

		// Fill the joinChan
		room.joinChan <- &ClientMessage{}

		client := &Client{send: make(chan *ServerMessage, 1)}
		joinMsg := &ClientMessage{
			BaseMessage: BaseMessage{Id: 1, Timestamp: Now()},
			Join:        &Join{RoomId: "fullroom"},
			client:      client,
		}

		cs.handleJoinRoom(joinMsg)

		select {
		case msg := <-client.send:
			assert.NotNil(t, msg, "expected a message to be sent to the client")
			assert.NotNil(t, msg.Response, "expected response message")
			assert.Equal(t, joinMsg.Id, msg.Id, "expected response ID to match join message ID")
			assert.Equal(t, 503, msg.Response.ResponseCode, "expected response code to be 503")
		default:
			t.Error("expected a message to be sent to the client, but none was sent")
		}
	})

	t.Run("successful join to inactive room with existing subscription", func(t *testing.T) {
		roomId := "testroom"
		db := &database.MockGoChatRepository{}
		dbRoom := database.Room{Id: 1, ExternalId: roomId, Subscriptions: []database.Subscription{{AccountId: 1}}}
		db.On("GetRoomByExternalId", roomId).Return(dbRoom, nil).Once()
		db.On("GetSubscribersByRoomId", dbRoom.Id).Return([]database.User{}, nil).Once()
		// These methods may be called in Room.handleJoin
		db.On("SubscriptionExists", 1, dbRoom.Id).Return(true).Maybe()
		db.On("GetRoomWithSubscribers", dbRoom.Id).Return(&dbRoom, nil).Maybe()
		defer db.AssertExpectations(t)

		su := &stats.MockStatsUpdater{}
		su.On("Incr", "NumActiveRooms").Once()
		su.On("Decr", "NumActiveRooms").Once()
		defer su.AssertExpectations(t)

		cs := newTestChatServer(t, db, su)
		client := &Client{
			user:     types.User{Id: 1},
			send:     make(chan *ServerMessage, 1),
			rooms:    make(map[string]*Room),
			log:      cs.log,
			exitRoom: make(chan string, 1),
		}
		joinMsg := &ClientMessage{
			BaseMessage: BaseMessage{Id: 1, Timestamp: time.Now()},
			Join:        &Join{RoomId: roomId},
			client:      client,
		}

		cs.handleJoinRoom(joinMsg)
		defer cs.unloadRoom(roomId, false) // handleJoinRoom calls Room.start which starts the rooms main goroutine

		// Check if the room was created and added to the chat server
		room, ok := cs.getRoom(joinMsg.Join.RoomId)
		assert.True(t, ok, "expected room to be loaded")
		assert.NotNil(t, room, "expected room to be non-nil")
		assert.Equal(t, room.externalId, roomId, "expected room externalId to match join request")
	})

	t.Run("join inactive room room not found", func(t *testing.T) {
		roomId := "notfound"
		db := &database.MockGoChatRepository{}
		db.On("GetRoomByExternalId", roomId).Return(database.Room{}, sql.ErrNoRows).Once()
		defer db.AssertExpectations(t)

		cs := newTestChatServer(t, db, &stats.MockStatsUpdater{})
		client := &Client{send: make(chan *ServerMessage, 1)}
		joinMsg := &ClientMessage{
			BaseMessage: BaseMessage{Id: 1, Timestamp: time.Now()},
			Join:        &Join{RoomId: roomId},
			client:      client,
		}

		cs.handleJoinRoom(joinMsg)

		_, ok := cs.getRoom(joinMsg.Join.RoomId)
		assert.False(t, ok, "expected room to not be loaded due to room not found error")

		select {
		case msg := <-client.send:
			assert.NotNil(t, msg, "expected message to be queued to client")
			assert.NotNil(t, msg.Response, "expected response message")
			assert.Equal(t, joinMsg.Id, msg.Id, "expected response ID to match join message ID")
			assert.Equal(t, 404, msg.Response.ResponseCode, "expected response code to be 404")
		default:
			t.Error("expected error message to be queued")
		}
	})

	t.Run("join inactive room db error getting room", func(t *testing.T) {
		roomId := "dberr"
		db := &database.MockGoChatRepository{}
		db.On("GetRoomByExternalId", roomId).Return(database.Room{}, errors.New("db error")).Once()
		defer db.AssertExpectations(t)

		cs := newTestChatServer(t, db, &stats.MockStatsUpdater{})
		client := &Client{send: make(chan *ServerMessage, 1)}
		joinMsg := &ClientMessage{
			BaseMessage: BaseMessage{Id: 4, Timestamp: time.Now()},
			Join:        &Join{RoomId: roomId},
			client:      client,
		}

		cs.handleJoinRoom(joinMsg)

		_, ok := cs.getRoom(joinMsg.Join.RoomId)
		assert.False(t, ok, "expected room to not be loaded due to GetRoomByExternalId error")

		select {
		case msg := <-client.send:
			assert.NotNil(t, msg, "expected message to be queued to client")
			assert.NotNil(t, msg.Response, "expected response message")
			assert.Equal(t, joinMsg.Id, msg.Id, "expected response ID to match join message ID")
			assert.Equal(t, 500, msg.Response.ResponseCode, "expected response code to be 500")
		default:
			t.Error("expected error message to be queued")
		}
	})

	t.Run("join inactive room db error getting subscriptions", func(t *testing.T) {
		roomId := "subserr"
		db := &database.MockGoChatRepository{}
		dbRoom := database.Room{Id: 1, ExternalId: roomId}
		db.On("GetRoomByExternalId", roomId).Return(dbRoom, nil).Once()
		db.On("GetSubscribersByRoomId", dbRoom.Id).Return([]database.User{}, errors.New("subscribers error")).Once()
		defer db.AssertExpectations(t)

		cs := newTestChatServer(t, db, &stats.MockStatsUpdater{})
		client := &Client{send: make(chan *ServerMessage, 1)}
		joinMsg := &ClientMessage{
			BaseMessage: BaseMessage{Id: 1, Timestamp: Now()},
			Join:        &Join{RoomId: roomId},
			client:      client,
		}

		cs.handleJoinRoom(joinMsg)

		_, ok := cs.getRoom(joinMsg.Join.RoomId)
		assert.False(t, ok, "expected room to not be loaded due to GetSubscribersByRoomId error")

		select {
		case msg := <-client.send:
			assert.NotNil(t, msg, "expected message to be queued to client")
			assert.NotNil(t, msg.Response, "expected response message")
			assert.Equal(t, joinMsg.Id, msg.Id, "expected response ID to match join message ID")
			assert.Equal(t, 500, msg.Response.ResponseCode, "expected response code to be 500")
		default:
			t.Error("expected error message to be queued")
		}
	})
}
