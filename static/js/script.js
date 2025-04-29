class GoChatClient {
  static MESSAGES_PAGE_LIMIT = 10

  constructor(host) {
    this.host = host;
    this.baseUrl = "http://" + this.host;
  }

  async _request(method, endpoint, data, params = {}) {
    const url = new URL(this.baseUrl + endpoint);

    Object.keys(params).forEach(key => {
      url.searchParams.append(key, params[key]);
    })

    const options = {
      method: method,
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json',
      },
      timeout: 5000
    }

    if (data && ['POST', 'PUT', 'PATCH'].includes(method)) {
      options.body = JSON.stringify(data);
    }

    try {
      const response = await fetch(url, options);

      if (response.status === 204) {
        return null; // No content, return null
      }

      let res;
      try {
        res = await response.json();
      } catch (err) {
        throw new Error("Failed to parse server response: " + err);
      }

      if (!response.ok) {
        throw new Error(res.message || "Request failed");
      }
      return res;
    } catch (err) {
      console.error("Request error:", err);
      throw err;
    }
  }

  async listSubscriptions() {
    return this._request('GET', '/subscriptions');
  }

  async getRoom(roomId) {
    return this._request('GET', '/rooms', null, { id: roomId });
  }

  async subscribeRoom(roomId) {
    return this._request('POST', '/subscriptions', null, { room_id: roomId });
  }

  async unsubscribeRoom(roomId) {
    return this._request('DELETE', '/subscriptions', null, { room_id: roomId });
  }

  async createRoom(name, description) {
    return this._request('POST', '/rooms', { name: name, description: description });
  }

  async deleteRoom(roomId) {
    return this._request('DELETE', '/rooms', null, { id: roomId });
  }

  async getMessages(roomId, before = 0) {
    const params = {
      room_id: roomId,
      limit: GoChatClient.MESSAGES_PAGE_LIMIT,
    }

    if (before > 0) {
      params.before = before;
    }

    return this._request('GET', '/messages', null, params);
  }

  async getAccount() {
    return this._request('GET', '/account');
  }

  async updateAccount(username, password) {
    return this._request('PUT', '/account', { username: username, password: password });
  }

  async logout() {
    return this._request('GET', '/logout')
  }

  async login(email, password) {
    return this._request('POST', '/login', { email: email, password: password });
  }

  async register(email, username, password) {
    return this._request('POST', '/register', { email: email, username: username, password: password });
  }
}

class GoChatWSClient {
  wsClient = null;

  static MessageTypeJoin = 'join'
  static MessageTypeLeave = 'leave'
  static MessageTypePublish = 'publish'

  static EventTypeUserSubscribed = 'subscribe'
  static EventTypeUserUnsubscribed = 'unsubscribe'
  static EventTypeRoomDeleted = 'room_deleted'
  static EventTypeUserPresent = 'user_present'
  static EventTypeUserAbsent = 'user_absent'

  onPublishMessage;
  onEventTypeUserPresent;
  onEventTypeUserAbsent;
  onEventTypeUserSubscribed;
  onEventTypeUserUnsubscribed;

  constructor(url) {
    this.currentRoom = null;
    this.wsClient = new WSClient(url);
    this.wsClient.connect();

    this.wsClient.onOpen(() => {
      console.log("WebSocket connection opened")
    })

    this.wsClient.onMessage((data) => {
      var msgs = data.split('\n');
      for (var i = 0; i < msgs.length; i++) {
        const parsedMsg = JSON.parse(msgs[i]);

        switch (parsedMsg.type) {
          case GoChatWSClient.MessageTypePublish:
            if (this.onPublishMessage) {
              this.onPublishMessage(parsedMsg);
            }
            break;
          case GoChatWSClient.EventTypeRoomDeleted:
            if (this.currentRoom && this.currentRoom.id === parsedMsg.room_id) {
              // removeRoomFromList(currentRoom);
              // clearRoomView();
            }
            this.clearCurrentRoom();
            break;
          case GoChatWSClient.EventTypeUserPresent:
            if (!this.currentRoom || this.currentRoom.id != parsedMsg.room_id) {
              return;
            }

            if (this.onEventTypeUserPresent) {
              this.onEventTypeUserPresent(parsedMsg);
            }
            break;
          case GoChatWSClient.EventTypeUserAbsent:
            if (!this.currentRoom || this.currentRoom.id != parsedMsg.room_id) {
              return;
            }

            if (this.onEventTypeUserAbsent) {
              this.onEventTypeUserAbsent(parsedMsg);
            }
            break;
          case GoChatWSClient.EventTypeUserSubscribed:
            if (this.currentRoom && this.currentRoom.id === parsedMsg.room_id) {
              this.currentRoom.subscribers = this.currentRoom.subscribers.filter(sub => sub.id !== parsedMsg.user_id);
              const subscribersList = document.querySelector('.subscribers-list');
              if (subscribersList) {
                const subscriberItem = subscribersList.querySelector(`li[data-user-id="${parsedMsg.user_id}"]`);
                if (subscriberItem) {
                  subscriberItem.remove();
                }
              }
            }
            break;
          case GoChatWSClient.EventTypeUserUnsubscribed:
            if (this.currentRoom && this.currentRoom.id === parsedMsg.room_id) {
              const newSubscriber = {
                id: parsedMsg.user_id,
                username: parsedMsg.username
              };
              this.currentRoom.subscribers.push(newSubscriber);

              if (this.onEventTypeUserUnsubscribed) {
                this.onEventTypeUserUnsubscribed(parsedMsg);
              }
            }
            break;
          default:
        }
      }
    })
  }

  joinRoom(roomId) {
    var msgObj = {
      type: GoChatWSClient.MessageTypeJoin,
      room_id: roomId,
    };

    this.wsClient.send(JSON.stringify(msgObj))
    this.setCurrentRoom(roomId);
  }

  leaveRoom(roomId) {
    var msgObj = {
      type: GoChatWSClient.MessageTypeLeave,
      room_id: roomId,
    };

    this.wsClient.send(JSON.stringify(msgObj))
    this.clearCurrentRoom();
  }

  sendMessage(msg) {
    var msgObj = {
      type: GoChatWSClient.MessageTypePublish,
      room_id: this.getCurrentRoom().id,
      content: msg
    };

    msg = JSON.stringify(msgObj)
    this.wsClient.send(msg)
  }

  setUsername(username) {
    this.username = username;
  }

  getUsername() {
    return this.username;
  }

  setCurrentRoom(room) {
    this.currentRoom = room;
  }

  getCurrentRoom() {
    return this.currentRoom;
  }

  clearCurrentRoom() {
    this.currentRoom = null;
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
      console.log("Sending message: " + message)
      this.socket.send(message);
    } else {
      throw new Error("Unable to send message, WebSocket is not open");
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

        if (this.autoreconnect) {
          this._reconnect();
        }
      }

      if (this.socket.readyState === WebSocket.CLOSED) {
        this._reconnect();
      }
    }
  }
  onOpen(callback) {
    if (this.socket) {
      this.socket.onopen = (event) => {
        callback(event);
      }
    }
  }
}

const JOIN_ROOM_FORM_ID = 'join-room-form'

var goChatClient
var wsClient

if (window["WebSocket"]) {
  goChatClient = new GoChatClient(document.location.host);
  wsClient = new GoChatWSClient("ws://" + document.location.host + "/ws");
  wsClient.setUsername(localStorage.getItem("username"));

  wsClient.onPublishMessage = function (msg) {
    var msg = createMsg(msg);
    appendMessage(msg);
  }

  wsClient.onEventTypeUserPresent = function (msg) {
    setPresence(msg.user_id, true);
  }

  wsClient.onEventTypeUserAbsent = function (msg) {
    setPresence(msg.user_id, false);
  }

  wsClient.onEventTypeUserSubscribed = function (msg) {
    const subscribersList = document.querySelector('.subscribers-list');
    if (subscribersList) {
      const newSubscriberItem = createUserListItem(msg.user_id, msg.username);
      subscribersList.appendChild(newSubscriberItem);
    }
  }

  wsClient.onEventTypeUserUnsubscribed = function (msg) {
    const subscribersList = document.querySelector('.subscribers-list');
    if (subscribersList) {
      const subscriberItem = subscribersList.querySelector(`li[data-user-id="${msg.user_id}"]`);
      if (subscriberItem) {
        subscriberItem.remove();
      }
    }
  }

  // goChatClient.listSubscriptions().then(subs => {
  //   updateRoomList(subs);
  // }).catch(err => {
  //   console.error("Error fetching subscriptions:", err);
  //   document.getElementById('room-list').innerHTML = `
  //     <p class="error">Failed to load rooms.</p>
  //   `
  // });

  renderRoomsList();
} else {
  var item = document.createElement('div');
  item.innerHTML = "<b>Your browser does not support WebSockets.</b>";
  appendMessage(item);
}

function handleMessagesScroll() {
  const messages = document.getElementById('chat-area');
  if (messages.scrollTop === 0) {
    const roomId = wsClient.getCurrentRoom().external_id;
    const firstMessage = messages.firstElementChild;
    if (firstMessage) {
      const firstMessageSeqId = firstMessage.getAttribute('data-message-seq-id');
      if (firstMessageSeqId > 1) {
        const previousScrollHeight = messages.scrollHeight; // Save current scroll height
        goChatClient.getMessages(roomId, firstMessageSeqId)
          .then(newMessages => {
            for (let i = newMessages.length - 1; i >= 0; i--) {
              let msg = createMsg(newMessages[i]);
              messages.insertBefore(msg, firstMessage);
            }
            messages.scrollTop += messages.scrollHeight - previousScrollHeight; // Adjust scroll position
          })
          .catch(error => console.error('Error fetching messages:', error));
      }
    }
  }
}

function createRoomInfo(room) {
  const sideBarId = 'room-info-sidebar'
  const existingSidebar = document.getElementById(sideBarId)
  if (existingSidebar) {
    existingSidebar.remove()
  }

  const sideBar = document.createElement('div')
  sideBar.className = 'sidebar'
  sideBar.id = sideBarId
  const closeBtnId = 'close-btn'
  const roomIdCpBbtn = 'room-id-cp-btn'
  const roomIdTextClass = 'room-id'

  sideBar.innerHTML = `
    <div class="close-header">
      <button id="${closeBtnId}" class="icon-button" aria-label="Close">
        X
      </button>
    </div>
    <div class="room-info">
      <h3>Name</h3>
      <p>${room.name}</p>
      <h3>Description</h3>
      <p>${room.description}</p>
      <div>
        <span>ID: </span><span class="${roomIdTextClass}">${room.external_id}</span>
        <i id="${roomIdCpBbtn}" class="fa fa-copy icon-button"></i>
      </div>
    </div>
    <div class="subscribers">
      <h3>Subscribers</h3>
      <ul class="subscribers-list">
        ${room.subscribers.map(user => createUserListItem(user.id, user.username).outerHTML).join('')}
      </ul>
    </div>
  `

  sideBar.querySelector(`#${closeBtnId}`).onclick = event => {
    hideRoomInfoPanel()
  };

  sideBar.querySelector(`#${roomIdCpBbtn}`).onclick = event => {
    var text = sideBar.querySelector(`.${roomIdTextClass}`).innerHTML;
    navigator.clipboard.writeText(text);
    const prevColor = event.target.style.color;
    event.target.style.color = '#15d438';
    setTimeout(() => {
      event.target.style.color = prevColor;
    }, 1000);
  };

  sideBar.style.display = 'none';
  document.body.appendChild(sideBar)
}

function createUserListItem(userId, username) {
  const item = document.createElement('li');
  item.className = 'status-offline';
  item.setAttribute('data-user-id', userId);
  item.innerText = username;
  return item;
}

function showRoomInfoPanel(event) {
  if (!wsClient.getCurrentRoom()) {
    return
  }

  document.getElementById('room-opts-dropdown-content').style.display = 'none'
  document.getElementById('options-btn').style.display = 'none'
  document.getElementById('room-info-sidebar').style.display = 'block'
}

function hideRoomInfoPanel(event) {
  document.getElementById('room-info-sidebar').style.display = 'none'
  document.getElementById('options-btn').style.display = 'block'
}

function handleUnsubscribe(event) {
  let room = wsClientClient.getCurrentRoom();
  if (!room) {
    return
  }

  goChatClient.unsubscribeRoom(room.external_id).then(() => {
    removeRoomFromList(room.external_id);
    clearRoomView();
    wsClient.clearCurrentRoom();
  }).catch(err => {
    console.error("Error unsubscribing from room:", err);
  })
}

function handleDeleteRoom(event) {
  let yes = confirm("Are you sure you want to delete this room?");

  if (yes) {
    let room = wsClient.getCurrentRoom();
    goChatClient.deleteRoom(room.external_id).then(() => {
      removeRoomFromList(room.external_id);
      clearRoomView();
      wsClient.clearCurrentRoom();
    }).catch(err => {
      console.log(err)
    })
  }
}

function updateRoomList(rooms) {
  const roomList = document.getElementById('room-list');
  if (!roomList) {
    console.error("Room list not found");
    return;
  }

  if (!rooms || rooms.length === 0) {
    roomList.innerHTML = "<p class='no-rooms'>Join a Room to get started.</p>";
    return;
  }

  rooms.forEach(room => {
    roomList.appendChild(createRoomElement(room));
  });

  if (wsClient.currentRoom) {
    toggleRoomActive(wsClient.getCurrentRoom().external_id);
  }
}

function activateRoom(roomId) {
  console.log("Activating room: " + roomId)
  goChatClient.getRoom(roomId).then(room => {
    toggleRoomActive(room.external_id)
    renderNewRoom(room)
    switchRoom(room)
  }).catch(err => {
    console.log("Error fetching room:", err)
  })
}

function toggleRoomActive(roomId) {
  document.querySelectorAll(".active-room").forEach(el => el.classList.remove('active-room'));
  const roomDiv = document.getElementById(roomId);
  if (!roomDiv) {
    console.warn(`Room with ID ${roomId} not found in list.`);
    return;
  }
  roomDiv.classList.add('active-room');
}

function renderNewRoom(room) {
  chatContainer = document.querySelector('.chat-container')
  chatContainer.innerHTML = `
    <div class="chat-header">
      <h2 class="chat-title">${room.name}</h2>
      <div class="dropdown">
        <button id="options-btn" class="icon-button" aria-label="Options">
          <i class="fa fa-ellipsis-v"></i>
        </button>
        <div id="room-opts-dropdown-content" class="dropdown-content">
          <a id="roomDetailsBtn">Room Details</a>
          <a id="leaveRoomBtn" >Leave Room</a>
          <a id="deleteRoomBtn">Delete Room</a>
        </div>
      </div>
    </div>
    <div class="chat-area" id="chat-area">
    </div>
    <form class="chat-input" id="chat-input">
      <input type="text" placeholder="Type a message..." name="" id="msg" autofocus>
      <button type="submit">Send</button>
    </form>
  `

  document.getElementById('chat-area').addEventListener('scroll', handleMessagesScroll);
  document.getElementById('options-btn').onclick = function (event) {
    const dropdown = document.getElementById('room-opts-dropdown-content')
    dropdown.style.display = dropdown.style.display === 'block' ? 'none' : 'block'
  }
  document.getElementById('leaveRoomBtn').onclick = handleUnsubscribe
  document.getElementById('deleteRoomBtn').onclick = handleDeleteRoom
  document.getElementById('roomDetailsBtn').onclick = showRoomInfoPanel
  document.getElementById('chat-input').onsubmit = sendMessage

  goChatClient.getMessages(room.external_id).then(messages => {
    if (!messages || messages.length === 0) {
      return;
    }
    for (let i = messages.length - 1; i >= 0; i--) {
      appendMessage(createMsg(messages[i]))
    }
  }).catch(err => {
    console.error(err)
    document.getElementById('chat-area').innerHTML = `
      <div class="error">Failed to load messages</div>
    `
  })

  createRoomInfo(room)
}

function switchRoom(room) {
  if (wsClient.getCurrentRoom()) {
    wsClient.leaveRoom(wsClient.getCurrentRoom().id)
  }

  wsClient.joinRoom(room.id)
  wsClient.setCurrentRoom(room)
}

function appendMessage(item) {
  const messages = document.getElementById('chat-area');
  var doScroll = messages.scrollTop > messages.scrollHeight - messages.clientHeight - 1
  messages.appendChild(item)

  if (doScroll) {
    messages.scrollTop = messages.scrollHeight - messages.clientHeight
  }
}

function sendMessage(e) {
  e.preventDefault()

  const formMsg = document.getElementById("msg");
  if (!formMsg) {
    return false
  }

  if (!formMsg.value) {
    return false
  }

  const msg = formMsg.value
  formMsg.value = ""

  wsClient.sendMessage(msg)

  return true
}

function createRoomElement(room) {
  const roomDiv = document.createElement('div');
  roomDiv.innerHTML = `<div class="room" id="${room.external_id}">${room.name}</div>`;
  roomDiv.onclick = function (event) {
    const roomId = event.target.id;
    if (wsClient.currentRoom && roomId === wsClient.currentRoom.external_id) {
      return false;
    }

    activateRoom(roomId);
  }

  return roomDiv
}

function clearRoomView() {
  document.querySelector('.chat-container').innerHTML = `
    <div class="logo-container">
      <img class="logo" src="/static/logo.png" alt="go-chat" />
    <div>
  `
}

function setPresence(userId, on) {
  const subscribersList = document.querySelector('.subscribers-list');
  if (!subscribersList) {
    console.error("Subscribers list not found");
    return;
  }

  var subscriberItem = subscribersList.querySelector(`li[data-user-id="${userId}"]`);
  if (!subscriberItem) {
    console.error("Subscriber item not found");
    return;
  }

  subscriberItem.classList.remove('status-online', 'status-offline');

  if (on) {
    subscriberItem.classList.add('status-online');
  } else {
    subscriberItem.classList.add('status-offline');
  }
}

function createMsg(rawMsg) {
  const msgEl = document.createElement('div');
  msgEl.classList.add('chat-message');
  msgEl.setAttribute('data-message-id', rawMsg.id);
  msgEl.setAttribute('data-message-seq-id', rawMsg.seq_id);

  const user = wsClient.getCurrentRoom().subscribers.find(sub => sub.id === rawMsg.user_id);
  const username = user ? user.username : "Unknown";

  if (username === localStorage.getItem("username")) {
    msgEl.classList.add("user");
  }

  msgEl.innerHTML = `<div class="meta">${username} • ${formatTimestamp(rawMsg.timestamp)}</div>${rawMsg.content}`;

  return msgEl;
}

function formatTimestamp(timestamp) {
  return new Date(timestamp).toLocaleTimeString()
}

function renderAddRoom(event) {
  const sideBar = document.querySelector('.sidebar')
  sideBar.innerHTML = ""

  const headerEl = document.createDocumentFragment()
  let header = document.createElement('div')
  header.className = 'actions-header'

  let backBtn = document.createElement('i')
  backBtn.className = 'icon-button'
  backBtn.classList.add("fa", "fa-arrow-left")
  backBtn.onclick = event => {
    renderRoomsList()
  }

  let headerTitle = document.createElement('h2')
  headerTitle.innerText = "Add Chat Room"

  header.appendChild(backBtn)
  header.appendChild(headerTitle)
  headerEl.appendChild(header)

  const formEl = document.createDocumentFragment();
  const form = document.createElement('form')
  form.classList.add("sidebar-form")
  form.onsubmit = handleCreateRoom

  const nameLabel = document.createElement('label');
  nameLabel.setAttribute('for', 'name');
  nameLabel.textContent = 'Name:';
  const nameInput = document.createElement('input');
  nameInput.type = 'text';
  nameInput.id = 'name';
  nameInput.name = 'name';
  nameInput.required = true;

  const descLabel = document.createElement('label');
  descLabel.setAttribute('for', 'description');
  descLabel.textContent = 'Description:';
  const descInput = document.createElement('input');
  descInput.type = 'text';
  descInput.id = 'description';
  descInput.name = 'description';
  descInput.required = true;

  const submitBtn = document.createElement('input')
  submitBtn.type = 'submit'
  submitBtn.value = 'Submit'

  form.appendChild(nameLabel)
  form.appendChild(nameInput)
  form.appendChild(descLabel)
  form.appendChild(descInput)
  form.appendChild(submitBtn)
  formEl.appendChild(form)

  sideBar.appendChild(header)
  sideBar.appendChild(formEl)
}

async function handleCreateRoom(event) {
  event.preventDefault();

  const form = event.target;
  const existingError = form.querySelector('.error');
  if (existingError) {
    existingError.remove();
  }

  const roomName = form.querySelector('#name');
  const roomDesc = form.querySelector('#description');

  if (!roomName || !roomName.value || !roomDesc || !roomDesc.value) {
    const errorMessage = document.createElement('p');
    errorMessage.className = 'error';
    errorMessage.textContent = 'Please fill in all fields.';
    form.appendChild(errorMessage);
    return;
  }

  try {
    const room = await goChatClient.createRoom(roomName.value, roomDesc.value)
    renderRoomsList();
    renderNewRoom(room)
    switchRoom(room)
  } catch (err) {
    console.error(err)
    const errorMessage = document.createElement('p');
    errorMessage.className = 'error';
    errorMessage.textContent = 'Failed to create room. Please try again.';
    roomName.value = '';
    roomDesc.value = '';
    form.appendChild(errorMessage);
  }
}

function renderAccountEdit(event) {
  const sideBar = document.querySelector('.sidebar')
  header = `
    <div class="actions-header">
      <button id="close-btn" class="icon-button" aria-label="Close">
        <i class="fa fa-arrow-left"></i>
      </button>
      <h2>Account</h2>
    </div>
  `
  sideBar.innerHTML = `
    ${header}
    <p>Loading account information...</p>
  `

  goChatClient.getAccount().then(user => {
    const dropdown = document.getElementById('account-opts-dropdown-content')
    if (dropdown) {
      dropdown.style.display = 'none'
    }

    sideBar.innerHTML = `
      ${header}
      <div class="account-info">
        <h3>Email</h3>
        <p>${user.email_address}</p>
        <form id="update-acct-form" class="sidebar-form">
          <label for="username">Username</label>
          <input 
            type="text" 
            id="username" 
            name="username" 
            value="${user.username}" 
            aria-label="Username"
          >
          <label for="password">Password</label>
          <input 
            type="password" 
            id="passsword" 
            name="password" 
            placeholder="**********"
            required 
            autocomplete="on"
            aria-label="Password"
          >
          <input type="submit" value="Update"></input>
        </form>
      </div>
    `

    sideBar.querySelector('#update-acct-form').onsubmit = async function (event) {
      event.preventDefault()
      const formData = new FormData(event.target)
      try {
        const user = await goChatClient.updateAccount(formData.get('username'), formData.get('password'))
        if (user && user.username) { // Ensure user is valid
          localStorage.setItem("username", user.username)
          renderAccountEdit()
        } else {
          console.error("Failed to update account: Invalid user data")
        }
      } catch (err) {
        console.error("Error updating account:", err)
      }
    }
  }).catch(err => {
    sideBar.innerHTML = `
    ${header}
    <p class="error">Failed to load account info.</p>
    `
  }).finally(() => {
    sideBar.querySelector('#close-btn').addEventListener('click', function () {
      renderRoomsList()
    })
  })
}

async function renderRoomsList(component = '.sidebar') {
  const container = document.querySelector(component)
  if (!container) {
    console.log(`Container ${component} not found`)
    return
  }

  container.innerHTML = ""

  const username = localStorage.getItem("username")
  container.innerHTML = `
    <div class="sidebar-header">
      <h2>${username}</h2>
      <div class="menu-icons">
        <button id="add-room-btn" class="icon-button" aria-label="Add Room">
          <i class="fa fa-plus"></i>
        </button>
        <div class="dropdown">
          <button id="account-opts-btn" class="icon-button" aria-label="Settings">
            <i class="fa fa-gear"></i>
          </button>
          <div id="account-opts-dropdown-content" class="dropdown-content">
            <a id="account">Account</a>
            <a id="logoutBtn">Logout</a>
          </div>
        </div>
      </div>
    </div>
    <form id="${JOIN_ROOM_FORM_ID}" class="sidebar-form">
      <label for="roomId">Join Room</label>
      <input 
        type="text" 
        id="roomId" 
        name="id" 
        placeholder="Enter room ID" 
        required 
        aria-label="Join Room"
      >
      <input type="submit" value="Join"></input>
    </form>
    <div id="room-list" class="room-list">
      <p class="loading-text" id="loading-text">Loading rooms...</p>
    </div>
  `;

  const loadingText = document.getElementById('loading-text');
  try {
    const subs = await goChatClient.listSubscriptions();
    if (loadingText) {
      loadingText.remove();
    }
    updateRoomList(subs);
  } catch (err) {
    loadingText.remove();
    document.getElementById('room-list').innerHTML = `
      <p class="error">Failed to load rooms.</p>
    `
    console.error("Error fetching subscriptions:", err);
  }

  addRoomListEvtListeners()
}

function addRoomToList(room) {
  const roomList = document.getElementById('room-list')
  if (roomList) {
    roomList.appendChild(createRoomElement(room));
  }
}

function removeRoomFromList(roomId) {
  const roomDiv = document.getElementById(roomId);
  if (roomDiv) {
    roomDiv.remove();
  }
}

async function handleJoinRoom(event) {
  event.preventDefault();

  const form = event.target;
  const existingError = form.querySelector('.error');
  if (existingError) {
    existingError.remove();
  }

  const roomIdInput = form.querySelector('#roomId');

  if (!roomIdInput || !roomIdInput.value) {
    return;
  }

  const roomId = roomIdInput.value.trim();

  if (currentRoom && roomId === currentRoom.external_id) {
    // Already in the room
    roomIdInput.value = '';
    return;
  }

  const roomList = document.getElementById('room-list');
  if (roomList.querySelector(`#${roomId}`)) {
    // Room already exists in the list, just activate it
    activateRoom(roomIdInput.value);
    roomIdInput.value = '';
    return;
  }

  try {
    const sub = await goChatClient.subscribeRoom(roomIdInput.value);
    addRoomToList(sub.room);
    activateRoom(sub.room.external_id);
  } catch (err) {
    console.error("Error joining room:", err);
    const errorMessage = document.createElement('p');
    errorMessage.className = 'error';
    errorMessage.textContent = 'Failed to join room. Please check the ID and try again.';
    form.appendChild(errorMessage);
  }

  roomIdInput.value = '';
}

function addRoomListEvtListeners() {
  const addRoomBtn = document.getElementById('add-room-btn')
  if (addRoomBtn) {
    addRoomBtn.onclick = renderAddRoom
  }

  const accountOptBtn = document.getElementById('account-opts-btn')
  if (accountOptBtn) {
    accountOptBtn.onclick = function (event) {
      const dropdown = document.getElementById('account-opts-dropdown-content')
      dropdown.style.display = dropdown.style.display === 'block' ? 'none' : 'block'
    }
  }

  const joinRoomForm = document.getElementById(JOIN_ROOM_FORM_ID)
  if (joinRoomForm) {
    joinRoomForm.onsubmit = handleJoinRoom
  }

  const accountBtn = document.getElementById('account')
  if (accountBtn) {
    accountBtn.onclick = renderAccountEdit
  }

  const logoutBtn = document.getElementById('logoutBtn')
  if (logoutBtn) {
    logoutBtn.onclick = handleLogout
  }
}

handleLogout = function (event) {
  event.preventDefault();

  const logoutBtn = event.target;
  logoutBtn.disabled = true;

  goChatClient.logout().then(_ => {
    wsClient.clearCurrentRoom();
    wsClient.close();
    localStorage.removeItem("username");
    window.location.href = "/login";
  }).catch(err => {
    console.error("Error during logout:", err);
  }).finally(() => {
    logoutBtn.disabled = false;
    logoutBtn.textContent = "Logout";
  });
}
