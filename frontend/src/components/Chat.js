import { use, useEffect, useRef, useState } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faEllipsisVertical } from '@fortawesome/free-solid-svg-icons'

import '../App.css';
import ChatMessage from './ChatMessage';
import RoomInfoPanel from './RoomInfoPanel';
import goChatClient from '../gochat';

export default function Chat({ currentUser, currentRoom, setCurrentRoom, rooms, setRooms, messages, setMessages, wsClient }) {
  const [roomInfoPanelVisible, setRoomInfoPanelVisible] = useState(false);
  const [showDropdownContent, setShowDropdownContent] = useState(false);
  const [loadingMsgs, setLoadingMsgs] = useState(true);
  const [before, setBefore] = useState(0);
  const [hasMore, setHasMore] = useState(true);
  const [prevHeight, setPrevHeight] = useState(null);
  const [message, setMessage] = useState('');
  const chatWindow = useRef();

  const fetchMessages = async () => {
    if (!hasMore) {
      return;
    }
    setLoadingMsgs(true);
    goChatClient.getMessages(currentRoom.external_id, before)
      .then(msgs => {
        if (!msgs) {
          setLoadingMsgs(false);
          setHasMore(false);
          return;
        }
        setMessages([...msgs.reverse(), ...messages]);
        setBefore(msgs[0].seq_id);
        setLoadingMsgs(false);
      })
      .catch(err => {
        console.error('Error fetching messages:', err);
      });
  }

  useEffect(() => {
    // Clear messages immediately when currentRoom changes
    setMessages([]);
    setBefore(0);
    setPrevHeight(null);
    setHasMore(true);
    setMessage('');

    if (currentRoom) {
      setLoadingMsgs(true);
      goChatClient.getMessages(currentRoom.external_id, 0)
        .then(msgs => {
          if (!msgs) {
            setLoadingMsgs(false);
            setHasMore(false);
            return;
          }
          setMessages([...msgs.reverse()]);
          setBefore(msgs[0].seq_id);
          setLoadingMsgs(false);
        });
    }
  }, [currentRoom]);

  useEffect(() => {
    if (!loadingMsgs) {
      const chatArea = chatWindow.current;
      if (prevHeight !== null) {
        chatArea.scrollTop = chatArea.scrollHeight - prevHeight;
      } else {
        chatArea.scrollTop = chatArea.scrollHeight;
      }
    }
  });

  const toggleRoomInfoPanel = () => {
    if (!roomInfoPanelVisible) {
      setShowDropdownContent(false);
    }

    setRoomInfoPanelVisible(!roomInfoPanelVisible);
  }

  const toggleDropdownContent = () => {
    setShowDropdownContent(!showDropdownContent);
  }

  const onScroll = (e) => {
    const { scrollTop } = e.target;

    if (scrollTop === 0) {
      const { scrollHeight } = chatWindow.current;
      setPrevHeight(scrollHeight);

      fetchMessages();
    }
  }

  const handleLeaveRoom = () => {
    wsClient.leaveRoom(currentRoom.external_id, true)
    .then(_ => {
      setCurrentRoom(null)
      setMessages([]);
      setBefore(0);
      setRooms(rooms.filter(room => room.external_id !== currentRoom.external_id));
    })
    .catch(err => {
      console.log("Failed to leave room: " + err);
    });
  }

  const handleDeleteRoom = () => {
    goChatClient.deleteRoom(currentRoom.external_id)
      .then(() => {
        setCurrentRoom(null);
        setMessages([]);
        setBefore(0);
        setRooms(rooms.filter(room => room.external_id !== currentRoom.external_id));
      })
      .catch(err => {
        console.error('Error deleting room:', err)
      });
  }

  const handleMessageChange = e => {
    setMessage(e.target.value);
  }
  
  const sendMessage = e => {
    e.preventDefault();
    if (message.trim() === '') {
      return;
    }

    wsClient.publishMessage(currentRoom.external_id, message)
      .then(() => {
        setMessage('');
      })
      .catch(err => {
        console.error('Error sending message:', err);
      });
  }

  return (
    <>
      <div className="chat-container">
        <div className="chat-header">
          <h2 className="chat-title">{currentRoom && currentRoom.name}</h2>
          <div className="dropdown">
            <button id="options-btn" className="icon-button" aria-label="Options" onClick={toggleDropdownContent}>
              <FontAwesomeIcon icon={faEllipsisVertical} style={{ display: roomInfoPanelVisible ? 'none' : 'block' }} />
            </button>
            <div id="room-opts-dropdown-content" className="dropdown-content" style={{ display: showDropdownContent ? 'block' : 'none' }}>
              <a id="room-details-btn" onClick={toggleRoomInfoPanel}>Room Details</a>
              <a id="leave-rm-btn" onClick={handleLeaveRoom}>Leave Room</a>
              <a id="delete-rm-btn" onClick={handleDeleteRoom}>Delete Room</a>
            </div>
          </div>
        </div>
        <>
          {loadingMsgs ?
            <div className='chat-area'>Loading...</div> :
            <div className="chat-area" id="chat-area" ref={chatWindow} onScroll={onScroll}>
              {messages.map((msg) => {
                return <ChatMessage key={msg.seq_id} message={msg} currentUser={currentUser} currentRoom={currentRoom} />
              })}
            </div>
          }
          <form className="chat-input" id="chat-input" onSubmit={sendMessage}>
            <input type="text" placeholder="Type a message..." autoFocus="" value={message} onChange={handleMessageChange} />
            <button type="submit">Send</button>
          </form>
        </>
      </div>
      <RoomInfoPanel visible={roomInfoPanelVisible} hideRoomInfoPanel={toggleRoomInfoPanel} currentRoom={currentRoom} />
    </>
  );
}
