package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/npezzotti/go-chatroom/db"
)

var idleRoomTimeout = time.Second * 5

type leaveReq struct {
	c *Client
}

type exitReq struct {
	deleted bool
}

type Room struct {
	Id            int    `json:"id"`
	Name          string `json:"name"`
	ExternalId    string `json:"external_id"`
	Description   string `json:"description"`
	Subscribers   []User `json:"subscribers"`
	cs            *ChatServer
	joinChan      chan *Client
	leaveChan     chan *Client
	clientMsgChan chan *UserMessage
	seq_id        int
	clients       map[*Client]struct{}
	userMap       map[int]map[*Client]struct{}
	clientLock    sync.RWMutex
	exit          chan exitReq
	done          chan struct{}
	log           *log.Logger
	killTimer     *time.Timer
}

func (r *Room) start() {
	defer func() {
		r.log.Println("room exiting")
	}()

	r.log.Printf("starting room %q", r.ExternalId)

	r.killTimer = time.NewTimer(time.Second * 10)
	r.killTimer.Stop()

	for {
		select {
		case c := <-r.joinChan:
			if !SubscriptionExists(c.user.Id, r.Id) {
				r.log.Println("couldn't find subscription, creating it")

				_, err := CreateSubscription(c.user.Id, r.Id)
				if err != nil {
					r.log.Println("CreateSubscription:", err)
					continue
				}
			}

			// stop the kill timer if it was running
			r.killTimer.Stop()

			r.addClient(c)

			// notify the client of user presence in the room
			for client := range r.clients {
				if client.user.Id == c.user.Id {
					continue
				}

				presenceMsg, err := json.Marshal(&SystemMessage{
					Type:   EventTypeUserPresent,
					RoomId: r.Id,
					UserId: client.user.Id,
				})
				if err != nil {
					r.log.Println("failed to marshal presence msg:", err)
					continue
				}

				c.send <- presenceMsg

			}

			// notify all clients user is online
			r.broadcast(&SystemMessage{
				Type:   EventTypeUserPresent,
				RoomId: r.Id,
				UserId: c.user.Id,
			})
		case client := <-r.leaveChan:
			r.log.Printf("removing %q from room %q", client.user.Username, r.ExternalId)
			r.removeClient(client)

			r.clientLock.Lock()
			// notify all clients user is offline
			// if no sessions for user in the room
			if r.userMap[client.user.Id] == nil {
				r.broadcast(&SystemMessage{
					Type:   EventTypeUserAbsent,
					RoomId: r.Id,
					UserId: client.user.Id,
				})
			}
			r.clientLock.Unlock()
		case msg := <-r.clientMsgChan:
			r.saveAndBroadcast(msg)
		case <-r.killTimer.C:
			r.log.Printf("room %q timed out", r.ExternalId)
			r.cs.unloadRoom(r.Id)
		case e := <-r.exit:
			if e.deleted {
				r.notifyDeleted()
			}

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

	if r.userMap[c.user.Id] == nil {
		r.userMap[c.user.Id] = make(map[*Client]struct{})
	}
	r.userMap[c.user.Id][c] = struct{}{}

	if len(r.userMap[c.user.Id]) > 1 {
		r.log.Printf("user %q has multiple clients in room %q", c.user.Username, r.ExternalId)
	}
	r.log.Printf("added %q to room %q, current clients %v", c.user.Username, r.ExternalId, r.clients)
	r.clientLock.Unlock()

	c.addRoom(r)
}

func (r *Room) removeClient(c *Client) {
	r.clientLock.Lock()
	defer r.clientLock.Unlock()

	// check if the client is in the room
	if _, ok := r.clients[c]; !ok {
		r.log.Printf("client %q not found in room %q", c.user.Username, r.ExternalId)
		return
	}

	r.log.Printf("removing client %q from room %q", c.user.Username, r.ExternalId)
	delete(r.clients, c)
	c.delRoom(r.Id)

	// remove the client from the userMap
	if userClients, ok := r.userMap[c.user.Id]; ok {
		delete(userClients, c)
		if len(userClients) == 0 {
			delete(r.userMap, c.user.Id)
		}
	}

	r.log.Printf("removed client %q from room %q, current clients %v", c.user.Username, r.ExternalId, r.clients)

	// if the client is the last one in the room, start the kill timer
	if len(r.clients) == 0 {
		r.log.Printf("no clients in %q, starting kill timer", r.ExternalId)
		r.killTimer.Reset(idleRoomTimeout)
	}
}

func (r *Room) removeAllClientsForUser(userId int) {
	r.clientLock.Lock()
	defer r.clientLock.Unlock()

	if userClients, ok := r.userMap[userId]; ok {
		for client := range userClients {
			delete(r.clients, client)
			client.delRoom(r.Id)
		}
		delete(r.userMap, userId)
	}

	r.log.Printf("removed all clients for user %d from room %q", userId, r.ExternalId)

	// if the user is the last one in the room, start the kill timer
	if len(r.clients) == 0 {
		r.log.Printf("no clients in %q, starting kill timer", r.ExternalId)
		r.killTimer.Reset(idleRoomTimeout)
	}
}

func (r *Room) saveAndBroadcast(msg *UserMessage) {
	if err := MessageCreate(db.UserMessage{
		SeqId:     r.seq_id + 1,
		RoomId:    r.Id,
		UserId:    msg.client.user.Id,
		Content:   msg.Content,
		CreatedAt: msg.Timestamp,
	}); err != nil {
		r.log.Println("error saving message:", err)
	}

	r.seq_id++

	data := &SystemMessage{
		Type:      EventTypeMessagePublished,
		UserId:    msg.UserId,
		Username:  msg.Username,
		Content:   msg.Content,
		Timestamp: msg.Timestamp,
		RoomId:    r.Id,
	}
	r.broadcast(data)
}

func (r *Room) broadcast(msg *SystemMessage) {
	msg.RoomId = r.Id
	msg.Timestamp = time.Now()

	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		r.log.Println(":", err)
		return
	}

	fmt.Printf("received message to room %q: %s\n", r.ExternalId, string(jsonMsg))
	for client := range r.clients {
		select {
		case client.send <- jsonMsg:
			r.log.Printf("broadcasting message: %q", jsonMsg)
		default:
			r.log.Println("default")
		}
	}
}

func (r *Room) notifyDeleted() {
	msg := &SystemMessage{
		Type:   EventTypeRoomDeleted,
		RoomId: r.Id,
	}

	r.broadcast(msg)
}
