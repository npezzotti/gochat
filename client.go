package main

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingInterval   = (pongWait * 9) / 10
	maxMessageSize = 1024
)

type Client struct {
	conn       *websocket.Conn
	chatServer *ChatServer
	log        *log.Logger
	user       User
	send       chan *SystemMessage
	rooms      map[int]*Room
	roomsLock  sync.RWMutex
	stop       chan struct{}
}

func NewClient(user User, conn *websocket.Conn, cs *ChatServer, l *log.Logger) *Client {
	return &Client{
		conn:       conn,
		chatServer: cs,
		log:        l,
		user:       user,
		send:       make(chan *SystemMessage, 256),
		rooms:      make(map[int]*Room),
		stop:       make(chan struct{}),
	}
}

func (c *Client) write() {
	ticker := time.NewTicker(time.Duration(pingInterval))
	defer func() {
		ticker.Stop()
		c.conn.Close()
		c.log.Println("write exiting")
	}()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}

			bytes, err := c.serializeMessage(msg)
			if err != nil {
				c.log.Println("failed to serialize message:", err)
				continue
			}

			if !c.sendMessage(websocket.TextMessage, bytes) {
				return
			}
		case <-c.stop:
			return
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !c.sendMessage(websocket.PingMessage, nil) {
				return
			}
		}
	}
}

func (c *Client) read() {
	defer func() {
		c.conn.Close()
		c.cleanup()
		c.log.Println("read exiting")
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(appData string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure,
				websocket.CloseNormalClosure) {
				c.log.Printf("ws: read: %v", err)
			}
			break
		}

		c.log.Println("Received message:", string(raw))
		var msg UserMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			c.log.Println("error parsing message:", err)
			continue
		}

		msg.client = c
		msg.UserId = c.user.Id
		msg.Timestamp = time.Now().UTC()

		switch msg.Type {
		case UserMessageTypeJoin:
			c.log.Println("read:", "join message")
			c.joinRoom(&msg)
		case UserMessageTypeLeave:
			c.log.Println("read:", "leave message")
			c.leaveRoom(&msg)
		case UserMessageTypePublish:
			c.log.Println("read:", "publish message")
			r := c.getRoom(msg.RoomId)
			if r != nil {
				r.clientMsgChan <- &msg
			} else {
				c.log.Println("user not subscribed to room")
			}
		}
	}
}

func (c *Client) queueMessage(msg *SystemMessage) bool {
	select {
	case c.send <- msg:
	default:
		c.log.Println("failed to send message to client, channel is full")
		return false
	}

	return true
}

func (c *Client) serializeMessage(msg *SystemMessage) ([]byte, error) {
	return json.Marshal(msg)
}

func (c *Client) sendMessage(msgType int, msg []byte) bool {
	c.conn.SetWriteDeadline(time.Now().Add(writeWait))

	if err := c.conn.WriteMessage(msgType, msg); err != nil {
		if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure,
			websocket.CloseNormalClosure) {
			c.log.Printf("write message: %s", err)
		}
		return false
	}

	return true
}

func (c *Client) stopClient() {
	close(c.stop)
}

func (c *Client) cleanup() {
	c.chatServer.deRegisterChan <- c
	c.leaveAllRooms()
	c.stopClient()
}

func (c *Client) leaveAllRooms() {
	c.roomsLock.RLock()
	defer c.roomsLock.RUnlock()

	for _, room := range c.rooms {
		room.leaveChan <- &UserMessage{
			Type:   UserMessageTypeLeave,
			RoomId: room.Id,
			UserId: c.user.Id,
			client: c,
		}
	}
}

func (c *Client) joinRoom(msg *UserMessage) {
	c.chatServer.joinChan <- msg
}

func (c *Client) leaveRoom(msg *UserMessage) {
	r := c.getRoom(msg.RoomId)
	if r != nil {
		r.leaveChan <- msg
	} else {
		c.log.Println("didn't find room")
	}
}

func (c *Client) delRoom(id int) {
	c.roomsLock.Lock()
	defer c.roomsLock.Unlock()

	if r, ok := c.rooms[id]; ok {
		delete(c.rooms, r.Id)
		c.log.Printf("removed room %q from rooms, current rooms: %v", r.ExternalId, c.rooms)
	}
}

func (c *Client) addRoom(r *Room) {
	c.roomsLock.Lock()
	defer c.roomsLock.Unlock()

	c.rooms[r.Id] = r
	c.log.Printf("added user %q to room %q, client's current rooms: %+v\n", c.user.Username, r.ExternalId, c.rooms)
}

func (c *Client) getRoom(id int) *Room {
	if room, ok := c.rooms[id]; ok {
		return room
	}

	return nil
}
