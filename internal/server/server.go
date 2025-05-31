package server

import (
	"fmt"
	"log"
	"sync"

	"github.com/npezzotti/go-chatroom/internal/database"
	"github.com/npezzotti/go-chatroom/internal/types"
)

type SubReq struct {
	SubType subReqType
	User    types.User
	RoomId  int
}

type subReqType string

const (
	SubReqTypeSubscribe   subReqType = "subscribe"
	SubReqTypeUnsubscribe subReqType = "unsubscribe"
)

type ChatServer struct {
	Log            *log.Logger
	clients        map[*Client]struct{}
	clientsLock    sync.Mutex
	joinChan       chan *ClientMessage
	RegisterChan   chan *Client
	deRegisterChan chan *Client
	SubChan        chan SubReq
	RmRoomChan     chan int
	rooms          map[int]*Room
	stop           chan struct{}
	done           chan struct{}
}

func NewChatServer(logger *log.Logger) (*ChatServer, error) {
	return &ChatServer{
		Log:            logger,
		joinChan:       make(chan *ClientMessage),
		clients:        make(map[*Client]struct{}),
		RegisterChan:   make(chan *Client),
		deRegisterChan: make(chan *Client),
		SubChan:        make(chan SubReq),
		RmRoomChan:     make(chan int),
		rooms:          make(map[int]*Room),
		stop:           make(chan struct{}),
		done:           make(chan struct{}),
	}, nil
}

func (cs *ChatServer) Run() {
	for {
		select {
		case joinMsg := <-cs.joinChan:
			cs.Log.Println("received join request")
			if room, ok := cs.rooms[joinMsg.Join.RoomId]; ok {
				select {
				case room.joinChan <- joinMsg:
				default:
					cs.Log.Printf("join channel full on room %d", room.Id)
				}
			} else {
				fmt.Println(joinMsg.Join.RoomId)
				dbRoom, err := database.DB.GetRoomByID(joinMsg.Join.RoomId)
				if err != nil {
					joinMsg.client.queueMessage(ErrRoomNotFound(joinMsg.Id))
					continue
				}

				room := &Room{
					Id:            dbRoom.Id,
					ExternalId:    dbRoom.ExternalId,
					Name:          dbRoom.Name,
					Description:   dbRoom.Description,
					cs:            cs,
					joinChan:      make(chan *ClientMessage, 256),
					leaveChan:     make(chan *ClientMessage, 256),
					clientMsgChan: make(chan *ClientMessage, 256),
					seq_id:        dbRoom.SeqId,
					clients:       make(map[*Client]struct{}),
					userMap:       make(map[int]map[*Client]struct{}),
					log:           cs.Log,
					exit:          make(chan exitReq),
					done:          make(chan struct{}),
				}

				cs.rooms[room.Id] = room
				room.joinChan <- joinMsg

				go room.start()

			}
		case client := <-cs.RegisterChan:
			cs.Log.Printf("adding connection from %q", client.user.Username)
			cs.addClient(client)
		case client := <-cs.deRegisterChan:
			cs.Log.Printf("removing connection from %q", client.user.Username)
			cs.removeClient(client)
		case req := <-cs.SubChan:
			switch req.SubType {
			case SubReqTypeSubscribe:
				// notify other users in the room
				if room, ok := cs.rooms[req.RoomId]; ok {
					room.broadcast(&ServerMessage{
						Notification: &Notification{
							SubscriptionChange: &SubscriptionChange{
								RoomId:     room.Id,
								Subscribed: true,
								User: types.User{
									Id:       req.User.Id,
									Username: req.User.Username,
								},
							},
						},
					})
				}
			case SubReqTypeUnsubscribe:
				cs.Log.Printf("unsubscribing user %q from room %d", req.User.Username, req.RoomId)
				if room, ok := cs.rooms[req.RoomId]; ok {
					room.removeAllClientsForUser(req.User.Id)
					room.broadcast(&ServerMessage{
						Notification: &Notification{
							SubscriptionChange: &SubscriptionChange{
								RoomId:     room.Id,
								Subscribed: false,
								User: types.User{
									Id:       req.User.Id,
									Username: req.User.Username,
								},
							},
						},
					})
				}
			}
		case id := <-cs.RmRoomChan:
			r, ok := cs.rooms[id]
			if ok {
				cs.unloadRoom(r.Id)
				r.exit <- exitReq{deleted: true}
				<-r.done
			}
		case <-cs.stop:
			cs.Log.Println("shutting down rooms")
			for _, r := range cs.rooms {
				cs.Log.Println("shutting down room", r.ExternalId)
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

func (cs *ChatServer) unloadRoom(roomId int) {
	if r, ok := cs.rooms[roomId]; ok {
		cs.Log.Printf("removing room %q", r.ExternalId)
		delete(cs.rooms, roomId)
	}

	cs.Log.Printf("current rooms: %v", cs.rooms)
}

func (cs *ChatServer) Shutdown() {
	cs.Log.Println("received shutdown signal")
	for c := range cs.clients {
		close(c.stop)
	}

	close(cs.stop)

	<-cs.done
}
