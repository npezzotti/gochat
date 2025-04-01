var conn
var currentRoom
var subscriptions = []

const MESSAGES_PAGE_LIMIT = 10

var formMsg = document.getElementById("msg");
const messages = document.getElementById('chat-area');

messages.addEventListener('scroll', handleScroll);
function handleScroll() {
  if (messages.scrollTop === 0) {
    const roomId = currentRoom.id;
    var firstMessage = messages.firstChild;
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

document.getElementById('leaveRoomBtn').onclick = function (event) {
  leaveRoom(currentRoom.id, true);
  removeRoom(currentRoom.id)
  updateRoomList()
  clearRoomView()
}

document.getElementById('deleteRoomBtn').onclick = function (event) {
  let result = confirm("Are you sure you want to delete this room?");

  if (result) {
    deleteRoom(currentRoom.id).then(roomId => {
      removeRoom(roomId)
      updateRoomList()
      clearRoomView()
    })
  }
}

document.getElementById('roomDetailsBtn').onclick = function (event) {
  console.log("fired")
  const sideBar = document.createElement('div')
  sideBar.className = 'sidebar'

  const closeHeader = document.createElement('div')
  closeHeader.className = 'close-header'

  const closeBtn = document.createElement('button')
  closeBtn.className = 'icon-button'
  closeBtn.innerText = 'X'
  closeBtn.onclick = function () {
    sideBar.remove()
  }
  closeHeader.appendChild(closeBtn)
  sideBar.appendChild(closeHeader)

  roomInfo = document.createElement('div')
  roomInfo.className = 'room-info'
  const roomNameHeader = document.createElement('h3')
  roomNameHeader.innerText = "Name"
  roomInfo.appendChild(roomNameHeader)
  const roomNameBody = document.createElement('p')
  roomNameBody.innerText = currentRoom.name
  roomInfo.appendChild(roomNameBody)
  const roomDescHeader = document.createElement('h3')
  roomDescHeader.innerText = "Description"
  roomInfo.appendChild(roomDescHeader)
  const roomDescBody = document.createElement('p')
  roomDescBody.innerText = currentRoom.description
  roomInfo.appendChild(roomDescBody)
  sideBar.appendChild(roomInfo)

  subscribersContainer = document.createElement('div')
  subscribersContainer.className = 'subscribers'
  const subscribersHeader = document.createElement('h3')
  subscribersHeader.innerText = "Subscribers"
  subscribersContainer.appendChild(subscribersHeader)

  const subscribersList = document.createElement('ul')
  subscribersList.className = 'subscribers-list'
  currentRoom.subscribers.forEach(sub => {
    const li = document.createElement('li')
    li.innerText = sub.username
    subscribersList.appendChild(li)
  })
  subscribersContainer.appendChild(subscribersList)
  sideBar.appendChild(subscribersContainer)

  document.body.appendChild(sideBar)
}

const Status = {
  MessageTypeJoin: 0,
  MessageTypeLeave: 1,
  MessageTypePublish: 2,
  MessageTypeRoomDeleted: 3
};

function setCurrentRoom(room) {
  console.log("Setting current room: " + JSON.stringify(room))
  currentRoom = room
}

function updateRoomList() {
  document.getElementById('room-list').innerHTML = "";
  if (subscriptions && subscriptions.length > 0) {
    subscriptions.forEach(room => {
      createRoomElement(room);
    });
  };
}

async function refreshRooms() {
  try {
    const response = await fetch("http://" + document.location.host + "/subscriptions", {
      method: 'GET',
      headers: { 'Content-type': 'application/json' },
    })

    const res = await response.json()
    if (response.status !== 200) {
      throw new Error(res.error || "Login failed")
    }

    res.forEach(room => {
      addRoom(room)
    })
  } catch (error) {
    console.log(error)
  }
}

async function getRoom(roomId) {
  try {
    const response = await fetch("http://" + document.location.host + `/room?id=${roomId}`, {
      method: 'GET',
      headers: { 'Content-type': 'application/json' },
    })

    if (!response.ok) {
      throw new Error(res.error)
    }

    const data = await response.json();
    return data
  } catch (error) {
    console.log(error)
  }
}

function activateRoom(event) {
  var roomId = event.target.id.replace("room-", "")
  if (currentRoom != null && roomId === currentRoom.id) {
    return false
  }

  getRoom(roomId).then(room => {
    switchRoom(room.id)
    setCurrentRoom(room)
    renderNewRoom(room)
  });
}

function toggleRoomActive(roomId) {
  const roomList = document.getElementById('room-list')
  document.querySelectorAll(".active-room").forEach(el => el.classList.remove('active-room'));
  let roomDiv = roomList.querySelector(`#room-${roomId}`)
  roomDiv.classList.add('active-room');
}

function renderNewRoom(room) {
  toggleRoomActive(room.id)
  clearRoomView()
  updateTitle(room)
  getMessages(room.id).then(messages => {
    if (!messages || messages.length === 0) {
      return;
    }
    for (let i = messages.length - 1; i >= 0; i--) {
      msg = createMsg(messages[i])
      appendMessage(msg)
    }
  })
}

function updateTitle(room) {
  document.querySelector('.chat-title').innerText = room.name
}

async function getMessages(roomId, before = 0) {
  let url = "http://" + document.location.host + `/messages?room_id=${roomId}&limit=${MESSAGES_PAGE_LIMIT}`
  if (before > 0) {
    url += `&before=${before}`
  }

  try {
    const response = await fetch(url, { method: 'GET' })
    if (!response.ok) {
      throw new Error(res.error)
    }

    return await response.json()
  } catch (err) {
    console.log(err)
  }
}

function switchRoom(roomId) {
  if (currentRoom) {
    leaveRoom(currentRoom.id, false)
  }

  joinRoom(roomId)
}

function joinRoom(roomId) {
  var msgObj = {
    type: Status.MessageTypeJoin,
    room_id: roomId,
  };

  conn.send(JSON.stringify(msgObj))
}

function leaveRoom(roomId, unsub) {
  var msgObj = {
    type: Status.MessageTypeLeave,
    room_id: roomId,
    unsub: unsub
  };

  conn.send(JSON.stringify(msgObj))
}

function appendMessage(item) {
  var doScroll = messages.scrollTop > messages.scrollHeight - messages.clientHeight - 1
  messages.appendChild(item)

  if (doScroll) {
    messages.scrollTop = messages.scrollHeight - messages.clientHeight
  }
}

document.getElementById("chat-input").onsubmit = sendMessage

function sendMessage(e) {
  e.preventDefault()

  if (!conn) {
    return false;
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

    addRoom(sub.room); // Add the new subscription to the list
    updateRoomList(); // Refresh the UI
    return sub;
  } catch (err) {
    console.log(err)
  }
}

function addRoom(room) {
  subscriptions.push(room)
}

function removeRoom(roomId) {
  subscriptions = subscriptions.filter(room => room.id !== roomId)
}

function createRoomElement(room) {
  const roomDiv = document.createElement('div');
  roomDiv.classList.add('room')
  roomDiv.id = `room-${room.id}`
  roomDiv.textContent = room.name
  roomDiv.onclick = activateRoom
  document.getElementById('room-list').appendChild(roomDiv)
}

async function deleteRoom(roomId) {
  try {
    const response = await fetch("http://" + document.location.host + `/room/delete?id=${roomId}`, {
      method: 'GET',
    })

    if (response.status !== 204) {
      throw new Error(res.error)
    }

    return roomId
  } catch (err) {
    console.log(err)
  }
}

function clearRoomView() {
  messages.innerHTML = "";
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
          removeRoom(renderedMessage.room_id)
          clearRoomView()
          currentRoom = null
        default:
      }
    }
  };
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

// Side panels

document.getElementById('addRoomBtn').onclick = renderAddRoom

function renderAddRoom(event) {
  const sideBar = document.querySelector('.sidebar')
  sideBar.innerHTML = ""

  const headerEl = document.createDocumentFragment()
  let header = document.createElement('div')
  header.className = 'sidebar-header'

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
  form.onsubmit = event => {
    event.preventDefault()
    const formData = new FormData(event.target)
    const name = formData.get('name');
    const description = formData.get('description');

    createRoom(name, description).then(room => {
      console.log(room)
      addRoom(room);
      renderRoomsList();
      switchRoom(room.id)
      setCurrentRoom(room)
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

function renderRoomsList(component = '.sidebar') {
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
        <button id="addRoomBtn" class="icon-button" aria-label="Add Room">
          <i class="fa fa-plus"></i>
        </button>
        <div class="dropdown">
          <button class="icon-button" aria-label="Settings">
            <i class="fa fa-gear"></i>
          </button>
          <div class="dropdown-content">
            <a id="account" href="/account/edit">Account</a>
            <a id="logoutBtn">Logout</a>
          </div>
        </div>
      </div>
    </div>
    
    <form id="joinRoomForm" class="join-room-form">
      <label for="roomId">Room ID</label>
      <input 
        type="text" 
        id="roomId" 
        name="id" 
        placeholder="Enter room ID" 
        required 
        aria-label="Room ID"
      >
      <button type="submit">Join</button>
    </form>
    
    <div id="room-list" class="room-list">
      <p class="loading-text">Loading rooms...</p>
    </div>
  `;

  addEventListeners()
  updateRoomList()
}

function handleJoinRoom(event) {
  event.preventDefault()
  const formData = new FormData(event.target)

  subscribeRoom(formData.get('id')).then(sub => {
    switchRoom(sub.room.id)
    renderNewRoom(sub.room.id)
  }).catch(err => {
    console.log(err)
  })
  event.target.reset()
}

function addEventListeners() {
  const addRoomBtn = document.getElementById('addRoomBtn')
  if (addRoomBtn) {
    addRoomBtn.onclick = renderAddRoom
  }

  const joinRoomForm = document.getElementById('joinRoomForm')
  if (joinRoomForm) {
    joinRoomForm.onsubmit = handleJoinRoom
  }

  const logoutBtn = document.getElementById('logoutBtn')
  if (logoutBtn) {
    logoutBtn.onclick = handleLogout
  }
}

handleLogout = function (event) {
  event.preventDefault()
  fetch("http://" + document.location.host + "/logout", {
    method: 'GET',
  }).then(res => {
    if (!res.ok) {
      throw new Error(res.error)
    }

    conn.close()
    subscriptions = []
    currentRoom = null
    localStorage.removeItem("username")
    window.location.href = "/login"
  }).catch(err => {
    console.log(err)
  })
}

refreshRooms().then(() => {
  renderRoomsList()
})
