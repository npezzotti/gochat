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

  function handlePresenceEvent(user_id, present) {
    const updatedSubscribers = currentRoomRef.current?.subscribers.map(subscriber =>
      subscriber.user_id === user_id ? { ...subscriber, isPresent: present } : subscriber
    );

    if (updatedSubscribers) {
      setCurrentRoom({ ...currentRoomRef.current, subscribers: updatedSubscribers });
    }
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

  useEffect(() => {
    console.log("Initializing WebSocket client...");
    const wsConn = new GoChatWSClient("ws://localhost:8000/ws");
    setWsClient(wsConn);

    wsConn.onPublishMessage = (msg) => {
      setMessages((prevMessages) => [...prevMessages, msg.message]);
    };
    wsConn.onEventTypePresence = (msg) => {
      const { user_id, present } = msg.notification.presence
      handlePresenceEvent(user_id, present);
    };
    wsConn.onEventTypeRoomDeleted = (msg) => {
      console.log("Room deleted event:", msg);
    };
    wsConn.onEventTypeSubscriptionChange = (msg) => {
      if (msg.notification.subscription_change.subscribed) {
        addSubscriber(msg.notification.subscription_change.user)
      } else {
        removeSubscriber(msg.notification.subscription_change.user.id)
      }
    };
    return () => {
      wsConn.close();
    };
  }, []);

  useEffect(() => {
    goChatClient.listSubscriptions()
      .then(data => {
        setRooms(data);
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