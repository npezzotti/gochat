var conn
var formMsg = document.getElementById("msg");
const messages = document.getElementById('chat-area');

const Status = {
  MessageTypeJoin: 0,
  MessageTypeLeave: 1,
  MessageTypePublish: 2
};

function appendMessage(item) {
  var doScroll = messages.scrollTop > messages.scrollHeight - messages.clientHeight - 1
  messages.appendChild(item)

  if (doScroll) {
    messages.scrollTop = messages.scrollHeight - messages.clientHeight
  }
}

document.getElementById("chat-input").onsubmit = sendMessage

function sendMessage (e) {
  if (!conn) {
    return false;
  }

  if (!formMsg.value) {
    return false
  }

  var msgObj = {
    type: 2,
    content: formMsg.value
  };

  conn.send(JSON.stringify(msgObj))
  formMsg.value = ""

  return false
}

if (window["WebSocket"]) {
  conn = new WebSocket("ws://" + document.location.host + "/ws");

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
