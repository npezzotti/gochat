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
	"github.com/npezzotti/go-chatroom/internal/types"
)

type ChatServer struct {
	log            *log.Logger
	db             database.GoChatRepository
	clients        map[*Client]struct{}
	clientsLock    sync.Mutex
	joinChan       chan *ClientMessage
	unloadRoomChan chan unloadRoomRequest
	broadcastChan  chan *ServerMessage
	numRooms       int
	roomsMap       sync.Map
	stop           chan struct{}
	done           chan struct{}
	userMap        map[int][]*Client
}

func NewChatServer(logger *log.Logger, db database.GoChatRepository) (*ChatServer, error) {
	return &ChatServer{
		log:            logger,
		db:             db,
		joinChan:       make(chan *ClientMessage, 256),
		clients:        make(map[*Client]struct{}),
		unloadRoomChan: make(chan unloadRoomRequest, 64),
		broadcastChan:  make(chan *ServerMessage, 256),
		stop:           make(chan struct{}),
		done:           make(chan struct{}),
		userMap:        make(map[int][]*Client),
	}, nil
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
		case <-cs.stop:
			cs.unloadAllRooms()
			cs.done <- struct{}{}
			return
		}
	}
}

// handleJoinRoom processes a join request from a client.
// It checks if the room is already loaded and, if so, forwards the join request to the room.
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
			exit:          make(chan exitReq),
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
}

// removeRoom removes a room from the server's list of active rooms by its ID.
// It is safe to call this method even if the room does not exist.
func (cs *ChatServer) removeRoom(id string) {
	_, loaded := cs.roomsMap.LoadAndDelete(id)
	if loaded {
		cs.numRooms--
	}
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

func (cs *ChatServer) unloadAllRooms() {
	cs.log.Println("shutting down all active rooms")
	// signal all rooms to exit
	roomDone := make(chan string)
	cs.roomsMap.Range(func(key, value interface{}) bool {
		room := value.(*Room)
		cs.log.Println("shutting down room", room.externalId)
		room.exit <- exitReq{deleted: false, done: roomDone}
		return true
	})

	for cs.numRooms > 0 {
		id := <-roomDone
		cs.log.Println("room shutdown complete", id)
		cs.removeRoom(id)
	}
}

func (cs *ChatServer) handleBroadcast(msg *ServerMessage) {
	userClients := cs.userMap[msg.UserId]
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

func (cs *ChatServer) addClient(c *Client) {
	cs.clientsLock.Lock()
	defer cs.clientsLock.Unlock()

	cs.clients[c] = struct{}{}
	cs.userMap[c.user.Id] = append(cs.userMap[c.user.Id], c)
}

func (cs *ChatServer) removeClient(c *Client) {
	cs.clientsLock.Lock()
	defer cs.clientsLock.Unlock()

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
}

// unloadRoom unloads a room by its ID.
// It signals the room to exit and waits for the exit to complete.
// If the room is deleted, it will remove it from the server's list of active rooms.
func (cs *ChatServer) unloadRoom(id string, deleted bool) {
	if room, ok := cs.getRoom(id); ok {
		// signal the room to exit
		done := make(chan string)
		room.exit <- exitReq{deleted: deleted, done: done}
		<-done
	}

	cs.removeRoom(id)
}

func (cs *ChatServer) Shutdown(ctx context.Context) error {
	cs.log.Println("shutting down chat server...")
	cs.stop <- struct{}{}

	select {
	case <-cs.done:
		cs.log.Println("chat server shutdown complete")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// RegisterClient adds a new client to the server's list of active clients.
func (cs *ChatServer) RegisterClient(client *Client) {
	cs.log.Printf("adding connection from %q", client.user.Username)
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
