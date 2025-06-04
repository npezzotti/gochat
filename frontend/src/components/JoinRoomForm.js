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
      if (currentRoom) {
        wsClient.leaveRoom(currentRoom.external_id)
          .then(_ => {
            wsClient.joinRoom(roomId)
              .then(joinedMsg => {
                setCurrentRoom(joinedMsg.response.data);
                setRoomId('');
              })
              .catch(err => {
                setError("Already subscribed to room - failed to join: " + err)
              })
          })
      } else {
        wsClient.joinRoom(roomId)
        .then(joinedMsg => {
          setCurrentRoom(joinedMsg.response.data);
          setRoomId('');
        })
        .catch(err => {
          setError("Already subscribed to room - failed to join: " + err)
        })
      }
    } else {
      // goChatClient.subscribeRoom(roomId)
        // .then(sub => {
          wsClient.joinRoom(roomId)
          .then(joinedMsg => {
              setRooms([...rooms, {
                id: joinedMsg.response.data.id,
                external_id: joinedMsg.response.data.external_id,
                name: joinedMsg.response.data.name,
                created_at: joinedMsg.response.data.created_at,
                updated_at: joinedMsg.response.data.updated_at,
              }]);
              setCurrentRoom(joinedMsg.response.data);
              setRoomId('');
            })
            .catch(err => {
              setError("Failed to join room: " + err);
            });
          e.target.roomId.value = '';
          setError(null);
        // })
        // .catch(err => {
        //   setError("Failed to subscribe to room: " + err);
        // });
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