class GoChatWSClient {
  wsClient;

  onPublishMessage;
  onEventTypePresence;
  onEventTypeUserAbsent;
  onEventTypeSubscriptionChange
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
          this.handleServerResponse(parsedMsg)
        } else if (parsedMsg.notification) {
          this.handleServerNotification(parsedMsg)
        } else {
          console.log("Unknown server message type")
        }
      }
    })

    this.wsClient.onClose(() => {
      console.log("WebSocket connection closed");
    })
  }

  handleServerResponse(msg) {
    if (this.pendingResponses.has(msg.id)) {
      if (msg.response.response_code < 200 || msg.response.response_code > 299) {
        const error = msg.response.error
        const reject = this.pendingResponses.get(msg.id).reject;
        reject(error)
      } else {
        const resolve = this.pendingResponses.get(msg.id).resolve;
        this.pendingResponses.delete(msg.id);
        resolve(msg);
      }
    }
  }

  handleServerNotification(msg) {
    if (msg.notification.room_deleted) {
      if (this.onEventTypeRoomDeleted) {
        this.onEventTypeRoomDeleted(msg);
      }
    } else if (msg.notification.presence) {
      if (this.onEventTypePresence) {
        this.onEventTypePresence(msg);
      }
    } else if (msg.notification.subscription_change) {
      if (this.onEventTypeSubscriptionChange) {
        this.onEventTypeSubscriptionChange(msg);
      }
    } else {
      console.log("Unknown notification type");
    }
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

  leaveRoom(roomId, unsub = false) {
    var msgObj = {
      id: this.generateMessageId(),
      leave: {
        unsubscribe: unsub,
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
