import { useState } from 'react';

import goChatClient from '../gochat'

export default function JoinRoomForm({ rooms, setRooms, currentRoom, setCurrentRoom, wsClient }) {
  const [error, setError] = useState(null)
  const [roomId, setRoomId] = useState('')

  const handleSubscribeRoom = e => {
    if (error) {
      setError(null);
    }

    e.preventDefault();
    if (roomId === '') {
      setError('Please enter a room ID');
      return;
    }

    if (roomId === currentRoom?.external_id) {
      setRoomId('');
      return;
    }

    if (rooms.some(room => room.external_id === roomId)) {
      const id = rooms.find(room => room.external_id === roomId).id;
      if (currentRoom) {
        wsClient.leaveRoom(currentRoom.id)
          .then(_ => {
            wsClient.joinRoom(id)
              .then(joinedMsg => {
                setCurrentRoom(joinedMsg.response.data);
                setRoomId('');
              })
              .catch(err => {
                setError("Already subscribed to room - failed to join: " + err)
              })
          })
      } else {
        wsClient.joinRoom(id)
        .then(joinedMsg => {
          setCurrentRoom(joinedMsg.response.data);
          setRoomId('');
        })
        .catch(err => {
          setError("Already subscribed to room - failed to join: " + err)
        })
      }
    } else {
      goChatClient.subscribeRoom(roomId)
        .then(sub => {
          setRooms([...rooms, sub.room]);
          wsClient.joinRoom(sub.room.id)
            .then(joinedMsg => {
              setCurrentRoom(joinedMsg.response.data);
              setRoomId('');
            })
            .catch(err => {
              setError("Failed to join room: " + err);
            });
          e.target.roomId.value = '';
          setError(null);
        })
        .catch(err => {
          setError("Failed to subscribe to room: " + err);
        });
    }
  }

  return (
    <form id="join-room-form" className="sidebar-form" onSubmit={handleSubscribeRoom}>
      {error !== null ?
        <p id="error-message" className="error">{error}</p>
        : ''}
      <label htmlFor="roomId">Join Room</label>
      <input
        type="text"
        id="roomId"
        name="roomId"
        value={roomId}
        onChange={e => setRoomId(e.target.value)}
        placeholder="Enter room ID"
        // required
        aria-label="Join Room"
      />
      <input type="submit" value="Join"></input>
    </form>
  );
}