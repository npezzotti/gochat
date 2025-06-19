package server

import (
	"log"
	"sync"
	"time"

	"slices"

	"github.com/npezzotti/go-chatroom/internal/database"
	"github.com/npezzotti/go-chatroom/internal/types"
)

const idleRoomTimeout = time.Second * 5

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
	// killTimer is used to automatically unload the room when it is no longer active
	killTimer *time.Timer
	// exit is used to signal the room to exit
	exit chan exitReq
}

func (r *Room) start() {
	r.log.Printf("starting room %q", r.externalId)
	r.killTimer = time.NewTimer(idleRoomTimeout)
	r.killTimer.Stop()

	for {
		select {
		case join := <-r.joinChan:
			r.handleJoin(join)
		case leaveMsg := <-r.leaveChan:
			r.handleLeave(leaveMsg)
		case msg := <-r.clientMsgChan:
			if msg.Publish != nil {
				r.saveAndBroadcast(msg)
			} else if msg.Read != nil {
				r.handleRead(msg)
			}
		case <-r.killTimer.C:
			r.handleRoomTimeout()
		case e := <-r.exit:
			r.handleRoomExit(e)
			return
		}
	}
}

// notifyActive sends a presence notification to all active subscribers
// indicating that the room is currently active.
func (r *Room) notifyActive(skipClient *Client) {
	for _, sub := range r.subscribers {
		r.cs.broadcastChan <- &ServerMessage{
			Notification: &Notification{
				Presence: &Presence{
					Present: true,
					RoomId:  r.externalId,
				},
			},
			UserId:     sub.Id,
			SkipClient: skipClient,
		}
	}
}

func (r *Room) handleRoomTimeout() {
	r.log.Printf("room %q timed out", r.externalId)
	r.cs.unloadRoomChan <- r.externalId
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
	if e.done != nil {
		e.done <- true
	}
}

func (r *Room) handleLeave(leaveMsg *ClientMessage) {
	if leaveMsg.Leave.Unsubscribe {
		r.log.Printf("unsubscribing %q from room %q", leaveMsg.client.user.Username, r.externalId)
		err := r.cs.db.DeleteSubscription(leaveMsg.UserId, r.id)
		if err != nil {
			r.log.Println("DeleteSubscription:", err)
			if leaveMsg.GetUserId() != 0 {
				// the leave message is from a client, so we need to notify them
				leaveMsg.client.queueMessage(ErrInternalError(leaveMsg.Id))
			}
			return
		}

		// remove all clients for this user from the room
		r.removeAllClientsForUser(leaveMsg.UserId)
		// remove the user from the in memory subscriber list
		// so we don't send a leave notification
		r.removeSubscriber(leaveMsg.UserId)

		if leaveMsg.GetUserId() != 0 {
			// if the leave message is from a user, notify the user the unsubscribe was successful
			leaveMsg.client.queueMessage(NoErrOK(leaveMsg.Id, nil))
		}

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

	if leaveMsg.GetUserId() != 0 {
		// if the leave message is from a user, notify the user that they left the room
		client.queueMessage(NoErrOK(leaveMsg.Id, nil))
	}

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

func (r *Room) handleJoin(join *ClientMessage) {
	// stop the kill timer since we have a new client
	r.killTimer.Stop()

	c := join.client
	if !r.cs.db.SubscriptionExists(c.user.Id, r.id) {
		// if the user is not subscribed, create a subscription
		r.log.Printf("Creating subscription for user %q in room %q", c.user.Username, r.externalId)
		sub, err := r.cs.db.CreateSubscription(c.user.Id, r.id)
		if err != nil {
			// reset timer since client join failed
			if len(r.clients) == 0 {
				r.killTimer.Reset(idleRoomTimeout)
			}
			r.log.Println("CreateSubscription:", err)
			c.queueMessage(ErrInternalError(join.Id))
			return
		}

		// update the room's internal subscriber list
		r.subscribers = append(r.subscribers, types.User{
			Id:       sub.AccountId,
			Username: sub.Username,
		})

		// notify users that the user has subscribed
		r.broadcast(&ServerMessage{
			BaseMessage: BaseMessage{
				Timestamp: Now(),
			},
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

	if len(r.clients) == 1 {
		// if this is the first client in the room, notify all subscribers that the room is now active
		r.notifyActive(c)
	}

	roomInfo := types.Room{
		Id:          dbRoom.Id,
		Name:        dbRoom.Name,
		ExternalId:  dbRoom.ExternalId,
		Description: dbRoom.Description,
		SeqId:       dbRoom.SeqId,
		OwnerId:     dbRoom.OwnerId,
		Subscribers: func() []types.User {
			subscribers := make([]types.User, len(dbRoom.Subscriptions))
			for i, sub := range dbRoom.Subscriptions {
				subscribers[i] = types.User{
					Id:        sub.AccountId,
					Username:  sub.Username,
					IsPresent: r.userMap[sub.AccountId] != nil,
				}
			}
			return subscribers
		}(),
		CreatedAt: dbRoom.CreatedAt,
		UpdatedAt: dbRoom.UpdatedAt,
	}

	// send the room info to the client
	c.queueMessage(NoErrOK(join.Id, roomInfo))

	// notify clients that the user has joined
	r.broadcast(&ServerMessage{
		BaseMessage: BaseMessage{
			Timestamp: Now(),
		},
		Notification: &Notification{
			Presence: &Presence{
				Present: true,
				RoomId:  r.externalId,
				UserId:  c.user.Id,
			},
		},
		SkipClient: c,
	})
}

func (r *Room) addClient(c *Client) {
	r.clientLock.Lock()
	defer r.clientLock.Unlock()

	r.clients[c] = struct{}{}
	if r.userMap[c.user.Id] == nil {
		r.userMap[c.user.Id] = make(map[*Client]struct{})
	}
	r.userMap[c.user.Id][c] = struct{}{}

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

	r.log.Printf("removed client %q from room %q", c.user.Username, r.externalId)

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
	// save the message to the database
	if err := r.cs.db.MessageCreate(database.Message{
		SeqId:     r.seq_id + 1,
		RoomId:    r.id,
		UserId:    msg.client.user.Id,
		Content:   msg.Publish.Content,
		CreatedAt: msg.Timestamp,
	}); err != nil {
		r.log.Println("error saving message:", err)
		msg.client.queueMessage(ErrInternalError(msg.Id))
		return
	}

	// increment the sequence ID for the room now that the message is saved
	r.seq_id++
	msg.client.queueMessage(NoErrAccepted(msg.Id))

	// broadcast the message to all clients in the room
	r.broadcast(&ServerMessage{
		BaseMessage: BaseMessage{
			Id:        msg.Id,
			Timestamp: msg.Timestamp,
		},
		Message: &types.Message{
			SeqId:     r.seq_id,
			RoomId:    r.id,
			UserId:    msg.UserId,
			Content:   msg.Publish.Content,
			Timestamp: msg.Timestamp,
		},
	})

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
