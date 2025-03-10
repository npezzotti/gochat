package main

import (
	"encoding/json"
	"fmt"
	"log"
)

type Room struct {
	Id            int
	Name          string
	Description   string
	joinChan      chan *Message
	leaveChan     chan *Message
	clientMsgChan chan *Message
	clients       map[*Client]struct{}
	exit          chan struct{}
	log           *log.Logger
}

func (r *Room) start() {
	r.log.Printf("starting room %q", r.Name)
	for {
		select {
		case msg := <-r.joinChan:
			r.log.Println("join msg:", msg)
			r.clients[msg.client] = struct{}{}
			msg.client.rooms[r.Id] = r
		case msg := <-r.leaveChan:
			r.log.Println("leave msg:", msg)
			delete(r.clients, msg.client)
		case msg := <-r.clientMsgChan:
			jsonMsg, err := json.Marshal(msg)
			if err != nil {
				r.log.Println(":", err)
				return
			}

			fmt.Printf("received message to room %d: %s", r.Id, string(jsonMsg))

			for client := range r.clients {
				select {
				case client.send <- jsonMsg:
					r.log.Printf("broadcasting message: %q", jsonMsg)
				default:
					r.log.Println("default")
				}
			}
		case <-r.exit:
			r.log.Println("exiting")
		}
	}
}
