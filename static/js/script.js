var conn
var currentRoom

const MESSAGES_PAGE_LIMIT = 10

const Status = {
  MessageTypeJoin: 0,
  MessageTypeLeave: 1,
  MessageTypePublish: 2,
  MessageTypeRoomDeleted: 3
};

function handleMessagesScroll() {
  const messages = document.getElementById('chat-area');
  if (messages.scrollTop === 0) {
    const roomId = currentRoom.id;
    const firstMessage = messages.firstElementChild;
    if (firstMessage) {
      const firstMessageSeqId = firstMessage.getAttribute('data-message-seq-id');
      if (firstMessageSeqId > 1) {
        const previousScrollHeight = messages.scrollHeight; // Save current scroll height
        getMessages(roomId, firstMessageSeqId)
          .then(newMessages => {
            for (let i = newMessages.length - 1; i >= 0; i--) {
              msg = createMsg(newMessages[i])
              messages.insertBefore(msg, firstMessage);
            }
            messages.scrollTop += messages.scrollHeight - previousScrollHeight; // Adjust scroll position
          })
          .catch(error => console.error('Error fetching messages:', error));
      }
    }
  }
}

function handleRenderRoomDetails(event) {
  if (!currentRoom) {
    return
  }

  document.getElementById('room-opts-dropdown-content').style.display = 'none'
  document.getElementById('options-btn').style.display = 'none'

  const sideBar = document.createElement('div')
  sideBar.className = 'sidebar'
  sideBar.innerHTML = `
    <div class="close-header">
      <button id="close-btn" class="icon-button" aria-label="Close">
        X
      </button>
    </div>
    <div class=room-info>
      <h3>Name</h3>
      <p>${currentRoom.name}</p>
      <h3>Description</h3>
      <p>${currentRoom.description}</p>
    </div>
    <div class="subscribers">
      <h3>Subscribers</h3>
      <ul class="subscribers-list">
        ${currentRoom.subscribers.map(sub => `<li>${sub.username}</li>`).join('')}
      </ul>
    </div>
  `

  sideBar.querySelector('#close-btn').addEventListener('click', function () {
    sideBar.remove();
    document.getElementById('options-btn').style.display = 'block';
  });

  document.body.appendChild(sideBar)
}

function handleUnsubscribe(event) {
  if (!currentRoom) {
    return
  }

  unsubscribeRoom(currentRoom.id).then(() => {
    leaveRoom(currentRoom.id)
    removeRoomFromList(currentRoom);
    clearRoomView();
    clearCurrentRoom();
  }).catch(err => {
    console.log(err)
  })
}

function handleDeleteRoom(event) {
  let yes = confirm("Are you sure you want to delete this room?");

  if (yes) {
    deleteRoom(currentRoom.id).then(() => {
      console.log(currentRoom)
      removeRoomFromList(currentRoom);
      clearRoomView();
    }).catch(err => {
      console.log(err)
    })
  }
}

function setCurrentRoom(room) {
  console.log("Setting current room: " + JSON.stringify(room))
  currentRoom = room
}

function clearCurrentRoom() {
  console.log("Clearing current room")
  currentRoom = null
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

  if (currentRoom) {
    toggleRoomActive(currentRoom.id);
  }
}

async function unsubscribeRoom(roomId) {
  try {
    const response = await fetch("http://" + document.location.host + `/subscriptions?room_id=${roomId}`, {
      method: 'DELETE',
    })
    if (!response.ok) {
      throw new Error(res.error || "Couldn't unsubscribe from room")
    }
  } catch (err) {
    console.log("Error unsubscribing from room:", err)
  }
}

async function listSubscriptions() {
  try {
    const response = await fetch("http://" + document.location.host + "/subscriptions", {
      method: 'GET',
      headers: { 'Content-type': 'application/json' },
    })

    const res = await response.json()
    if (!response.ok) {
      throw new Error(res.error || "Couldn't fetch rooms")
    }

    return res
  } catch (error) {
    throw new Error("Error fetching subscriptions: " + error)
  }
}

async function getRoom(roomId) {
  try {
    const response = await fetch("http://" + document.location.host + `/room?id=${roomId}`, {
      method: 'GET',
      headers: { 'Content-type': 'application/json' },
    })

    const data = await response.json();
    if (!response.ok) {
      throw new Error(data.error)
    }

    return data
  } catch (error) {
    console.log(error)
  }
}

function activateRoom(roomId) {
  console.log("Activating room: " + roomId)
  getRoom(roomId).then(room => {
    toggleRoomActive(room.id)
    renderNewRoom(room)
    switchRoom(room)
  }).catch(err => {
    console.log("Error fetching room:", err)
  })
}

function toggleRoomActive(roomId) {
  document.querySelectorAll(".active-room").forEach(el => el.classList.remove('active-room'));
  const roomDiv = document.getElementById(`room-${roomId}`);
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
          <a id="roomDetailsBtn" href="#">Room Details</a>
          <a href="#" id="leaveRoomBtn" >Leave Room</a>
          <a href="#" id="deleteRoomBtn">Delete Room</a>
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
  document.getElementById('roomDetailsBtn').onclick = handleRenderRoomDetails
  document.getElementById('chat-input').onsubmit = sendMessage

  getMessages(room.id).then(messages => {
    if (!messages || messages.length === 0) {
      return;
    }
    for (let i = messages.length - 1; i >= 0; i--) {
      appendMessage(createMsg(messages[i]))
    }
  }).catch(err => {
    const messages = document.getElementById('chat-area').innerHTML = `
      <div class="error">Failed to load messages.</div>
    `
  })
}

async function getMessages(roomId, before = 0) {
  const params = new URLSearchParams({
    room_id: roomId,
    limit: MESSAGES_PAGE_LIMIT,
  });
  if (before > 0) {
    params.append('before', before);
  }

  const url = `http://${document.location.host}/messages?${params.toString()}`;

  try {
    const response = await fetch(url, { method: 'GET' });
    if (!response.ok) {
      throw new Error(res.error);
    }

    return await response.json();
  } catch (err) {
    console.log(err);
  }
}

function switchRoom(room) {
  if (currentRoom) {
    leaveRoom(currentRoom.id, false)
  }

  joinRoom(room.id)
  setCurrentRoom(room)
}

function joinRoom(roomId) {
  var msgObj = {
    type: Status.MessageTypeJoin,
    room_id: roomId,
  };

  conn.send(JSON.stringify(msgObj))
}

function leaveRoom(roomId) {
  var msgObj = {
    type: Status.MessageTypeLeave,
    room_id: roomId,
  };

  conn.send(JSON.stringify(msgObj))
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

  if (!conn) {
    return false;
  }

  const formMsg = document.getElementById("msg");
  if (!formMsg) {
    return false
  }

  if (!formMsg.value) {
    return false
  }

  var msgObj = {
    type: Status.MessageTypePublish,
    room_id: currentRoom.id,
    content: formMsg.value
  };

  msg = JSON.stringify(msgObj)
  console.log("Sending message: " + msg)
  conn.send(msg)
  formMsg.value = ""

  return false
}

async function createRoom(name, description) {
  try {
    const response = await fetch("http://" + document.location.host + "/room/new", {
      method: 'POST',
      headers: { 'Content-type': 'application/json' },
      body: JSON.stringify({ name: name, description: description })
    })

    const room = await response.json()

    if (response.status !== 201) {
      throw new Error(room.error)
    }

    return room
  } catch (err) {
    console.log(err)
  }
}

async function subscribeRoom(roomId) {
  try {
    const response = await fetch("http://" + document.location.host + `/subscriptions?room_id=${roomId}`, {
      method: 'POST',
      headers: { 'Content-type': 'application/json' },
    })

    const sub = await response.json()

    if (response.status !== 201) {
      throw new Error(sub.error)
    }

    return sub;
  } catch (err) {
    console.log(err)
  }
}

function createRoomElement(room) {
  const roomDiv = document.createElement('div');
  roomDiv.innerHTML = `<div class="room" id ="room-${room.id}">${room.name}</div>`;
  roomDiv.onclick = function (event) {
    const roomId = event.target.id.replace("room-", "");
    if (currentRoom != null && roomId === currentRoom.id) {
      return false;
    }

    activateRoom(roomId);
  }

  return roomDiv
}

async function deleteRoom(roomId) {
  try {
    const response = await fetch("http://" + document.location.host + `/room/delete?id=${roomId}`, {
      method: 'GET',
    })

    if (response.status !== 204) {
      throw new Error(res.error)
    }
  } catch (err) {
    console.log(err)
  }
}

function clearRoomView() {
  document.querySelector('.chat-container').innerHTML = `
    <div class="logo-container">
      <img class="logo" src="/static/logo.png" alt="go-chat" />
    <div>
  `
}

if (window["WebSocket"]) {
  conn = new WebSocket("ws://" + document.location.host + "/ws");

  conn.onopen = function (event) {
    console.log("WebSocket connection opened");
  };

  conn.onclose = function (evt) {
    console.log("WebSocket connection closed")
  };

  conn.onmessage = function (evt) {
    var msgs = evt.data.split('\n');
    for (var i = 0; i < msgs.length; i++) {
      console.log("Received message: " + msgs[i])
      var renderedMessage = JSON.parse(msgs[i]);
      var msg

      switch (renderedMessage.type) {
        case Status.MessageTypeJoin:
        case Status.MessageTypeLeave:
          break
        case Status.MessageTypePublish:
          msg = createMsg(renderedMessage)
          appendMessage(msg);
          break
        case Status.MessageTypeRoomDeleted:
          if (currentRoom && currentRoom.id === renderedMessage.room_id) {
            removeRoomFromList(renderedMessage.room_id)
            clearRoomView();
            clearCurrentRoom();
          }
        default:
      }
    }
  };

  renderRoomsList();
} else {
  var item = document.createElement('div');
  item.innerHTML = "<b>Your browser does not support WebSockets.</b>";
  appendMessage(item);
}

function createMsg(rawMsg) {
  const msgEl = document.createElement('div');
  msgEl.classList.add('chat-message');
  msgEl.setAttribute('data-message-id', rawMsg.id);
  msgEl.setAttribute('data-message-seq-id', rawMsg.seq_id);

  const metaEl = document.createElement('div');
  metaEl.classList.add('meta');

  // Map user ID to username
  const user = currentRoom.subscribers.find(sub => sub.id === rawMsg.user_id);
  const username = user ? user.username : "Unknown";

  metaEl.textContent = `${username} • ${formatTimestamp(rawMsg.timestamp)}`;

  const contentText = document.createTextNode(rawMsg.content);

  msgEl.appendChild(metaEl);
  msgEl.appendChild(contentText);

  if (username === localStorage.getItem("username")) {
    msgEl.classList.add("user");
  }

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
  form.onsubmit = event => {
    event.preventDefault()
    const formData = new FormData(event.target)
    const name = formData.get('name');
    const description = formData.get('description');

    createRoom(name, description).then(room => {
      switchRoom(room)
      renderRoomsList();
      renderNewRoom(room)
    })
  }

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

async function getAccount() {
  try {
    const response = await fetch("http://" + document.location.host + "/account", { method: 'GET' })

    if (!response.ok) {
      throw new Error(res.error || "Couldn't get account info.")
    }

    return await response.json()
  } catch (err) {
    console.log(err)
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

  getAccount().then(user => {
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
        const user = await handleUpdateAccount(formData.get('username'), formData.get('password'))
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

async function handleUpdateAccount(username, password) {
  try {
    const response = await fetch("http://" + document.location.host + "/account", {
      method: 'PUT',
      headers: { 'Content-type': 'application/json' },
      body: JSON.stringify({ username: username, password: password })
    })

    const resp = await response.json()
    if (!response.ok) {
      throw new Error(resp.error || "Couldn't update account")
    }

    return resp
  } catch (err) {
    console.log(err)
  }
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
    <form id="joinRoomForm" class="sidebar-form">
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

  try {
    const subs = await listSubscriptions()
    updateRoomList(subs);
    const loadingText = document.getElementById('loading-text');
    if (loadingText) {
      loadingText.remove();
    }

    addRoomListEvtListeners()

    if (currentRoom) {
      toggleRoomActive(currentRoom.id);
    }
  } catch (err) {
    console.log(err);
  }
}

function addRoomToList(room) {
  const roomList = document.getElementById('room-list')
  if (roomList) {
    roomList.appendChild(createRoomElement(room));
  }
}

function removeRoomFromList(room) {
  const roomDiv = document.getElementById(`room-${room.id}`);
  if (roomDiv) {
    roomDiv.remove();
  }
}

function handleJoinRoom(event) {
  event.preventDefault();
  const formData = new FormData(event.target);

  subscribeRoom(formData.get('id')).then(sub => {
    addRoomToList(sub.room);
    activateRoom(sub.room.id);
  }).catch(err => {
    console.log(err);
  });

  // Clear the input field
  const roomIdInput = event.target.querySelector('#roomId');
  if (roomIdInput) {
    roomIdInput.value = '';
  }
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

  const joinRoomForm = document.getElementById('joinRoomForm')
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

  fetch("http://" + document.location.host + "/logout", {
    method: 'GET',
  }).then(res => {
    if (!res.ok) {
      throw new Error(res.statusText || "Logout failed");
    }

    conn.close();
    clearCurrentRoom();
    localStorage.removeItem("username");
    window.location.href = "/login";
  }).catch(err => {
    console.error("Error during logout:", err);
    alert("Failed to log out. Please try again.");
  }).finally(() => {
    logoutBtn.disabled = false;
    logoutBtn.textContent = "Logout";
  });
}
