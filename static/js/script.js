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

function handleRenderRoomDetails (event) {
  if (!currentRoom) {
    return
  }

  const sideBar = document.createElement('div')
  sideBar.className = 'sidebar'
  sideBar.innerHTML = `
    <div class="close-header">
      <button id="closeBtn" class="icon-button" aria-label="Close">
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

  sideBar.querySelector('#closeBtn').addEventListener('click', function () {
    sideBar.remove();
  });

  document.body.appendChild(sideBar)
}

function handleUnsubscribe(event) {
  if (!currentRoom) {
    return
  }

  unsubscribeRoom(currentRoom.id).then(() => {
    leaveRoom(currentRoom.id, false)
    updateRoomList()
    clearRoomView()
  }).catch(err => {
    console.log(err)
  })
}

function handleDeleteRoom (event) {
  let yes = confirm("Are you sure you want to delete this room?");

  if (yes) {
    deleteRoom(currentRoom.id).then(roomId => {
      updateRoomList()
      clearRoomView()
    })
  }
}

function setCurrentRoom(room) {
  console.log("Setting current room: " + JSON.stringify(room))
  currentRoom = room
}

function updateRoomList() {
  const roomList = document.getElementById('room-list')
  if (!roomList) {
    console.log("Room list not found")
    return
  }

  listSubscriptions().then(subs => {
    roomList.innerHTML = "";
    subs.forEach(sub => {
      roomList.appendChild(createRoomElement(sub));
    })
  }).catch(err => {
    console.log(err)
    roomList.innerHTML = `<p class="error">Failed to load chat rooms.</p>`
  })
}

async function unsubscribeRoom(roomId) {
  try {
    const response = await fetch("http://" + document.location.host + `/subscriptions?room_id=${roomId}`, {
      method: 'DELETE',
    })
    const res = await response.json()
    if (!response.ok) {
      throw new Error(res.error || "Couldn't unsubscribe from room")
    }
  } catch (err) {
    console.log(err)
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
    console.log(error)
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
  
  chatContainer = document.querySelector('.chat-container')
  chatContainer.innerHTML = `
    <div class="chat-header">
      <h2 class="chat-title">${room.name}</h2>
      <div class="dropdown">
        <i class="fa fa-ellipsis-v" style="font-size:1em"></i>
        <div class="dropdown-content">
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
  document.getElementById('leaveRoomBtn').onclick = handleUnsubscribe
  document.getElementById('deleteRoomBtn').onclick = handleDeleteRoom
  document.getElementById('roomDetailsBtn').onclick = handleRenderRoomDetails
  document.getElementById('chat-input').onsubmit = sendMessage

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

    addSub(sub.room); // Add the new subscription to the list
    updateRoomList(); // Refresh the UI
    return sub;
  } catch (err) {
    console.log(err)
  }
}

function createRoomElement(room) {
  const roomDiv = document.createElement('div');
  roomDiv.innerHTML = `<div class="room" id ="room-${room.id}">${room.name}</div>`;
  roomDiv.onclick = activateRoom

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

    return roomId
  } catch (err) {
    console.log(err)
  }
}

// function clearRoomView() {
//   document.getElementById('chat-area').innerHTML = "";
// }

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
          // removeRoom(renderedMessage.room_id)
          updateRoomList()
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
      // addSub(room);
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

  addRoomListEvtListeners()
  updateRoomList()
}

function handleJoinRoom(event) {
  event.preventDefault()
  const formData = new FormData(event.target)

  subscribeRoom(formData.get('id')).then(sub => {
    updateRoomList()
    switchRoom(sub.room.id)
    renderNewRoom(sub.room.id)
  }).catch(err => {
    console.log(err)
  })
  event.target.reset()
}

function addRoomListEvtListeners() {
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
    currentRoom = null
    localStorage.removeItem("username")
    window.location.href = "/login"
  }).catch(err => {
    console.log(err)
  })
}

renderRoomsList()
