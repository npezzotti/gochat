package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/npezzotti/go-chatroom/internal/database"
	"github.com/npezzotti/go-chatroom/internal/stats"
	"github.com/npezzotti/go-chatroom/internal/types"
)

type ChatServer struct {
	log            *log.Logger
	db             database.GoChatRepository
	clients        map[*Client]struct{}
	userMap        map[int][]*Client
	clientsMu      sync.RWMutex // mutex to protect access to clients and userMap
	joinChan       chan *ClientMessage
	unloadRoomChan chan unloadRoomRequest
	broadcastChan  chan *ServerMessage
	numRooms       int
	roomsMap       sync.Map
	stop           chan stopReq
	stats          stats.StatsProvider
}

func NewChatServer(logger *log.Logger, db database.GoChatRepository, statsUpdater stats.StatsProvider) (*ChatServer, error) {
	cs := &ChatServer{
		log:            logger,
		db:             db,
		clients:        make(map[*Client]struct{}),
		userMap:        make(map[int][]*Client),
		joinChan:       make(chan *ClientMessage, 256),
		unloadRoomChan: make(chan unloadRoomRequest, 64),
		broadcastChan:  make(chan *ServerMessage, 256),
		stop:           make(chan stopReq),
		stats:          statsUpdater,
	}

	cs.stats.RegisterMetric("NumActiveRooms")
	cs.stats.RegisterMetric("NumActiveClients")
	cs.stats.RegisterMetric("TotalIncomingMessages")
	cs.stats.RegisterMetric("TotalOutgoingMessages")

	return cs, nil
}

func (cs *ChatServer) Run() {
	for {
		select {
		case joinMsg := <-cs.joinChan:
			cs.handleJoinRoom(joinMsg)
		case msg := <-cs.broadcastChan:
			cs.handleBroadcast(msg)
		case req := <-cs.unloadRoomChan:
			cs.unloadRoom(req.roomId, req.deleted)
		case req := <-cs.stop:
			cs.unloadAllRooms()
			close(req.done)
			return
		}
	}
}

// handleJoinRoom processes a join request from a client.
// It checks if the room is already loaded and forwards the join request to the room.
// If the room is not loaded, it retrieves the room from the database,
// creates a new Room instance, and starts the room before forwarding the join request.
func (cs *ChatServer) handleJoinRoom(joinMsg *ClientMessage) {
	// check if the room is already loaded
	if room, ok := cs.getRoom(joinMsg.Join.RoomId); ok {
		select {
		case room.joinChan <- joinMsg:
		default:
			cs.log.Printf("joinChan full for room %q ", room.externalId)
			joinMsg.client.queueMessage(ErrServiceUnavailable(joinMsg.Id))
			return
		}
	} else {
		// room not loaded, load it
		dbRoom, err := cs.db.GetRoomByExternalId(joinMsg.Join.RoomId)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				joinMsg.client.queueMessage(ErrRoomNotFound(joinMsg.Id))
				return
			} else {
				joinMsg.client.queueMessage(ErrInternalError(joinMsg.Id))
				cs.log.Println("GetRoomByExternalId:", err)
				return
			}
		}

		dbSubs, err := cs.db.GetSubscribersByRoomId(dbRoom.Id)
		if err != nil {
			joinMsg.client.queueMessage(ErrInternalError(joinMsg.Id))
			cs.log.Println("GetSubscriptionsByRoomId:", err)
			return
		}

		var subs []types.User
		for _, dbSub := range dbSubs {
			subs = append(subs, types.User{
				Id:       dbSub.Id,
				Username: dbSub.Username,
			})
		}

		room := &Room{
			id:            dbRoom.Id,
			externalId:    dbRoom.ExternalId,
			subscribers:   subs,
			cs:            cs,
			db:            cs.db,
			joinChan:      make(chan *ClientMessage, 256),
			leaveChan:     make(chan *ClientMessage, 256),
			clientMsgChan: make(chan *ClientMessage, 256),
			seq_id:        dbRoom.SeqId,
			clients:       make(map[*Client]struct{}),
			userMap:       make(map[int]map[*Client]struct{}),
			log:           cs.log,
			killTimer:     time.NewTimer(time.Second * 10),
			exit:          make(chan exitReq, 1),
		}

		cs.addRoom(room.externalId, room)
		// forward join request to the room
		room.joinChan <- joinMsg
		go room.start()
	}
}

// addRoom adds a room to the server's list of active rooms.
func (cs *ChatServer) addRoom(id string, r *Room) {
	cs.roomsMap.Store(id, r)
	cs.numRooms++

	cs.stats.Incr("NumActiveRooms")
}

// removeRoom removes a room from the server's list of active rooms by its ID.
func (cs *ChatServer) removeRoom(id string) {
	_, loaded := cs.roomsMap.LoadAndDelete(id)
	if loaded {
		cs.numRooms--
	}

	cs.stats.Decr("NumActiveRooms")
}

// getRoom retrieves a room by its ID from the server's list of active rooms.
// It returns the room and a boolean indicating whether the room was found.
func (cs *ChatServer) getRoom(id string) (*Room, bool) {
	r, ok := cs.roomsMap.Load(id)
	if !ok {
		return nil, false
	}

	return r.(*Room), ok
}

// unloadAllRooms unloads all active rooms.
// It signals each room to exit and waits for all rooms to complete their shutdown.
func (cs *ChatServer) unloadAllRooms() {
	// signal all rooms to exit
	roomDone := make(chan string)
	cs.roomsMap.Range(func(key, value interface{}) bool {
		room := value.(*Room)
		room.exit <- exitReq{deleted: false, done: roomDone}
		return true
	})

	for cs.numRooms > 0 {
		id := <-roomDone
		cs.removeRoom(id)
	}
}

// handleBroadcast processes a broadcast message.
// It queues a message to all clients associated with the user ID in the message,
// except for any client specified in SkipClient.
func (cs *ChatServer) handleBroadcast(msg *ServerMessage) {
	userClients := cs.getClients(msg.UserId)
	// if there are no clients for this user, skip broadcasting
	if userClients == nil {
		return
	}

	for _, c := range userClients {
		if msg.SkipClient != nil && c == msg.SkipClient {
			continue
		}
		c.queueMessage(msg)
	}
}

// addClient adds a new client to the server's list of active clients.
// It also adds the client to the userMap, which maps user IDs to their associated clients
func (cs *ChatServer) addClient(c *Client) {
	cs.clientsMu.Lock()
	defer cs.clientsMu.Unlock()

	cs.clients[c] = struct{}{}
	cs.userMap[c.user.Id] = append(cs.userMap[c.user.Id], c)

	cs.stats.Incr("NumActiveClients")
}

// removeClient removes a client from the server's list of active clients.
// It also removes the client from the userMap if it exists.
func (cs *ChatServer) removeClient(c *Client) {
	cs.clientsMu.Lock()
	defer cs.clientsMu.Unlock()

	delete(cs.clients, c)
	if userClients, ok := cs.userMap[c.user.Id]; ok {
		for i, client := range userClients {
			if client == c {
				cs.userMap[c.user.Id] = append(userClients[:i], userClients[i+1:]...)
				break
			}
		}
		if len(cs.userMap[c.user.Id]) == 0 {
			delete(cs.userMap, c.user.Id)
		}
	}
	cs.stats.Decr("NumActiveClients")
}

// getClients retrieves all clients associated with a specific user ID.
func (cs *ChatServer) getClients(userId int) []*Client {
	cs.clientsMu.RLock()
	defer cs.clientsMu.RUnlock()

	clients := cs.userMap[userId]
	if clients == nil {
		return nil
	}

	result := make([]*Client, len(clients))
	copy(result, clients)
	return result
}

// unloadRoom unloads a room by its ID.
// It signals the room to exit and waits for the exit to complete.
// If the room is deleted, it will remove it from the server's list of active rooms.
func (cs *ChatServer) unloadRoom(id string, deleted bool) {
	room, ok := cs.getRoom(id)
	if !ok {
		cs.log.Printf("attempted to unload non-existent room: %s", id)
		return
	}

	done := make(chan string, 1)
	select {
	case room.exit <- exitReq{deleted: deleted, done: done}:
		// Wait for the room to signal it has exited, but don't block forever
		select {
		case <-done:
			cs.removeRoom(id)
		case <-time.After(5 * time.Second):
			cs.log.Printf("timeout waiting for room %s to unload", id)
		}
	default:
		cs.log.Printf("room exit channel full for room %s", id)
	}
}

type stopReq struct {
	done chan struct{}
}

// Shutdown gracefully stops the chat server.
// It signals the server to stop processing messages and waits for all active rooms to be unloaded.
func (cs *ChatServer) Shutdown(ctx context.Context) error {
	done := make(chan struct{})
	cs.stop <- stopReq{done: done}

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// RegisterClient adds a new client to the server's list of active clients.
func (cs *ChatServer) RegisterClient(client *Client) {
	cs.addClient(client)

	subs, err := cs.db.ListSubscriptions(client.user.Id)
	if err != nil {
		cs.log.Println("ListSubscriptions:", err)
		return
	}

	// notify the client of any active rooms to which they are subscribed
	for _, sub := range subs {
		room, ok := cs.getRoom(sub.Room.ExternalId)
		if !ok {
			// room not loaded, skip notification
			continue
		}

		client.queueMessage(&ServerMessage{
			BaseMessage: BaseMessage{
				Timestamp: Now(),
			},
			Notification: &Notification{
				Presence: &Presence{
					Present: true,
					RoomId:  room.externalId,
				},
			},
		})
	}
}

// DeRegisterClient removes a client from the server's list of active clients.
func (cs *ChatServer) DeRegisterClient(c *Client) {
	cs.removeClient(c)
}

// unloadRoomRequest represents a request to unload a room by its external ID.
type unloadRoomRequest struct {
	roomId  string
	deleted bool
}

// UnloadRoom signals the chat server to unload a room by its external ID.
func (cs *ChatServer) UnloadRoom(ctx context.Context, roomId string, deleted bool) error {
	if roomId == "" {
		return fmt.Errorf("roomId cannot be empty")
	}

	// Attempt to send the unload request to the unloadRoomChan.
	// If the channel is full, return an error.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case cs.unloadRoomChan <- unloadRoomRequest{
		roomId:  roomId,
		deleted: deleted,
	}:
		return nil // unload request accepted
	default:
		return fmt.Errorf("unload room channel is full, unable to unload room %s", roomId)
	}
}
