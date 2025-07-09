package server

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/npezzotti/go-chatroom/internal/stats"
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
	// roomsLock is a mutex for rooms. rooms is accessed
	// concurrently by both the client and the room.
	roomsLock sync.RWMutex
	exitRoom  chan string
	stop      chan struct{}
	stats     stats.StatsProvider
}

func NewClient(user types.User, conn *websocket.Conn, cs *ChatServer, l *log.Logger, statsUpdater stats.StatsProvider) *Client {
	return &Client{
		conn:       conn,
		chatServer: cs,
		log:        l,
		user:       user,
		send:       make(chan *ServerMessage, 128),
		rooms:      make(map[string]*Room),
		exitRoom:   make(chan string, 64),
		stop:       make(chan struct{}),
		stats:      statsUpdater,
	}
}

func (c *Client) Write() {
	ticker := time.NewTicker(time.Duration(pingInterval))
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}

			bytes, err := serializeMessage(msg)
			if err != nil {
				c.log.Println("failed to serialize message:", err)
				continue
			}

			if !c.sendMessage(bytes) {
				return
			}
		case roomId := <-c.exitRoom:
			c.delRoom(roomId)
		case <-c.stop:
			return
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.writeWS(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) Read() {
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

		c.stats.Incr("TotalIncomingMessages")

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
			r, ok := c.getRoom(msg.Publish.RoomId)
			if ok {
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
			r, ok := c.getRoom(msg.Read.RoomId)
			if ok {
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

// serializeMessage serializes a ServerMessage to JSON.
func serializeMessage(msg *ServerMessage) ([]byte, error) {
	return json.Marshal(msg)
}

func (c *Client) sendMessage(msg []byte) bool {
	if err := c.writeWS(websocket.TextMessage, msg); err != nil {
		return false
	}
	c.stats.Incr("TotalOutgoingMessages")

	return true
}

func (c *Client) writeWS(msgType int, msg []byte) error {
	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err := c.conn.WriteMessage(msgType, msg); err != nil {
		if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure,
			websocket.CloseNormalClosure) {
			c.log.Printf("writeWS: %s", err)
		}
		return err
	}
	return nil
}

func (c *Client) stopClient() {
	close(c.stop)
}

func (c *Client) cleanup() {
	c.leaveAllRooms()
	c.stopClient()
	c.chatServer.DeRegisterClient(c)
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
		msg.client.queueMessage(ErrServiceUnavailable(msg.Id))
		c.log.Printf("server joinChan full")
	}
}

func (c *Client) leaveRoom(msg *ClientMessage) {
	r, ok := c.getRoom(msg.Leave.RoomId)
	if ok {
		select {
		case r.leaveChan <- msg:
		default:
			c.log.Printf("leaveChan full for room %q", r.externalId)
			c.queueMessage(ErrServiceUnavailable(msg.Id))
			return
		}
	} else {
		msg.client.queueMessage(ErrRoomNotFound(msg.Id))
	}
}

// delRoom removes the room from the client's list of rooms.
func (c *Client) delRoom(id string) {
	c.roomsLock.Lock()
	defer c.roomsLock.Unlock()

	delete(c.rooms, id)
}

// addRoom adds the room to the client's list of rooms.
func (c *Client) addRoom(r *Room) {
	c.roomsLock.Lock()
	defer c.roomsLock.Unlock()

	c.rooms[r.externalId] = r
}

func (c *Client) getRoom(id string) (*Room, bool) {
	c.roomsLock.RLock()
	defer c.roomsLock.RUnlock()

	room, ok := c.rooms[id]
	return room, ok
}
