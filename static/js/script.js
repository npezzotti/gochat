var conn
var formMsg = document.getElementById("msg");
const messages = document.getElementById('chat-area');
const chatContainer = document.getElementById('chat-container');
const roomList = document.getElementById('room-list')
var currentRoom

document.getElementById('leaveRoomBtn').onclick = function (event) {
  leaveRoom(currentRoom, true);
  removeRoom(currentRoom)
  clearRoomView()
}

document.getElementById('deleteRoomBtn').onclick = function (event) {
  let result = confirm("Are you sure you want to delete this room?");

  if (result) {
    deleteRoom(currentRoom)
  }
}

const Status = {
  MessageTypeJoin: 0,
  MessageTypeLeave: 1,
  MessageTypePublish: 2,
  MessageTypeRoomDeleted: 3
};

async function refreshRooms() {
  try {
    const response = await fetch("http://" + document.location.host + "/rooms", {
      method: 'GET',
      headers: { 'Content-type': 'application/json' },
    })

    const res = await response.json()
    if (response.status !== 200) {
      throw new Error(res.error || "Login failed")
    }

    roomList.innerHTML = ""

    res.forEach(room => {
      addRoom(room)
    })
  } catch (error) {
    console.log(error)
  }
}

function activateRoom(event) {
  var roomId = parseInt(event.target.id.replace("room-", ""))
  if (roomId === currentRoom) {
    return false
  }

  switchRoom(roomId)
  renderNewRoom(roomId)
}

function toggleRoomActive(roomId) {
  document.querySelectorAll(".active-room").forEach(el => el.classList.remove('active-room'));
  let roomDiv = roomList.querySelector(`#room-${roomId}`)
  roomDiv.classList.add('active-room');
}

function renderNewRoom(roomId) {
  toggleRoomActive(roomId)
  clearRoomView()
}

function switchRoom(roomId) {
  if (currentRoom != null) {
    leaveRoom(currentRoom, false)
  }

  joinRoom(roomId)
}

function joinRoom(roomId) {
  var msgObj = {
    type: Status.MessageTypeJoin,
    room_id: roomId,
  };

  conn.send(JSON.stringify(msgObj))
  currentRoom = roomId
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
  if (!conn) {
    return false;
  }

  if (!formMsg.value) {
    return false
  }

  var msgObj = {
    type: Status.MessageTypePublish,
    room_id: currentRoom,
    content: formMsg.value
  };

  conn.send(JSON.stringify(msgObj))
  formMsg.value = ""

  return false
}

async function createRoom() {
  try {
    const response = await fetch("http://" + document.location.host + "/room/new", {
      method: 'POST',
      headers: { 'Content-type': 'application/json' },
      body: JSON.stringify({ name: "example", description: "example description" })
    })

    let room = await response.json()
    if (response.status !== 201) {
      throw new Error(room.error)
    }

    addRoom(room)
    switchRoom(room.id)
    renderNewRoom(room.id)
  } catch (err) {
    console.log(err)
  }
}

function removeRoom(roomId) {
  let roomDiv = roomList.querySelector(`#room-${roomId}`)
  if (roomDiv) {
    roomDiv.remove()
  }
}

function addRoom(room) {
  const roomDiv = document.createElement('div');
  roomDiv.classList.add('room')
  roomDiv.id = `room-${room.id}`
  roomDiv.textContent = room.name
  roomDiv.onclick = activateRoom
  roomList.appendChild(roomDiv)
}

async function deleteRoom(id) {
  try {
    const response = await fetch("http://" + document.location.host + `/room/delete?id=${id}`, {
      method: 'GET',
    })

    if (response.status !== 204) {
      throw new Error(res.error)
    }

    removeRoom(currentRoom)
    clearRoomView()
  } catch (err) {
    console.log(err)
  }
}

function clearRoomView() {
  messages.innerHTML = "";
}

refreshRooms()

if (window["WebSocket"]) {
  conn = new WebSocket("ws://" + document.location.host + "/ws");

  conn.onopen = function (event) {
    console.log("WebSocket connection opened");

    setTimeout(() => {
      createRoom()
    }, 3000)
  };

  conn.onclose = function (evt) {
    console.log("WebSocket connection closed")
  };

  conn.onmessage = function (evt) {
    var msgs = evt.data.split('\n');
    for (var i = 0; i < msgs.length; i++) {
      var renderedMessage = JSON.parse(msgs[i]);
      console.log(renderedMessage)
      const msg = document.createElement('div');

      switch (renderedMessage.type) {
        case Status.MessageTypeJoin:
        case Status.MessageTypeLeave:
          msg.textContent = renderedMessage.content;
          break
        case Status.MessageTypePublish:
          msg.textContent = `${renderedMessage.from}: ${renderedMessage.content}`;
          if (renderedMessage.from === localStorage.getItem("username")) {
            msg.classList.add("user")
          }
          break
        case Status.MessageTypeRoomDeleted:
          removeRoom(renderedMessage.room_id)

          if (currentRoom === renderedMessage.room_id) {
            clearRoomView()
            currentRoom = null
          }
        default:
      }

      msg.classList.add('chat-message');
      appendMessage(msg);
    }
  };
} else {
  var item = document.createElement('div');
  item.innerHTML = "<b>Your browser does not support WebSockets.</b>";
  appendMessage(item);
}

// Side panels

document.getElementById('addRoomBtn').onclick = renderAddRoom

function renderAddRoom (event) {
  const sideBar = document.querySelector('.sidebar')
  sideBar.innerHTML = ""

  let header = document.createElement('div')
  header.className = 'sidebar-header'

  let backBtn = document.createElement('i')
  backBtn.classList.add("fa", "fa-arrow-left")
  backBtn.onclick = renderRoomsList

  let headerTitle = document.createElement('h2')
  headerTitle.innerText = "Add Chat Room"
  
  header.appendChild(backBtn)
  header.appendChild(headerTitle)

  sideBar.appendChild(header)
}

function renderRoomsList() {
  const sideBar = document.querySelector('.sidebar')
  sideBar.innerHTML = ""

  let header = document.createElement('div')
  header.className = 'sidebar-header'

  let headerTitle = document.createElement('h2')
  headerTitle.textContent = localStorage.getItem("username")

  let menuIcons = document.createElement('div')
  menuIcons.className = 'menu-icons'

  let addRoomBtn = document.createElement('i')
  addRoomBtn.id = "addRoomBtn"
  addRoomBtn.classList.add("fa", "fa-plus")
  addRoomBtn.onclick = renderAddRoom

  let dropdown = document.createElement('div')
  dropdown.className = ('dropdown')
  dropdown.innerHTML =  `
  <i class="fa fa-gear"></i>
  <div class="dropdown-content">
    <a id="account" href="/account/edit">Account</a>
    <a id="logout-btn">Logout</a>
  </div>
  `

  menuIcons.appendChild(addRoomBtn)
  menuIcons.appendChild(dropdown)
  
  header.appendChild(headerTitle)
  header.appendChild(menuIcons)

  sideBar.appendChild(header)
  sideBar.appendChild(roomList)
  
  refreshRooms()
}

renderRoomsList()
