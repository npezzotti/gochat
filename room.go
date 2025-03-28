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
	c     *Client
	unSub bool
}

type exitReq struct {
	deleted bool
}

type Room struct {
	Id            int    `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Subscribers   []User `json:"subscribers"`
	cs            *ChatServer
	joinChan      chan *Client
	leaveChan     chan leaveReq
	clientMsgChan chan *Message
	seq_id        int
	clients       map[*Client]struct{}
	clientLock    sync.RWMutex
	exit          chan exitReq
	done          chan struct{}
	log           *log.Logger
	killTimer     *time.Timer
}

type UserMessage struct {
	Id      int    `json:"id"`
	SeqId   int    `json:"seq_id"`
	RoomId  int    `json:"room_id"`
	UserId  int    `json:"user_id"`
	Content string `json:"content"`
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
			if !SubscriptionExists(c.user.Id, r.Id) {
				r.log.Println("couldn't find subscription, creating it")

				_, err := CreateSubscription(c.user.Id, r.Id)
				if err != nil {
					r.log.Println("CreateSubscription:", err)
					continue
				}
			}

			r.addClient(c)
		case leave := <-r.leaveChan:
			if leave.unSub {
				r.log.Printf("unscribing user %q from room %q", leave.c.user.Username, r.Name)
				if !SubscriptionExists(leave.c.user.Id, r.Id) {
					r.log.Println("subscription doesn't exist")
					continue
				}

				if err := DeleteSubscription(leave.c.user.Id, r.Id); err != nil {
					r.log.Println("DeleteSubscription", err)
					continue
				}
			}

			r.removeClient(leave.c)
			leave.c.delRoom(r.Id)

			if len(r.clients) == 0 {
				r.log.Printf("no clients in %q, starting kill timer", r.Name)
				r.killTimer.Reset(idleRoomTimeout)
			}

		case msg := <-r.clientMsgChan:
			r.saveAndBroadcast(msg)
		case <-r.killTimer.C:
			r.log.Printf("room %q timed out", r.Name)
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

func (r *Room) saveAndBroadcast(msg *Message) {
	if err := MessageCreate(db.UserMessage{
		SeqId:   r.seq_id + 1,
		RoomId:  r.Id,
		UserId:  msg.client.user.Id,
		Content: msg.Content,
	}); err != nil {
		r.log.Println("error saving message:", err)
	}

	r.seq_id++
	r.broadcast(msg)
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

func (r *Room) notifyDeleted() {
	msg := &Message{
		Type:   MessageTypeRoomDeleted,
		RoomId: r.Id,
	}

	r.broadcast(msg)
}
