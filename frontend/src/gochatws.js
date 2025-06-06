class GoChatWSClient {
  wsClient;

  onServerMessageMessage;
  onServerMessagePresence;
  onServerMessageSubscriptionChange;
  onServerMessageRoomDeleted;

  _pendingPromises;
  _messageId = 1;

  constructor(url) {
    this._pendingPromises = new Map();
    this.wsClient = new WSClient(url);
    this.wsClient.connect();

    this.wsClient.onError((err) => {
      console.error("WebSocket error: ", err.Error);
    });

    this.wsClient.onMessage((data) => {
      this.#processMessage(data);
    });

    this.wsClient.onClose(() => {
      console.log("WebSocket connection closed");
    });
  }

  #processMessage(data) {
    var msgs = data.split('\n');
    for (var i = 0; i < msgs.length; i++) {
      const parsedMsg = JSON.parse(msgs[i]);

      if (parsedMsg.message) {
        if (this.onServerMessageMessage) {
          this.onServerMessageMessage(parsedMsg);
        }
      } else if (parsedMsg.response) {
        this.#handleServerResponse(parsedMsg);
      } else if (parsedMsg.notification) {
        this.#handleServerNotification(parsedMsg);
      } else {
        console.log("Unknown server message type");
      }
    }
  }

  #handleServerResponse(msg) {
    if (this._pendingPromises.has(msg.id)) {
      if (msg.response.response_code < 200 || msg.response.response_code > 299) {
        const error = msg.response.error;
        const reject = this._pendingPromises.get(msg.id).reject;
        reject(error);
      } else {
        const resolve = this._pendingPromises.get(msg.id).resolve;
        resolve(msg);
      }
      this._pendingPromises.delete(msg.id);
    } else {
      console.warn("Received response for unknown message ID: " + msg.id);
    }
  }

  #handleServerNotification(msg) {
    if (msg.notification.room_deleted) {
      if (this.onServerMessageRoomDeleted) {
        this.onServerMessageRoomDeleted(msg);
      }
    } else if (msg.notification.presence) {
      if (this.onServerMessagePresence) {
        this.onServerMessagePresence(msg);
      }
    } else if (msg.notification.subscription_change) {
      if (this.onServerMessageSubscriptionChange) {
        this.onServerMessageSubscriptionChange(msg);
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

    try {
      this.wsClient.send(JSON.stringify(msgObj));
    } catch (err) {
      if (msgObj.id) {
        const reject = this._pendingPromises.get(msgObj.id).reject;
        reject("Error sending message: " + err.message);
        this._pendingPromises.delete(msgObj.id);
      } else {
        throw err;
      }
    }

    return promise;
  }

  #makePromise(id) {
    return new Promise((resolve, reject) => {
      this._pendingPromises.set(id, {
        resolve: resolve,
        reject: reject,
      });
    });
  }

  close() {
    this.wsClient.close();
  }
}

class WSClient {
  #socket = null;

  constructor(url) {
    this.url = url;
  }

  connect() {
    this.#socket = new WebSocket(this.url);
  }
  send(message) {
    if (this.#socket && this.#socket.readyState === WebSocket.OPEN) {
      console.log("Sending message: " + message);
      this.#socket.send(message);
    } else {
      throw new Error("WebSocket is not open");
    }
  }
  close() {
    if (this.#socket && this.#socket.readyState !== WebSocket.CLOSED) {
      this.#socket.close();
      this.#socket = null;
    }
  }
  _reconnect() {
    if (this.#socket && this.#socket.readyState !== WebSocket.CLOSED) {
      this.#socket.close();
    }
    this.connect();
  }
  isConnected() {
    return this.#socket && this.#socket.readyState === WebSocket.OPEN;
  }
  reconnect() {
    this._reconnect();
  }
  isOpen() {
    return this.#socket && this.#socket.readyState === WebSocket.OPEN;
  }
  isClosed() {
    return this.#socket && this.#socket.readyState === WebSocket.CLOSED;
  }
  onMessage(callback) {
    if (this.#socket) {
      this.#socket.onmessage = (event) => {
        console.log("Received message: " + event.data);
        callback(event.data);
      };
    }
  }
  onError(callback) {
    if (this.#socket) {
      this.#socket.onerror = (event) => {
        callback(event);
      };
    }
  }
  onClose(callback) {
    if (this.#socket) {
      this.#socket.onclose = (event) => {
        this.#socket = null;
        callback(event);
      };
    }
  }
  onOpen(callback) {
    if (this.#socket) {
      this.#socket.onopen = (event) => {
        console.log("WebSocket connection opened");
        callback(event);
      };
    }
  }
}

export default GoChatWSClient;