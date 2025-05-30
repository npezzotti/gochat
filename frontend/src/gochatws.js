class GoChatWSClient {
  wsClient;

  static MessageTypeJoin = 'join'
  static MessageTypeLeave = 'leave'
  static MessageTypePublish = 'publish'

  static EventTypeUserSubscribed = 'subscribe'
  static EventTypeUserUnsubscribed = 'unsubscribe'
  static EventTypeRoomDeleted = 'room_deleted'
  static EventTypeUserPresent = 'user_present'
  static EventTypeUserAbsent = 'user_absent'
  static EventTypeRoomJoined = 'joined'
  static EventTypeRoomLeft = 'left'

  onPublishMessage;
  onEventTypePresence;
  onEventTypeUserAbsent;
  onEventTypeUserSubscribed;
  onEventTypeRoomDeleted;
  onEventTypeUserUnsubscribed;
  EventTypeSystemMessage;

  pendingPromises;
  _messageId = 1;

  constructor(url) {
    this.pendingResponses = new Map();
    this.wsClient = new WSClient(url);
    this.wsClient.connect();

    this.wsClient.onError((err) => {
      console.error("WebSocket error: ", err.Error);
    });

    this.wsClient.onMessage((data) => {
      var msgs = data.split('\n');
      for (var i = 0; i < msgs.length; i++) {
        const parsedMsg = JSON.parse(msgs[i]);

        if (parsedMsg.message) {
          if (this.onPublishMessage) {
            this.onPublishMessage(parsedMsg);
          }
        } else if (parsedMsg.response) {
          if (this.pendingResponses.has(parsedMsg.id)) {
            const resolve = this.pendingResponses.get(parsedMsg.id).resolve;
            this.pendingResponses.delete(parsedMsg.id);
            resolve(parsedMsg);
          }
        } else if (parsedMsg.notification) {
          if (parsedMsg.notification.room_deleted) {
            if (this.onEventTypeRoomDeleted) {
              this.onEventTypeRoomDeleted(parsedMsg);
            }
          } else if (parsedMsg.notification.presence) {
            if (this.onEventTypePresence) {
              this.onEventTypePresence(parsedMsg);
            }
          } else if (parsedMsg.notification.subscription_change) {
            if (parsedMsg.notification.subscription_change.subscribed) {
              if (this.onEventTypeUserSubscribed) {
                this.onEventTypeUserSubscribed(parsedMsg);
              }
            } else {
              if (this.onEventTypeUserUnsubscribed) {
                this.onEventTypeUserUnsubscribed(parsedMsg);
              }
            }
          }
        } else {
          console.log("Unknown server message type")
        }
      }
    })

    this.wsClient.onClose(() => {
      console.log("WebSocket connection closed");
    })
  }

  generateMessageId() {
    return this._messageId++;
  }

  joinRoom(roomId) {
    var msgObj = {
      id: this.generateMessageId(),
      join: {
        room_id: roomId,
      },
    };

    return new Promise((resolve, reject) => {
      this.pendingResponses.set(msgObj.id, {
        resolve: resolve,
        reject: reject
      });

      this.wsClient.send(JSON.stringify(msgObj));
    });
  }

  leaveRoom(roomId) {
    var msgObj = {
      id: this.generateMessageId(),
      leave: {
        room_id: roomId,
      },
    };

    return new Promise((resolve, reject) => {
      this.pendingResponses.set(msgObj.id, {
        resolve: resolve,
        reject: reject
      });

      this.wsClient.send(JSON.stringify(msgObj));
    });
  }

  publishMessage(roomId, msg) {
    var msgObj = {
      publish: {
        content: msg,
        room_id: roomId,
      },
    };

    msg = JSON.stringify(msgObj)
    this.wsClient.send(msg)
  }

  close() {
    this.wsClient.close();
  }
}

class WSClient {
  constructor(url) {
    this.url = url;
    this.socket = null;
  }

  connect() {
    this.socket = new WebSocket(this.url);
  }
  send(message) {
    if (this.socket && this.socket.readyState === WebSocket.OPEN) {
      console.log("Sending message: " + message);
      this.socket.send(message);
    } else {
      throw new Error("WebSocket is not open");
    }
  }
  close() {
    if (this.socket && this.socket.readyState !== WebSocket.CLOSED) {
      this.socket.close();
      this.socket = null;
    }
  }
  _reconnect() {
    if (this.socket && this.socket.readyState !== WebSocket.CLOSED) {
      this.socket.close();
    }
    this.connect();
  }
  isConnected() {
    return this.socket && this.socket.readyState === WebSocket.OPEN;
  }
  reconnect() {
    this._reconnect();
  }
  isOpen() {
    return this.socket && this.socket.readyState === WebSocket.OPEN;
  }
  isClosed() {
    return this.socket && this.socket.readyState === WebSocket.CLOSED;
  }
  onMessage(callback) {
    if (this.socket) {
      this.socket.onmessage = (event) => {
        console.log("Received message: " + event.data)
        callback(event.data);
      }
    }
  }
  onError(callback) {
    if (this.socket) {
      this.socket.onerror = (event) => {
        callback(event);
      }
    }
  }
  onClose(callback) {
    if (this.socket) {
      this.socket.onclose = (event) => {
        this.socket = null;
        callback(event);
      }
    }
  }
  onOpen(callback) {
    if (this.socket) {
      this.socket.onopen = (event) => {
        console.log("WebSocket connection opened")
        callback(event);
      }
    }
  }
}

export default GoChatWSClient;
