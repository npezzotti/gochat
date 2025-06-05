class GoChatWSClient {
  wsClient;

  onSystemMessageMessage;
  onSystemMessagePresence;
  onSystemMessageSubscriptionChange
  onSystemMessageRoomDeleted;

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
          if (this.onSystemMessageMessage) {
            this.onSystemMessageMessage(parsedMsg);
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
      if (this.onSystemMessageRoomDeleted) {
        this.onSystemMessageRoomDeleted(msg);
      }
    } else if (msg.notification.presence) {
      if (this.onSystemMessagePresence) {
        this.onSystemMessagePresence(msg);
      }
    } else if (msg.notification.subscription_change) {
      if (this.onSystemMessageSubscriptionChange) {
        this.onSystemMessageSubscriptionChange(msg);
      }
    } else {
      console.log("Unknown notification type");
    }
  }

  #generateMessageId() {
    return this._messageId++;
  }

  joinRoom(roomId) {
    var msgObj = {
      id: this.#generateMessageId(),
      join: {
        room_id: roomId,
      },
    };

    return this.#sendMessage(msgObj);
  }

  leaveRoom(roomId, unsub = false) {
    var msgObj = {
      id: this.#generateMessageId(),
      leave: {
        unsubscribe: unsub,
        room_id: roomId,
      },
    };

    return this.#sendMessage(msgObj);
  }

  publishMessage(roomId, msg) {
    var msgObj = {
      id: this.#generateMessageId(),
      publish: {
        content: msg,
        room_id: roomId,
      },
    };

    return this.#sendMessage(msgObj);
  }

  #sendMessage(msgObj) {
    let promise;
    if (msgObj.id) {
      promise = this.#makePromise(msgObj.id);
    }

    this.wsClient.send(JSON.stringify(msgObj));
    return promise;
  }

  #makePromise(id) {
    return new Promise((resolve, reject) => {
      this.pendingResponses.set(id, {
        resolve: resolve,
        reject: reject
      });
    });
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
