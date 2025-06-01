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
	Id            int          `json:"id"`
	Name          string       `json:"name"`
	ExternalId    string       `json:"external_id"`
	Description   string       `json:"description"`
	Subscribers   []types.User `json:"subscribers"`
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

	r.log.Printf("starting room %q", r.ExternalId)

	r.killTimer = time.NewTimer(time.Second * 10)
	r.killTimer.Stop()

	for {
		select {
		case join := <-r.joinChan:
			r.handleAddClient(join)
		case leaveMsg := <-r.leaveChan:
			c := leaveMsg.client
			r.log.Printf("removing %q from room %q", c.user.Username, r.ExternalId)
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
							RoomId:  r.Id,
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
			r.log.Printf("room %q timed out", r.ExternalId)
			r.cs.unloadRoom(r.Id)
		case e := <-r.exit:
			if e.deleted {
				r.broadcast(&ServerMessage{
					Notification: &Notification{
						RoomDeleted: &RoomDeleted{RoomId: r.Id},
					},
				})
			}

			for c := range r.clients {
				c.delRoom(r.Id)
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
	if !r.cs.db.SubscriptionExists(c.user.Id, r.Id) {
		r.log.Printf("Creating subscription for user %q in room %q", c.user.Username, r.ExternalId)
		if _, err := r.cs.db.CreateSubscription(c.user.Id, r.Id); err != nil {
			// reset timer since client join failed
			if len(r.clients) == 0 {
				r.killTimer.Reset(idleRoomTimeout)
			}
			r.log.Println("CreateSubscription:", err)
			return
		}
	}

	dbRoom, err := r.cs.db.FetchRoomWithSubscribers(r.Id)
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
				RoomId:  r.Id,
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

func (r *Room) saveAndBroadcast(msg *ClientMessage) {
	seq_id := r.seq_id + 1
	if err := r.cs.db.MessageCreate(database.UserMessage{
		SeqId:     seq_id,
		RoomId:    r.Id,
		UserId:    msg.client.user.Id,
		Content:   msg.Publish.Content,
		CreatedAt: msg.Timestamp,
	}); err != nil {
		r.log.Println("error saving message:", err)
	}

	r.seq_id++

	data := &ServerMessage{
		Message: &Message{
			SeqId:     seq_id,
			RoomId:    r.Id,
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

	fmt.Printf("received message to room %q: %v\n", r.ExternalId, msg)
	for client := range r.clients {
		if client == msg.SkipClient {
			continue
		}

		client.queueMessage(msg)
	}
}
