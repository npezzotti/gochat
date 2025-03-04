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
	Content string      `json:"content"`
	From    string      `json:"from"`
}

type ChatServer struct {
	log            *log.Logger
	clients        map[*Client]struct{}
	registerChan   chan *Client
	deRegisterChan chan *Client
	broadcastChan  chan Message
}

func NewChatServer(logger *log.Logger) *ChatServer {
	return &ChatServer{
		log:            logger,
		clients:        make(map[*Client]struct{}),
		registerChan:   make(chan *Client),
		deRegisterChan: make(chan *Client),
		broadcastChan:  make(chan Message),
	}
}

func (cs *ChatServer) run() {
	for {
		select {
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
