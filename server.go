package main

import (
	"fmt"
	"log"
	"sync"
)

type subReq struct {
	subType subReqType
	user    User
	roomId  int
}

type subReqType string

const (
	subReqTypeSubscribe   subReqType = "subscribe"
	subReqTypeUnsubscribe subReqType = "unsubscribe"
)

type ChatServer struct {
	log            *log.Logger
	clients        map[*Client]struct{}
	clientsLock    sync.Mutex
	joinChan       chan *ClientMessage
	registerChan   chan *Client
	deRegisterChan chan *Client
	subChan        chan subReq
	rmRoomChan     chan int
	rooms          map[int]*Room
	stop           chan struct{}
	done           chan struct{}
}

func NewChatServer(logger *log.Logger) (*ChatServer, error) {
	return &ChatServer{
		log:            logger,
		joinChan:       make(chan *ClientMessage),
		clients:        make(map[*Client]struct{}),
		registerChan:   make(chan *Client),
		deRegisterChan: make(chan *Client),
		subChan:        make(chan subReq),
		rmRoomChan:     make(chan int),
		rooms:          make(map[int]*Room),
		stop:           make(chan struct{}),
		done:           make(chan struct{}),
	}, nil
}

func (cs *ChatServer) run() {
	for {
		select {
		case joinMsg := <-cs.joinChan:
			cs.log.Println("received join request")
			if room, ok := cs.rooms[joinMsg.Join.RoomId]; ok {
				select {
				case room.joinChan <- joinMsg:
				default:
					cs.log.Printf("join channel full on room %d", room.Id)
				}
			} else {
				fmt.Println(joinMsg.Join.RoomId)
				dbRoom, err := DB.GetRoomByID(joinMsg.Join.RoomId)
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
					log:           cs.log,
					exit:          make(chan exitReq),
					done:          make(chan struct{}),
				}

				cs.rooms[room.Id] = room
				room.joinChan <- joinMsg

				go room.start()

			}
		case client := <-cs.registerChan:
			cs.log.Printf("adding connection from %q", client.user.Username)
			cs.addClient(client)
		case client := <-cs.deRegisterChan:
			cs.log.Printf("removing connection from %q", client.user.Username)
			cs.removeClient(client)
		case req := <-cs.subChan:
			switch req.subType {
			case subReqTypeSubscribe:
				// notify other users in the room
				if room, ok := cs.rooms[req.roomId]; ok {
					room.broadcast(&ServerMessage{
						Notification: &Notification{
							SubscriptionChange: &SubscriptionChange{
								RoomId:     room.Id,
								Subscribed: true,
								User: User{
									Id:       req.user.Id,
									Username: req.user.Username,
								},
							},
						},
					})
				}
			case subReqTypeUnsubscribe:
				cs.log.Printf("unsubscribing user %q from room %d", req.user.Username, req.roomId)
				if room, ok := cs.rooms[req.roomId]; ok {
					room.removeAllClientsForUser(req.user.Id)
					room.broadcast(&ServerMessage{
						Notification: &Notification{
							SubscriptionChange: &SubscriptionChange{
								RoomId:     room.Id,
								Subscribed: false,
								User: User{
									Id:       req.user.Id,
									Username: req.user.Username,
								},
							},
						},
					})
				}
			}
		case id := <-cs.rmRoomChan:
			r, ok := cs.rooms[id]
			if ok {
				cs.unloadRoom(r.Id)
				r.exit <- exitReq{deleted: true}
				<-r.done
			}
		case <-cs.stop:
			cs.log.Println("shutting down rooms")
			for _, r := range cs.rooms {
				cs.log.Println("shutting down room", r.ExternalId)
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
		cs.log.Printf("removing room %q", r.ExternalId)
		delete(cs.rooms, roomId)
	}

	cs.log.Printf("current rooms: %v", cs.rooms)
}

func (cs *ChatServer) shutdown() {
	cs.log.Println("received shutdown signal")
	for c := range cs.clients {
		close(c.stop)
	}

	close(cs.stop)

	<-cs.done
}
