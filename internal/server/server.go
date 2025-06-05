package server

import (
	"log"
	"sync"

	"github.com/npezzotti/go-chatroom/internal/database"
)

type ChatServer struct {
	log            *log.Logger
	db             *database.DBConn
	clients        map[*Client]struct{}
	clientsLock    sync.Mutex
	joinChan       chan *ClientMessage
	RegisterChan   chan *Client
	deRegisterChan chan *Client
	RmRoomChan     chan string
	rooms          map[string]*Room
	stop           chan struct{}
	done           chan struct{}
}

func NewChatServer(logger *log.Logger, db *database.DBConn) (*ChatServer, error) {
	return &ChatServer{
		log:            logger,
		db:             db,
		joinChan:       make(chan *ClientMessage),
		clients:        make(map[*Client]struct{}),
		RegisterChan:   make(chan *Client),
		deRegisterChan: make(chan *Client),
		RmRoomChan:     make(chan string),
		stop:           make(chan struct{}),
		done:           make(chan struct{}),
		rooms:          make(map[string]*Room),
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
					cs.log.Printf("join channel full on room %q", room.externalId)
				}
			} else {
				dbRoom, err := cs.db.GetRoomByExternalID(joinMsg.Join.RoomId)
				if err != nil {
					joinMsg.client.queueMessage(ErrRoomNotFound(joinMsg.Id))
					continue
				}

				room := &Room{
					id:            dbRoom.Id,
					externalId:    dbRoom.ExternalId,
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
		case id := <-cs.RmRoomChan:
			r, ok := cs.rooms[id]
			if ok {
				cs.unloadRoom(r.externalId)
				r.exit <- exitReq{deleted: true}
				<-r.done
			}
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
}

func (cs *ChatServer) removeClient(c *Client) {
	cs.clientsLock.Lock()
	defer cs.clientsLock.Unlock()
	delete(cs.clients, c)
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
