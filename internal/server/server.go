package server

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/npezzotti/go-chatroom/internal/database"
	"github.com/npezzotti/go-chatroom/internal/types"
)

type ChatServer struct {
	log            *log.Logger
	db             *database.DBConn
	clients        map[*Client]struct{}
	clientsLock    sync.Mutex
	joinChan       chan *ClientMessage
	RegisterChan   chan *Client
	deRegisterChan chan *Client
	unloadRoomChan chan string
	DelRoomChan    chan string
	broadcastChan  chan *ServerMessage
	rooms          map[string]*Room
	roomsLock      sync.Mutex
	stop           chan struct{}
	done           chan struct{}
	userMap        map[int][]*Client
}

func NewChatServer(logger *log.Logger, db *database.DBConn) (*ChatServer, error) {
	return &ChatServer{
		log:            logger,
		db:             db,
		joinChan:       make(chan *ClientMessage, 256),
		clients:        make(map[*Client]struct{}),
		RegisterChan:   make(chan *Client),
		deRegisterChan: make(chan *Client),
		unloadRoomChan: make(chan string),
		DelRoomChan:    make(chan string),
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
			// check if the room is already loaded
			if room, ok := cs.rooms[joinMsg.Join.RoomId]; ok {
				select {
				case room.joinChan <- joinMsg:
				default:
					cs.log.Printf("joinChan full for room %q ", room.externalId)
					joinMsg.client.queueMessage(ErrServiceUnavailable(joinMsg.Id))
					continue
				}
			} else {
				// room not loaded, load it
				dbRoom, err := cs.db.GetRoomByExternalID(joinMsg.Join.RoomId)
				if err != nil {
					joinMsg.client.queueMessage(ErrRoomNotFound(joinMsg.Id))
					continue
				}

				dbSubs, err := cs.db.GetSubscribersForRoom(dbRoom.Id)
				if err != nil {
					joinMsg.client.queueMessage(ErrInternalError(joinMsg.Id))
					cs.log.Println("GetSubscriptionsByRoomId:", err)
					continue
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

				cs.rooms[room.externalId] = room
				// forward join request to the room
				room.joinChan <- joinMsg
				go room.start()
			}
		case client := <-cs.RegisterChan:
			cs.log.Printf("adding connection from %q", client.user.Username)
			cs.addClient(client)

			subs, err := cs.db.ListSubscriptions(client.user.Id)
			if err != nil {
				cs.log.Println("ListSubscriptions:", err)
				continue
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
		case client := <-cs.deRegisterChan:
			cs.log.Printf("removing connection from %q", client.user.Username)
			cs.removeClient(client)
		case msg := <-cs.broadcastChan:
			cs.log.Printf("broadcasting message %v", msg)
			cs.handleBroadcast(msg)
		case id := <-cs.unloadRoomChan:
			cs.handleUnloadRoom(id, false)
		case id := <-cs.DelRoomChan:
			cs.handleUnloadRoom(id, true)
			cs.log.Printf("deleted room %q", id)
		case <-cs.stop:
			cs.log.Println("shutting down rooms")
			cs.unloadAllRooms()
			close(cs.done)
			return
		}
	}
}

func (cs *ChatServer) unloadAllRooms() {
	for _, r := range cs.rooms {
		cs.log.Println("shutting down room", r.externalId)
		exit := exitReq{deleted: false, done: make(chan bool)}
		r.exit <- exit
		<-exit.done
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

func (cs *ChatServer) handleUnloadRoom(id string, deleted bool) {
	cs.roomsLock.Lock()
	room, ok := cs.rooms[id]
	cs.roomsLock.Unlock()
	if !ok {
		return
	}

	// Signal the room to exit and wait for cleanup
	exit := exitReq{deleted: deleted, done: make(chan bool)}
	room.exit <- exit
	<-exit.done

	cs.roomsLock.Lock()
	delete(cs.rooms, id)
	cs.roomsLock.Unlock()
}

func (cs *ChatServer) Shutdown(ctx context.Context) error {
	for c := range cs.clients {
		close(c.stop)
	}

	close(cs.stop)

	select {
	case <-cs.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
