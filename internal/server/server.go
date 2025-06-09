package server

import (
	"log"
	"sync"

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
	DelRoomChan    chan string
	broadcastChan  chan *ServerMessage
	rooms          map[string]*Room
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
			cs.log.Println("received join request")
			if room, ok := cs.rooms[joinMsg.Join.RoomId]; ok {
				select {
				case room.joinChan <- joinMsg:
				default:
					joinMsg.client.queueMessage(ErrServiceUnavailable(joinMsg.Id))
					cs.log.Printf("joinChan full for room %q ", room.externalId)
					continue
				}
			} else {
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
					exit:          make(chan exitReq),
					done:          make(chan struct{}),
				}

				cs.rooms[room.externalId] = room
				room.joinChan <- joinMsg

				go room.start()

			}
		case client := <-cs.RegisterChan:
			cs.log.Printf("adding connection from %q", client.user.Username)
			cs.addClient(client)
		case client := <-cs.deRegisterChan:
			cs.log.Printf("removing connection from %q", client.user.Username)
			cs.removeClient(client)
		case msg := <-cs.broadcastChan:
			cs.log.Printf("broadcasting message %v", msg)
			userClients := cs.userMap[msg.UserId]
			// if there are no clients for this user, skip broadcasting
			if userClients == nil {
				continue
			}
			for _, c := range userClients {
				c.queueMessage(msg)
			}
		case id := <-cs.DelRoomChan:
			r, ok := cs.rooms[id]
			if !ok {
				cs.log.Printf("room %q not active for deletion", id)
				continue
			}
			cs.unloadRoom(r.externalId)
			r.exit <- exitReq{deleted: true}
			<-r.done
			cs.log.Printf("deleted room %q", id)
		case <-cs.stop:
			cs.log.Println("shutting down rooms")
			for _, r := range cs.rooms {
				cs.log.Println("shutting down room", r.externalId)
				close(r.exit)

				<-r.done
			}

			close(cs.done)
			return
		}
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

func (cs *ChatServer) unloadRoom(roomId string) {
	if r, ok := cs.rooms[roomId]; ok {
		cs.log.Printf("removing room %q", r.externalId)
		delete(cs.rooms, roomId)
	}

	cs.log.Printf("current rooms: %v", cs.rooms)
}

func (cs *ChatServer) Shutdown() {
	cs.log.Println("received shutdown signal")
	for c := range cs.clients {
		close(c.stop)
	}

	close(cs.stop)

	<-cs.done
}
