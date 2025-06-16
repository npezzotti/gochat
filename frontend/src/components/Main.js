import { useEffect, useState, useRef } from 'react';

import Logo from './Logo';
import GoChatWSClient from '../gochatws';

import '../App.css';
import Sidebar from './Sidebar';
import Chat from './Chat';
import goChatClient from '../gochat';

export default function Main({ currentUser, setCurrentUser }) {
  const [wsClient, setWsClient] = useState(null);
  const [currentRoom, setCurrentRoom] = useState(null);
  const [messages, setMessages] = useState([]);
  const [rooms, setRooms] = useState([]);
  const currentRoomRef = useRef(null);

  function handleUserPresenceEvent(user_id, present) {
    const updatedSubscribers = currentRoomRef.current?.subscribers.map(subscriber =>
      subscriber.user_id === user_id ? { ...subscriber, isPresent: present } : subscriber
    );

    if (updatedSubscribers) {
      setCurrentRoom({ ...currentRoomRef.current, subscribers: updatedSubscribers });
    }
  }

  function handleRoomPresenceEvent(room_id, present) {
    setRooms(rooms =>
      rooms.map(room =>
        room.external_id === room_id ? { ...room, isOnline: present } : room
      )
    );
  }

  function addSubscriber(user) {
    const updatedSubscribers = currentRoomRef.current.subscribers
    updatedSubscribers.push({
      user_id: user.id,
      username: user.username,
    });

    setCurrentRoom({ ...currentRoomRef.current, subscribers: updatedSubscribers });
  }

  function removeSubscriber(userId) {
    currentRoomRef.current.subscribers = currentRoomRef.current.subscribers.filter(
      subscriber => subscriber.user_id !== userId
    );
    setCurrentRoom({ ...currentRoomRef.current });
  }

  function handleDeleteRoom(roomId) {
    // If the current room is deleted, reset the state
    if (currentRoomRef.current && currentRoomRef.current.external_id === roomId) {
      setCurrentRoom(null);
      setMessages([]);
    }
    // Remove the room from the list of rooms
    setRooms((prevRooms) => prevRooms.filter(room => room.external_id !== roomId));
  }

  useEffect(() => {
    console.log("Initializing WebSocket client...");
    const wsConn = new GoChatWSClient("ws://localhost:8000/ws");
    setWsClient(wsConn);

    wsConn.onServerMessageMessage = (msg) => {
      setMessages((prevMessages) => [...prevMessages, msg.message]);
      // update the room's seq_id and last_read_seq_id when a new message is received
      const { room_id, seq_id } = msg.message;
      setRooms((prevRooms) =>
        prevRooms.map((room) => {
          if (room.id === room_id) {
            return { ...room, seq_id: seq_id };
          }
          return room;
        })
      );

      wsConn.readMessage(currentRoomRef.current?.external_id, seq_id)
        .then(() => {
          setRooms(prevRooms => {
            return prevRooms.map(room => {
              if (room.id === room_id) {
                return { ...room, last_read_seq_id: seq_id };
              }
              return room
            })
          })
        })
    };
    wsConn.onServerMessageUserPresence = (msg) => {
      const { user_id, present } = msg.notification.presence
      handleUserPresenceEvent(user_id, present);
    };
    wsConn.onServerMessageRoomPresence = (msg) => {
      const { room_id, present } = msg.notification.presence;
      handleRoomPresenceEvent(room_id, present);
    };
    wsConn.onServerMessageRoomDeleted = (msg) => {
      const roomId = msg.notification.room_deleted.room_id;
      handleDeleteRoom(roomId);
    };
    wsConn.onServerMessageSubscriptionChange = (msg) => {
      if (msg.notification.subscription_change.subscribed) {
        addSubscriber(msg.notification.subscription_change.user)
      } else {
        removeSubscriber(msg.notification.subscription_change.user.id)
      }
    };
    wsConn.onServerMessageNotificationMessage = (msg) => {
      // This is a notification message, update the room's seq_id
      const { room_id, seq_id } = msg.notification.message;
      setRooms((prevRooms) =>
        prevRooms.map((room) => {
          if (room.external_id === room_id) {
            return { ...room, seq_id: seq_id };
          }
          return room;
        })
      );
    }
    return () => {
      wsConn.close();
    };
  }, []);

  useEffect(() => {
    goChatClient.listSubscriptions()
      .then(subs => {
        setRooms(
          subs.map(sub => ({
            ...sub.room,
            last_read_seq_id: sub.last_read_seq_id
          }))
        );
      })
      .catch(err => {
        console.log("Failed to fetch rooms: " + err);
      });
  }, []);

  useEffect(() => {
    currentRoomRef.current = currentRoom;
  }, [currentRoom]);

  return (
    <>
      <Sidebar
        currentUser={currentUser}
        setCurrentUser={setCurrentUser}
        currentRoom={currentRoom}
        setCurrentRoom={setCurrentRoom}
        rooms={rooms}
        setRooms={setRooms}
        wsClient={wsClient}
      />
      {currentRoom ?
        <Chat
          currentUser={currentUser}
          currentRoom={currentRoom}
          setCurrentRoom={setCurrentRoom}
          rooms={rooms}
          setRooms={setRooms}
          messages={messages}
          setMessages={setMessages}
          wsClient={wsClient}
        /> :
        <Logo />}
    </>
  );
}