import { useState } from 'react';

export default function JoinRoomForm({ rooms, setRooms, currentRoom, setCurrentRoom, wsClient, handleJoinRoomSuccess }) {
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
                handleJoinRoomSuccess(joinedMsg.response.data);
                setRoomId('');
              })
              .catch(err => {
                setCurrentRoom(null);
                setError("Failed to join room: " + err)
              })
          })
      } else {
        wsClient.joinRoom(roomId)
          .then(joinedMsg => {
            handleJoinRoomSuccess(joinedMsg.response.data);
            setRoomId('');
          })
          .catch(err => {
            setError("Already subscribed to room - failed to join: " + err)
          })
      }
    } else {
      wsClient.joinRoom(roomId)
        .then(joinedMsg => {
          // Add the room to the list of rooms
          setRooms([...rooms, {...joinedMsg.response.data}]);
          handleJoinRoomSuccess(joinedMsg.response.data);
          setRoomId('');
        })
        .catch(err => {
          setError("Failed to join room: " + err);
        });
      e.target.roomId.value = '';
      setError(null);
    }
  }

  return (
    <form id="join-room-form" className="sidebar-form" onSubmit={handleSubscribeRoom}>
      {error !== null ?
        <p id="error-message" className="error">{error}</p>
        : ''}
      <h3 className="sidebar-section-title" htmlFor="roomId">Join Room</h3>
      <input
        type="text"
        id="roomId"
        name="roomId"
        className='sidebar-input'
        value={roomId}
        onChange={e => setRoomId(e.target.value)}
        placeholder="Enter Room ID"
        aria-label="Join Room"
      />
      <input type="submit" value="Join Room"></input>
    </form>
  );
}