package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

var idleRoomTimeout = time.Second * 5

type Room struct {
	Id            int
	Name          string
	Description   string
	cs            *ChatServer
	joinChan      chan *Client
	leaveChan     chan *Client
	clientMsgChan chan *Message
	clients       map[*Client]struct{}
	clientLock    sync.RWMutex
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
		case c := <-r.joinChan:
			r.addClient(c)
		case c := <-r.leaveChan:
			r.removeClient(c)
			c.delRoom(r.Id)

			if len(r.clients) == 0 {
				r.log.Printf("no clients in %q, starting kill timer", r.Name)
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

func (r *Room) addClient(c *Client) {
	r.killTimer.Stop()

	r.clientLock.Lock()
	r.clients[c] = struct{}{}
	r.log.Printf("added %q to room %q, current clients %v", c.user.Username, r.Name, r.clients)
	r.clientLock.Unlock()

	c.addRoom(r)
}

func (r *Room) removeClient(c *Client) {
	r.clientLock.Lock()
	delete(r.clients, c)
	r.clientLock.Unlock()

	r.log.Printf("removed client %q from room %q, current clients %v", c.user.Username, r.Name, r.clients)
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
