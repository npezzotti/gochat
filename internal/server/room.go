package server

import (
	"log"
	"sync"
	"time"

	"slices"

	"github.com/npezzotti/go-chatroom/internal/database"
	"github.com/npezzotti/go-chatroom/internal/types"
)

var idleRoomTimeout = time.Second * 5

type exitReq struct {
	deleted bool
	done    chan bool
}

type Room struct {
	id            int
	externalId    string
	subscribers   []types.User
	cs            *ChatServer
	joinChan      chan *ClientMessage
	leaveChan     chan *ClientMessage
	clientMsgChan chan *ClientMessage
	seq_id        int
	clients       map[*Client]struct{}
	userMap       map[int]map[*Client]struct{}
	clientLock    sync.RWMutex
	log           *log.Logger
	killTimer     *time.Timer
	exit          chan exitReq
}

func (r *Room) start() {
	defer func() {
		r.log.Println("room exiting")
	}()

	r.log.Printf("starting room %q", r.externalId)

	r.killTimer = time.NewTimer(time.Second * 10)
	r.killTimer.Stop()

	for {
		select {
		case join := <-r.joinChan:
			r.handleAddClient(join)
		case leaveMsg := <-r.leaveChan:
			r.handleLeave(leaveMsg)
		case msg := <-r.clientMsgChan:
			if msg.Publish != nil {
				r.saveAndBroadcast(msg)
			} else if msg.Read != nil {
				r.handleRead(msg)
			}
		case <-r.killTimer.C:
			r.log.Printf("room %q timed out", r.externalId)
			r.cs.unloadRoomChan <- r.externalId
		case e := <-r.exit:
			r.handleRoomExit(e)
			return
		}
	}
}

func (r *Room) handleRoomExit(e exitReq) {
	r.log.Printf("room %q is exiting", r.externalId)
	if e.deleted {
		// notify all clients that the room is deleted
		r.broadcast(&ServerMessage{
			BaseMessage: BaseMessage{
				Timestamp: Now(),
			},
			Notification: &Notification{
				RoomDeleted: &RoomDeleted{RoomId: r.externalId},
			},
		})
	}

	// remove the room for all clients
	r.clientLock.Lock()
	for c := range r.clients {
		c.delRoom(r.externalId)
	}
	r.clientLock.Unlock()

	// notify active subscribers that the room is offline
	for _, sub := range r.subscribers {
		r.cs.broadcastChan <- &ServerMessage{
			BaseMessage: BaseMessage{
				Timestamp: Now(),
			},
			Notification: &Notification{
				Presence: &Presence{
					Present: false,
					RoomId:  r.externalId,
				},
			},
			UserId: sub.Id,
		}
	}

	// notify the chat server the room is done cleaning up
	e.done <- true
}

func (r *Room) handleLeave(leaveMsg *ClientMessage) {
	if leaveMsg.Leave.Unsubscribe {
		r.log.Printf("unsubscribing %q from room %q", leaveMsg.client.user.Username, r.externalId)
		err := r.cs.db.DeleteSubscription(leaveMsg.UserId, r.id)
		if err != nil {
			r.log.Println("DeleteSubscription:", err)
			leaveMsg.client.queueMessage(ErrInternalError(leaveMsg.Id))
			return
		}

		// remove all clients for this user from the room
		r.removeAllClientsForUser(leaveMsg.UserId)
		// remove the user from the in memory subscriber list
		// so we don't send a leave notification
		r.removeSubscriber(leaveMsg.UserId)

		// notify the client that the unsubscribe was successful
		leaveMsg.client.queueMessage(NoErrOK(leaveMsg.Id, nil))

		// broadcast that the user unsubscribed
		r.broadcast(&ServerMessage{
			Notification: &Notification{
				SubscriptionChange: &SubscriptionChange{
					RoomId:     r.externalId,
					Subscribed: false,
					User: types.User{
						Id:       leaveMsg.UserId,
						Username: leaveMsg.client.user.Username,
					},
				},
			},
		})
		return
	}

	client := leaveMsg.client
	// remove the client from the room
	r.removeClient(client)
	client.queueMessage(NoErrOK(leaveMsg.Id, nil))

	// notify all clients user is offline
	// if no sessions for user in the room
	if r.userMap[client.user.Id] == nil {
		r.broadcast(&ServerMessage{
			Notification: &Notification{
				Presence: &Presence{
					Present: false,
					RoomId:  r.externalId,
					UserId:  client.user.Id,
				},
			},
			SkipClient: client,
		})
	}
}

func (r *Room) handleRead(msg *ClientMessage) {
	// update the last read seq id for the user
	if err := r.cs.db.UpdateLastReadSeqId(msg.UserId, r.id, msg.Read.SeqId); err != nil {
		r.log.Println("UpdateLastReadSeqId:", err)
		msg.client.queueMessage(ErrInternalError(msg.Id))
		return
	}

	msg.client.queueMessage(NoErrOK(msg.Id, nil))
}

func (r *Room) handleAddClient(join *ClientMessage) {
	// stop the kill timer since we have a new client
	r.killTimer.Stop()

	c := join.client
	if !r.cs.db.SubscriptionExists(c.user.Id, r.id) {
		r.log.Printf("Creating subscription for user %q in room %q", c.user.Username, r.externalId)
		sub, err := r.cs.db.CreateSubscription(c.user.Id, r.id)
		if err != nil {
			// reset timer since client join failed
			if len(r.clients) == 0 {
				r.killTimer.Reset(idleRoomTimeout)
			}
			r.log.Println("CreateSubscription:", err)
			return
		}

		r.subscribers = append(r.subscribers, types.User{
			Id:       sub.AccountId,
			Username: sub.Username,
		})

		r.broadcast(&ServerMessage{
			Notification: &Notification{
				SubscriptionChange: &SubscriptionChange{
					RoomId:     r.externalId,
					Subscribed: true,
					User: types.User{
						Id:       join.UserId,
						Username: join.client.user.Username,
					},
				},
			},
		})
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
		"created_at":  dbRoom.CreatedAt,
		"updated_at":  dbRoom.UpdatedAt,
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

	c.queueMessage(NoErrOK(join.Id, roomInfo))

	data := &ServerMessage{
		Notification: &Notification{
			Presence: &Presence{
				Present: true,
				RoomId:  r.externalId,
				UserId:  c.user.Id,
			},
		},
		SkipClient: c,
	}
	r.broadcast(data)

	for _, sub := range r.subscribers {
		r.cs.broadcastChan <- &ServerMessage{
			Notification: &Notification{
				Presence: &Presence{
					Present: true,
					RoomId:  r.externalId,
				},
			},
			UserId: sub.Id,
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
		r.log.Printf("user %q has multiple clients in room %q", c.user.Username, r.externalId)
	}
	r.log.Printf("added %q to room %q, current clients %v", c.user.Username, r.externalId, r.clients)
	r.clientLock.Unlock()

	c.addRoom(r)
}

func (r *Room) removeClient(c *Client) {
	r.clientLock.Lock()
	defer r.clientLock.Unlock()

	// check if the client is in the room
	if _, ok := r.clients[c]; !ok {
		r.log.Printf("client %q not found in room %q", c.user.Username, r.externalId)
		return
	}

	r.log.Printf("removing client %q from room %q", c.user.Username, r.externalId)
	delete(r.clients, c)
	c.delRoom(r.externalId)

	// remove the client from the userMap
	if userClients, ok := r.userMap[c.user.Id]; ok {
		delete(userClients, c)
		if len(userClients) == 0 {
			delete(r.userMap, c.user.Id)
		}
	}

	r.log.Printf("removed client %q from room %q, current clients %v", c.user.Username, r.externalId, r.clients)

	// if the client is the last one in the room, start the kill timer
	if len(r.clients) == 0 {
		r.log.Printf("no clients in %q, starting kill timer", r.externalId)
		r.killTimer.Reset(idleRoomTimeout)
	}
}

func (r *Room) removeAllClientsForUser(userId int) {
	r.clientLock.Lock()
	defer r.clientLock.Unlock()

	if userClients, ok := r.userMap[userId]; ok {
		for client := range userClients {
			delete(r.clients, client)
			client.delRoom(r.externalId)
		}
		delete(r.userMap, userId)
	}

	r.log.Printf("removed all clients for user %q from room %q", userId, r.externalId)

	// if the user is the last one in the room, start the kill timer
	if len(r.clients) == 0 {
		r.log.Printf("no clients in %q, starting kill timer", r.externalId)
		r.killTimer.Reset(idleRoomTimeout)
	}
}

func (r *Room) removeSubscriber(userId int) {
	for i, sub := range r.subscribers {
		if sub.Id == userId {
			r.log.Printf("removing subscriber %q from room %q", sub.Username, r.externalId)
			r.subscribers = slices.Delete(r.subscribers, i, i+1)
			return
		}
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
		msg.client.queueMessage(ErrInternalError(msg.Id))
	}
	r.seq_id++
	msg.client.queueMessage(NoErrAccepted(msg.Id))

	data := &ServerMessage{
		BaseMessage: BaseMessage{
			Id:        msg.Id,
			Timestamp: msg.Timestamp,
		},
		Message: &types.Message{
			SeqId:     seq_id,
			RoomId:    r.id,
			UserId:    msg.UserId,
			Content:   msg.Publish.Content,
			Timestamp: msg.Timestamp,
		},
	}

	r.broadcast(data)

	// notify inactive subscribers of new message
	for _, sub := range r.subscribers {
		if r.userMap[sub.Id] != nil {
			// skip broadcasting to users that are already in the room
			continue
		}

		r.cs.broadcastChan <- &ServerMessage{
			Notification: &Notification{
				Message: &MessageNotification{
					RoomId: r.externalId,
					SeqId:  r.seq_id,
				},
			},
			UserId: sub.Id,
		}
	}
}

func (r *Room) broadcast(msg *ServerMessage) {
	msg.Timestamp = time.Now()

	r.log.Printf("broadcast message to room %q: %v\n", r.externalId, msg)
	for client := range r.clients {
		if client == msg.SkipClient {
			continue
		}

		client.queueMessage(msg)
	}
}
