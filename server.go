package main

import (
	"log"
	"time"

	"github.com/teris-io/shortid"
)

type UserMessage struct {
	Id        int             `json:"id"`
	Type      UserMessageType `json:"type"`
	SeqId     int             `json:"seq_id,omitempty"`
	RoomId    int             `json:"room_id,"`
	Content   string          `json:"content,omitempty"`
	UserId    int             `json:"user_id,omitempty"`
	Username  string          `json:"username,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
	client    *Client         `json:"-"`
}

type UserMessageType string

const (
	UserMessageTypeJoin    UserMessageType = "join"
	UserMessageTypeLeave   UserMessageType = "leave"
	UserMessageTypePublish UserMessageType = "publish"
)

type SystemMessage struct {
	Id      int               `json:"id,omitempty"`
	Type    SystemMessageType `json:"type"`
	RoomId  int               `json:"room_id"`
	Data    map[string]any    `json:"data,omitempty"`
	SeqId   int               `json:"seq_id,omitempty"`
	Content string            `json:"content,omitempty"`
	UserId  int               `json:"user_id,omitempty"`
	// todo can username be removed?
	Username  string    `json:"username,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type SystemMessageType string

const (
	EventTypeMessagePublished SystemMessageType = "publish"
	EventTypeUserSubscribe    SystemMessageType = "subscribe"
	EventTypeUserUnSubscribe  SystemMessageType = "unsubscribe"

	EventTypeUserPresent SystemMessageType = "user_present"
	EventTypeUserAbsent  SystemMessageType = "user_absent"
	EventTypeRoomDeleted SystemMessageType = "room_deleted"
	EventTypeRoomJoined  SystemMessageType = "joined"
	EventTypeRoomLeft    SystemMessageType = "left"
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
	joinChan       chan *UserMessage
	registerChan   chan *Client
	deRegisterChan chan *Client
	subChan        chan subReq
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
		joinChan:       make(chan *UserMessage),
		clients:        make(map[*Client]struct{}),
		registerChan:   make(chan *Client),
		deRegisterChan: make(chan *Client),
		subChan:        make(chan subReq),
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
		case joinMsg := <-cs.joinChan:
			cs.log.Println("received join request")
			if room, ok := cs.rooms[joinMsg.RoomId]; ok {
				select {
				case room.joinChan <- joinMsg:
				default:
					cs.log.Printf("join channel full on room %d", room.Id)
				}
			} else {
				dbRoom, err := GetRoomByID(joinMsg.RoomId)
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
					joinChan:      make(chan *UserMessage, 256),
					leaveChan:     make(chan *UserMessage, 256),
					clientMsgChan: make(chan *UserMessage, 256),
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
			cs.log.Printf("registering connection from %q", client.user.Username)
			cs.clients[client] = struct{}{}
		case client := <-cs.deRegisterChan:
			cs.log.Printf("deregistering connection from %q", client.user.Username)
			if _, ok := cs.clients[client]; ok {
				delete(cs.clients, client)
				close(client.send)
			}
		case req := <-cs.subChan:
			switch req.subType {
			case subReqTypeSubscribe:
				// notify other users in the room
				if room, ok := cs.rooms[req.roomId]; ok {
					room.broadcast(&SystemMessage{
						Type:     EventTypeUserSubscribe,
						UserId:   req.user.Id,
						Username: req.user.Username,
					})
				}
			case subReqTypeUnsubscribe:
				cs.log.Printf("unsubscribing user %q from room %d", req.user.Username, req.roomId)
				if room, ok := cs.rooms[req.roomId]; ok {
					room.removeAllClientsForUser(req.user.Id)
					room.broadcast(&SystemMessage{
						Type:   EventTypeUserUnSubscribe,
						RoomId: room.Id,
						UserId: req.user.Id,
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
