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
	send       chan []byte
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
		send:       make(chan []byte, 256),
		rooms:      make(map[int]*Room),
		stop:       make(chan struct{}),
	}
}

func (c *Client) write() {
	ticker := time.NewTicker(time.Duration(pingInterval))
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				return
			}

			writer, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				c.log.Println("failed to create writer:", err)
			}

			if _, err := writer.Write(msg); err != nil {
				c.log.Println("write:", err)
				return
			}

			n := len(c.send)
			for i := 0; i < n; i++ {
				writer.Write([]byte{'\n'})
				if _, err := writer.Write(<-c.send); err != nil {
					c.log.Println("write extras:", err)
					return
				}
			}

			if err := writer.Close(); err != nil {
				c.log.Println("close writer:", err)
				return
			}
		case <-c.stop:
			c.log.Println("stopping client")
			return
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.log.Println("ping:", err)
				return
			}
		}
	}
}

func (c *Client) read() {
	defer func() {
		c.conn.Close()
		c.cleanup()
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
		var msg Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			c.log.Println("error parsing message:", err)
			continue
		}

		msg.client = c
		msg.UserId = c.user.Id
		msg.Timestamp = time.Now().UTC()

		switch msg.Type {
		case MessageTypeJoin:
			c.log.Println("read:", "join message")
			c.joinRoom(&msg)
		case MessageTypeLeave:
			c.log.Println("read:", "leave message")
			c.leaveRoom(&msg)
		case MessageTypePublish:
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

func (c *Client) cleanup() {
	c.chatServer.deRegisterChan <- c
	c.leaveAllRooms()
}

func (c *Client) leaveAllRooms() {
	c.roomsLock.Lock()
	defer c.roomsLock.Unlock()

	for _, room := range c.rooms {
		room.leaveChan <- c
	}
}

func (c *Client) joinRoom(msg *Message) {
	c.chatServer.joinChan <- msg
}

func (c *Client) leaveRoom(msg *Message) {
	r := c.getRoom(msg.RoomId)
	if r != nil {
		r.leaveChan <- c
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
	c.log.Printf("added user %q to room %q, client's current rooms: %+v\n", c.user.Username, r.Name, c.rooms)
}

func (c *Client) getRoom(id int) *Room {
	if room, ok := c.rooms[id]; ok {
		return room
	}

	return nil
}
