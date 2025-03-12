package main

import (
	"encoding/json"
	"fmt"
	"log"
)

type MessageType int

const (
	MessageTypeJoin MessageType = iota
	MessageTypeLeave
	MessageTypePublish
)

func (mt MessageType) String() string {
	return [...]string{
		"join",
		"leave",
		"publish",
	}[mt]
}

type Message struct {
	Type    MessageType `json:"type"`
	RoomId  int         `json:"room_id"`
	Content string      `json:"content"`
	From    string      `json:"from"`
	client  *Client     `json:"-"`
}

type ChatServer struct {
	log            *log.Logger
	clients        map[*Client]struct{}
	joinChan       chan *Message
	registerChan   chan *Client
	deRegisterChan chan *Client
	broadcastChan  chan Message
	rooms          map[int]*Room
	stop           chan struct{}
	done           chan struct{}
}

func NewChatServer(logger *log.Logger) *ChatServer {
	return &ChatServer{
		log:            logger,
		joinChan:       make(chan *Message),
		clients:        make(map[*Client]struct{}),
		registerChan:   make(chan *Client),
		deRegisterChan: make(chan *Client),
		broadcastChan:  make(chan Message),
		rooms:          make(map[int]*Room),
		stop:           make(chan struct{}),
		done:           make(chan struct{}),
	}
}

func (cs *ChatServer) run() {
	for {
		select {
		case join := <-cs.joinChan:
			cs.log.Println("received join request")
			if room, ok := cs.rooms[join.RoomId]; ok {
				select {
				case room.joinChan <- join:
				default:
					cs.log.Printf("join channel full on room %d", room.Id)
				}
			} else {
				dbRoom, err := GetRoomById(join.RoomId)
				if err != nil {
					cs.log.Println("get room:", err)
					continue
				}

				room := &Room{
					Id:            dbRoom.Id,
					Name:          dbRoom.Name,
					Description:   dbRoom.Description,
					joinChan:      make(chan *Message, 256),
					leaveChan:     make(chan *Message, 256),
					clientMsgChan: make(chan *Message, 256),
					clients:       make(map[*Client]struct{}),
					log:           cs.log,
					exit:          make(chan struct{}),
					done:          make(chan struct{}),
				}

				cs.rooms[room.Id] = room
				room.joinChan <- join

				go room.start()

			}
		case client := <-cs.registerChan:
			cs.log.Printf("registering connection from %+v", client)
			cs.broadcast(Message{
				Type:    MessageTypeJoin,
				Content: fmt.Sprintf("%s has joined the chat.", client.user.Username),
			})
			cs.clients[client] = struct{}{}
		case client := <-cs.deRegisterChan:
			cs.log.Printf("deregistering connection from %+v", client)
			if _, ok := cs.clients[client]; ok {
				delete(cs.clients, client)
				cs.broadcast(Message{
					Type:    MessageTypeLeave,
					Content: fmt.Sprintf("%s has left the chat.", client.user.Username),
				})
				close(client.send)
			}
		case msg := <-cs.broadcastChan:
			cs.broadcast(msg)
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
