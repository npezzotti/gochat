package server

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/npezzotti/go-chatroom/internal/types"
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
	user       types.User
	send       chan *ServerMessage
	rooms      map[string]*Room
	roomsLock  sync.RWMutex
	stop       chan struct{}
}

func NewClient(user types.User, conn *websocket.Conn, cs *ChatServer, l *log.Logger) *Client {
	return &Client{
		conn:       conn,
		chatServer: cs,
		log:        l,
		user:       user,
		send:       make(chan *ServerMessage, 256),
		rooms:      make(map[string]*Room),
		stop:       make(chan struct{}),
	}
}

func (c *Client) Write() {
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

func (c *Client) Read() {
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
		var msg ClientMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			c.log.Println("error parsing message:", err)
			c.queueMessage(ErrInvalidMessage(-1))
			continue
		}

		msg.client = c
		msg.UserId = c.user.Id
		msg.Timestamp = Now()

		switch {
		case msg.Join != nil:
			c.joinRoom(&msg)
		case msg.Leave != nil:
			c.leaveRoom(&msg)
		case msg.Publish != nil:
			r := c.getRoom(msg.Publish.RoomId)
			if r != nil {
				select {
				case r.clientMsgChan <- &msg:
				default:
					c.queueMessage(ErrServiceUnavailable(msg.Id))
					c.log.Printf("clientMsgChan full for room %q", r.externalId)
				}
			} else {
				c.queueMessage(ErrRoomNotFound(msg.Id))
			}
		case msg.Read != nil:
			r := c.getRoom(msg.Read.RoomId)
			if r != nil {
				select {
				case r.clientMsgChan <- &msg:
				default:
					c.queueMessage(ErrServiceUnavailable(msg.Id))
					c.log.Printf("readChan full for room %q", r.externalId)
				}
			} else {
				c.queueMessage(ErrRoomNotFound(msg.Id))
			}
		}
	}
}

func (c *Client) queueMessage(msg *ServerMessage) bool {
	select {
	case c.send <- msg:
	default:
		c.log.Println("failed to send message to client, channel is full")
		return false
	}

	return true
}

func (c *Client) serializeMessage(msg *ServerMessage) ([]byte, error) {
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
		room.leaveChan <- &ClientMessage{
			Leave:  &Leave{RoomId: room.externalId},
			UserId: c.user.Id,
			client: c,
		}
	}
}

func (c *Client) joinRoom(msg *ClientMessage) {
	select {
	case c.chatServer.joinChan <- msg:
	default:
		c.log.Printf("joinChan full")
		c.queueMessage(ErrServiceUnavailable(msg.Id))
		return
	}
}

func (c *Client) leaveRoom(msg *ClientMessage) {
	r := c.getRoom(msg.Leave.RoomId)
	if r != nil {
		select {
		case r.leaveChan <- msg:
		default:
			c.log.Printf("leaveChan full for room %q", r.externalId)
			c.queueMessage(ErrServiceUnavailable(msg.Id))
			return
		}
	} else {
		c.log.Println("didn't find room")
	}
}

func (c *Client) delRoom(id string) {
	c.roomsLock.Lock()
	defer c.roomsLock.Unlock()

	if _, ok := c.rooms[id]; ok {
		delete(c.rooms, id)
		c.log.Printf("removed room %q from rooms, current rooms: %v", id, c.rooms)
	}
}

func (c *Client) addRoom(r *Room) {
	c.roomsLock.Lock()
	defer c.roomsLock.Unlock()

	c.rooms[r.externalId] = r
	c.log.Printf("added user %s to room %q, client's current rooms: %+v\n", c.user.Username, r.externalId, c.rooms)
}

func (c *Client) getRoom(id string) *Room {
	if room, ok := c.rooms[id]; ok {
		return room
	}

	return nil
}
