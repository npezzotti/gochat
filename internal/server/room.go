package server

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/npezzotti/go-chatroom/internal/database"
	"github.com/npezzotti/go-chatroom/internal/types"
)

var idleRoomTimeout = time.Second * 5

type exitReq struct {
	deleted bool
}

type Room struct {
	id            int
	cs            *ChatServer
	joinChan      chan *ClientMessage
	leaveChan     chan *ClientMessage
	clientMsgChan chan *ClientMessage
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

	r.log.Printf("starting room %d", r.id)

	r.killTimer = time.NewTimer(time.Second * 10)
	r.killTimer.Stop()

	for {
		select {
		case join := <-r.joinChan:
			r.handleAddClient(join)
		case leaveMsg := <-r.leaveChan:
			c := leaveMsg.client
			r.log.Printf("removing %q from room %d", c.user.Username, r.id)
			r.removeClient(c)

			// if this leave message is from a user
			// send a leave response
			resp := &ServerMessage{
				Response: &Response{
					ResponseCode: ResponseCodeOK,
				},
			}
			resp.Id = leaveMsg.Id
			resp.Timestamp = time.Now()
			c.queueMessage(resp)

			r.clientLock.Lock()
			// notify all clients user is offline
			// if no sessions for user in the room
			if r.userMap[c.user.Id] == nil {
				r.broadcast(&ServerMessage{
					Notification: &Notification{
						Presence: &Presence{
							Present: false,
							RoomId:  r.id,
							UserId:  c.user.Id,
						},
					},
					SkipClient: c,
				})
			}
			r.clientLock.Unlock()
		case msg := <-r.clientMsgChan:
			r.saveAndBroadcast(msg)
		case <-r.killTimer.C:
			r.log.Printf("room %d timed out", r.id)
			r.cs.unloadRoom(r.id)
		case e := <-r.exit:
			if e.deleted {
				r.broadcast(&ServerMessage{
					Notification: &Notification{
						RoomDeleted: &RoomDeleted{RoomId: r.id},
					},
				})
			}

			for c := range r.clients {
				c.delRoom(r.id)
			}

			close(r.done)
			return
		}
	}
}

func (r *Room) handleAddClient(join *ClientMessage) {
	// stop the kill timer since we have a new client
	r.killTimer.Stop()

	c := join.client
	if !r.cs.db.SubscriptionExists(c.user.Id, r.id) {
		r.log.Printf("Creating subscription for user %q in room %d", c.user.Username, r.id)
		if _, err := r.cs.db.CreateSubscription(c.user.Id, r.id); err != nil {
			// reset timer since client join failed
			if len(r.clients) == 0 {
				r.killTimer.Reset(idleRoomTimeout)
			}
			r.log.Println("CreateSubscription:", err)
			return
		}
	}

	dbRoom, err := r.cs.db.FetchRoomWithSubscribers(r.id)
	if err != nil {
		r.log.Println("FetchRoomWithSubscribers:", err)
		return
	}

	r.addClient(c)

	roomInfo := map[string]any{
		"id":          dbRoom.Id,
		"name":        dbRoom.Name,
		"external_id": dbRoom.ExternalId,
		"description": dbRoom.Description,
		"subscribers": func() []map[string]any {
			subscribers := make([]map[string]any, len(dbRoom.Subscriptions))
			for i, sub := range dbRoom.Subscriptions {
				subscribers[i] = map[string]any{
					"id":        sub.Id,
					"user_id":   sub.AccountId,
					"username":  sub.Username,
					"isPresent": r.userMap[sub.AccountId] != nil,
				}
			}
			return subscribers
		}(),
	}

	resp := &ServerMessage{
		Response: &Response{
			ResponseCode: ResponseCodeOK,
			Data:         roomInfo,
		},
	}
	resp.Id = join.Id
	resp.Timestamp = time.Now()

	c.queueMessage(resp)

	data := &ServerMessage{
		Notification: &Notification{
			Presence: &Presence{
				Present: true,
				RoomId:  r.id,
				UserId:  c.user.Id,
			},
		},
		SkipClient: c,
	}
	r.broadcast(data)
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
		r.log.Printf("user %q has multiple clients in room %d", c.user.Username, r.id)
	}
	r.log.Printf("added %q to room %d, current clients %v", c.user.Username, r.id, r.clients)
	r.clientLock.Unlock()

	c.addRoom(r)
}

func (r *Room) removeClient(c *Client) {
	r.clientLock.Lock()
	defer r.clientLock.Unlock()

	// check if the client is in the room
	if _, ok := r.clients[c]; !ok {
		r.log.Printf("client %q not found in room %d", c.user.Username, r.id)
		return
	}

	r.log.Printf("removing client %q from room %d", c.user.Username, r.id)
	delete(r.clients, c)
	c.delRoom(r.id)

	// remove the client from the userMap
	if userClients, ok := r.userMap[c.user.Id]; ok {
		delete(userClients, c)
		if len(userClients) == 0 {
			delete(r.userMap, c.user.Id)
		}
	}

	r.log.Printf("removed client %q from room %d, current clients %v", c.user.Username, r.id, r.clients)

	// if the client is the last one in the room, start the kill timer
	if len(r.clients) == 0 {
		r.log.Printf("no clients in %d, starting kill timer", r.id)
		r.killTimer.Reset(idleRoomTimeout)
	}
}

func (r *Room) removeAllClientsForUser(userId int) {
	r.clientLock.Lock()
	defer r.clientLock.Unlock()

	if userClients, ok := r.userMap[userId]; ok {
		for client := range userClients {
			delete(r.clients, client)
			client.delRoom(r.id)
		}
		delete(r.userMap, userId)
	}

	r.log.Printf("removed all clients for user %d from room %d", userId, r.id)

	// if the user is the last one in the room, start the kill timer
	if len(r.clients) == 0 {
		r.log.Printf("no clients in %d, starting kill timer", r.id)
		r.killTimer.Reset(idleRoomTimeout)
	}
}

func (r *Room) saveAndBroadcast(msg *ClientMessage) {
	seq_id := r.seq_id + 1
	if err := r.cs.db.MessageCreate(database.Message{
		SeqId:     seq_id,
		RoomId:    r.id,
		UserId:    msg.client.user.Id,
		Content:   msg.Publish.Content,
		CreatedAt: msg.Timestamp,
	}); err != nil {
		r.log.Println("error saving message:", err)
	}

	r.seq_id++

	data := &ServerMessage{
		Message: &types.Message{
			SeqId:     seq_id,
			RoomId:    r.id,
			UserId:    msg.UserId,
			Content:   msg.Publish.Content,
			Timestamp: msg.Timestamp,
		},
	}
	data.Id = msg.Id
	data.Timestamp = msg.Timestamp

	r.broadcast(data)
}

func (r *Room) broadcast(msg *ServerMessage) {
	msg.Timestamp = time.Now()

	fmt.Printf("received message to room %d: %v\n", r.id, msg)
	for client := range r.clients {
		if client == msg.SkipClient {
			continue
		}

		client.queueMessage(msg)
	}
}
