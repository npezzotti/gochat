package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/teris-io/shortid"
)

type MessageType int

const (
	MessageTypeJoin MessageType = iota
	MessageTypeLeave
	MessageTypePublish
	MessageTypeRoomDeleted
	MessageTypePresence

	PresenceTypeOnline  = "online"
	PresenceTypeOffline = "offline"
)

func (mt MessageType) String() string {
	return [...]string{
		"join",
		"leave",
		"publish",
	}[mt]
}

type Message struct {
	Id        int         `json:"id"`
	Type      MessageType `json:"type,omitempty"`
	SeqId     int         `json:"seq_id,omitempty"`
	RoomId    int         `json:"room_id"`
	Content   string      `json:"content"`
	UserId    int         `json:"user_id,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	client    *Client     `json:"-"`
}

type ChatServer struct {
	log            *log.Logger
	clients        map[*Client]struct{}
	joinChan       chan *Message
	registerChan   chan *Client
	deRegisterChan chan *Client
	broadcastChan  chan Message
	rmRoomChan     chan int
	rooms          map[int]*Room
	stop           chan struct{}
	done           chan struct{}
	sid            *shortid.Shortid
}

func NewChatServer(logger *log.Logger) (*ChatServer, error) {
	sid, err := shortid.New(1, shortid.DefaultABC, 2342)
	if err != nil {
		return nil, err
	}

	return &ChatServer{
		log:            logger,
		joinChan:       make(chan *Message),
		clients:        make(map[*Client]struct{}),
		registerChan:   make(chan *Client),
		deRegisterChan: make(chan *Client),
		broadcastChan:  make(chan Message),
		rmRoomChan:     make(chan int),
		rooms:          make(map[int]*Room),
		stop:           make(chan struct{}),
		done:           make(chan struct{}),
		sid:            sid,
	}, nil
}

func (cs *ChatServer) run() {
	for {
		select {
		case join := <-cs.joinChan:
			cs.log.Println("received join request")
			if room, ok := cs.rooms[join.RoomId]; ok {
				select {
				case room.joinChan <- join.client:
				default:
					cs.log.Printf("join channel full on room %d", room.Id)
				}
			} else {
				dbRoom, err := GetRoomByID(join.RoomId)
				if err != nil {
					cs.log.Println("get room:", err)
					continue
				}

				room := &Room{
					Id:            dbRoom.Id,
					ExternalId:    dbRoom.ExternalId,
					Name:          dbRoom.Name,
					Description:   dbRoom.Description,
					cs:            cs,
					joinChan:      make(chan *Client, 256),
					leaveChan:     make(chan *Client, 256),
					clientMsgChan: make(chan *Message, 256),
					seq_id:        dbRoom.SeqId,
					clients:       make(map[*Client]struct{}),
					log:           cs.log,
					exit:          make(chan exitReq),
					done:          make(chan struct{}),
				}

				cs.rooms[room.Id] = room
				room.joinChan <- join.client

				go room.start()

			}
		case client := <-cs.registerChan:
			cs.log.Printf("registering connection from %q", client.user.Username)
			cs.clients[client] = struct{}{}
		case client := <-cs.deRegisterChan:
			cs.log.Printf("deregistering connection from %q", client.user.Username)
			if _, ok := cs.clients[client]; ok {
				delete(cs.clients, client)
				close(client.send)
			}
		case msg := <-cs.broadcastChan:
			cs.broadcast(msg)
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
				cs.log.Println("shutting down room", r.Name)
				close(r.exit)

				<-r.done
			}

			close(cs.done)
			return
		}
	}
}

func (cs *ChatServer) unloadRoom(roomId int) {
	if r, ok := cs.rooms[roomId]; ok {
		cs.log.Printf("removing room %q", r.Name)
		delete(cs.rooms, roomId)
	}

	cs.log.Printf("current rooms: %v", cs.rooms)
}

func (cs *ChatServer) broadcast(msg Message) {
	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		cs.log.Println(":", err)
		return
	}

	for client := range cs.clients {
		select {
		case client.send <- jsonMsg:
			cs.log.Printf("broadcasting message: %q", jsonMsg)
		default:
			cs.log.Println("default")
		}
	}
}

func (cs *ChatServer) shutdown() {
	cs.log.Println("received shutdown signal")
	for c := range cs.clients {
		close(c.stop)
	}

	close(cs.stop)

	<-cs.done
}
