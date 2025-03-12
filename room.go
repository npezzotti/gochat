package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

var idleRoomTimeout = time.Second * 5

type Room struct {
	Id            int
	Name          string
	Description   string
	cs            *ChatServer
	joinChan      chan *Message
	leaveChan     chan *Message
	clientMsgChan chan *Message
	clients       map[*Client]struct{}
	exit          chan struct{}
	done          chan struct{}
	log           *log.Logger
	killTimer     *time.Timer
}

func (r *Room) start() {
	defer func() {
		r.log.Println("room exiting")
	}()

	r.log.Printf("starting room %q", r.Name)

	r.killTimer = time.NewTimer(time.Second * 10)
	r.killTimer.Stop()

	for {
		select {
		case msg := <-r.joinChan:
			r.log.Println("join msg:", msg)
			r.killTimer.Stop()
			r.clients[msg.client] = struct{}{}
			msg.client.addRoom(r)
		case msg := <-r.leaveChan:
			r.log.Println("leave msg:", msg)
			delete(r.clients, msg.client)
			msg.client.delRoom(r.Id)

			if len(r.clients) == 0 {
				r.killTimer.Reset(idleRoomTimeout)
			}
		case msg := <-r.clientMsgChan:
			r.broadcast(msg)
		case <-r.killTimer.C:
			r.log.Printf("room %q timed out", r.Name)
			r.cs.unloadRoom(r.Id)
		case <-r.exit:
			for c := range r.clients {
				c.delRoom(r.Id)
			}

			close(r.done)
			return
		}
	}
}

func (r *Room) broadcast(msg *Message) {
	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		r.log.Println(":", err)
		return
	}

	fmt.Printf("received message to room %d: %s\n", r.Id, string(jsonMsg))
	for client := range r.clients {
		select {
		case client.send <- jsonMsg:
			r.log.Printf("broadcasting message: %q", jsonMsg)
		default:
			r.log.Println("default")
		}
	}
}
