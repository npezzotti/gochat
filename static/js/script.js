var conn
var formMsg = document.getElementById("msg");
const messages = document.getElementById('chat-area');

var currentRoom = null
const rooms = document.querySelectorAll('div.room')
rooms.forEach(room => {
  room.onclick = function(event) {
    roomId = parseInt(event.target.id)
    if (roomId === currentRoom) {
      return false
    }

    document.querySelectorAll(".active-room").forEach(el => el.classList.remove('active-room')); 

    event.target.classList.add('active-room');

    if (currentRoom != null) {
      leaveRoom(currentRoom)
    }

    messages.replaceChildren();
    joinRoom(roomId)
  }
})

const Status = {
  MessageTypeJoin: 0,
  MessageTypeLeave: 1,
  MessageTypePublish: 2
};

function joinRoom(roomId) {
  var msgObj = {
    type: Status.MessageTypeJoin,
    room_id: roomId,
  };

  conn.send(JSON.stringify(msgObj))
  currentRoom = roomId
}

function leaveRoom(roomId) {
  if (currentRoom != null) {
    var msgObj = {
      type: Status.MessageTypeLeave,
      room_id: roomId,
    };

    conn.send(JSON.stringify(msgObj))
  }
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

if (window["WebSocket"]) {
  conn = new WebSocket("ws://" + document.location.host + "/ws");

  conn.onopen = function (event) {
    console.log("WebSocket connection opened!");
  };

  conn.onclose = function (evt) {
    console.log("connection closed")
  };

  conn.onmessage = function (evt) {
    var msgs = evt.data.split('\n');
    for (var i = 0; i < msgs.length; i++) {
      var renderedMessage = JSON.parse(msgs[i]);
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
