package server

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/npezzotti/go-chatroom/internal/database"
	"github.com/npezzotti/go-chatroom/internal/types"
)

type GoChatServer interface {
	Run()
	Shutdown(ctx context.Context) error
	DeleteRoom(ctx context.Context, roomId string) error
	RegisterClient(c *Client)
	DeRegisterClient(c *Client)
	JoinRoom(joinMsg *ClientMessage) error
}

type ChatServer struct {
	log            *log.Logger
	db             database.GoChatRepository
	clients        map[*Client]struct{}
	clientsLock    sync.Mutex
	joinChan       chan *ClientMessage
	unloadRoomChan chan string
	delRoomChan    chan *deleteRoomRequest
	broadcastChan  chan *ServerMessage
	rooms          map[string]*Room
	roomsLock      sync.RWMutex
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
		unloadRoomChan: make(chan string),
		delRoomChan:    make(chan *deleteRoomRequest, 256),
		broadcastChan:  make(chan *ServerMessage, 256),
		stop:           make(chan struct{}),
		done:           make(chan struct{}),
		rooms:          make(map[string]*Room),
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
		case id := <-cs.unloadRoomChan:
			cs.unloadRoom(id, false)
		case req := <-cs.delRoomChan:
			cs.unloadRoom(req.roomId, true)
			close(req.done)
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
	if room, ok := cs.rooms[joinMsg.Join.RoomId]; ok {
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
	cs.roomsLock.Lock()
	defer cs.roomsLock.Unlock()
	cs.rooms[id] = r
}

// removeRoom removes a room from the server's list of active rooms by its ID.
// It is safe to call this method even if the room does not exist.
func (cs *ChatServer) removeRoom(id string) {
	cs.roomsLock.Lock()
	defer cs.roomsLock.Unlock()
	delete(cs.rooms, id)
}

// getRoom retrieves a room by its ID from the server's list of active rooms.
// It returns the room and a boolean indicating whether the room was found.
func (cs *ChatServer) getRoom(id string) (*Room, bool) {
	cs.roomsLock.RLock()
	defer cs.roomsLock.RUnlock()
	room, ok := cs.rooms[id]
	return room, ok
}

func (cs *ChatServer) unloadAllRooms() {
	cs.log.Println("shutting down all active rooms")
	// signal all rooms to exit
	roomDone := make(chan string)
	for _, r := range cs.rooms {
		cs.log.Println("shutting down room", r.externalId)
		r.exit <- exitReq{deleted: false, done: roomDone}
	}

	for range len(cs.rooms) {
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

func (cs *ChatServer) RegisterClient(client *Client) {
	cs.log.Printf("adding connection from %q", client.user.Username)
	cs.addClient(client)

	subs, err := cs.db.ListSubscriptions(client.user.Id)
	if err != nil {
		cs.log.Println("ListSubscriptions:", err)
	}
	// notify the client of any active rooms to which they are subscribed
	for _, sub := range subs {
		if room, ok := cs.rooms[sub.Room.ExternalId]; ok {
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
}

func (cs *ChatServer) DeRegisterClient(c *Client) {
	cs.log.Printf("removing connection from %q", c.user.Username)
	cs.removeClient(c)
}

type deleteRoomRequest struct {
	roomId string
	done   chan error
}

// DeleteRoom initiates unloading of a deleted room by its external ID and returns a channel
// that will be closed when the operation is complete.
func (cs *ChatServer) DeleteRoom(ctx context.Context, roomId string) error {
	done := make(chan error, 1)
	req := &deleteRoomRequest{
		roomId: roomId,
		done:   done,
	}
	cs.delRoomChan <- req
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (cs *ChatServer) JoinRoom(joinMsg *ClientMessage) error {
	select {
	case cs.joinChan <- joinMsg:
		return nil
	default:
		return errors.New("join channel is full")
	}
}
