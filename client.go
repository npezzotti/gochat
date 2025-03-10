package main

import (
	"encoding/json"
	"log"
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
}

func NewClient(user User, conn *websocket.Conn, cs *ChatServer, l *log.Logger) *Client {
	return &Client{
		conn:       conn,
		chatServer: cs,
		log:        l,
		user:       user,
		send:       make(chan []byte, 256),
		rooms:      make(map[int]*Room),
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
				if _, err := writer.Write(<-c.send); err != nil {
					c.log.Println("write extras:", err)
					return
				}
			}

			if err := writer.Close(); err != nil {
				c.log.Println("close writer:", err)
				return
			}
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
		c.chatServer.deRegisterChan <- c
		c.conn.Close()
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

		var msg Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			c.log.Println("error parsing message:", err)
			continue
		}

		msg.client = c
		msg.From = c.user.Username

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
			}
		}
	}
}

func (c *Client) joinRoom(msg *Message) {
	c.chatServer.joinChan <- msg
}

func (c *Client) leaveRoom(msg *Message) {
	r := c.getRoom(msg.RoomId)
	if r != nil {
		r.leaveChan <- msg
	} else {
		c.log.Println("didn't find room")
	}
}

func (c *Client) getRoom(id int) *Room {
	if room, ok := c.rooms[id]; ok {
		return room
	}

	return nil
}
