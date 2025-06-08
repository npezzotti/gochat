import { useState } from 'react';

export default function RoomList({ currentRoom, setCurrentRoom, rooms, wsClient }) {
  const [error, setError] = useState(null);

  const handleJoinRoom = e => {
    if (error) {
      setError(null);
    }

    const roomId = e.currentTarget.dataset.roomExternalId;
    if (roomId === currentRoom?.external_id) {
      return; // Already in this room, do nothing
    }

    if (currentRoom) {
      wsClient.leaveRoom(currentRoom.external_id)
        .then(_ => {
          wsClient.joinRoom(roomId)
            .then(joinedMsg => {
              setCurrentRoom(joinedMsg.response.data);
            })
            .catch(err => {
              setCurrentRoom(null);
              setError("Failed to join room: " + err);
            });
        })
        .catch(err => {
          setError("Failed to leave room: " + err);
        });
    } else {
      wsClient.joinRoom(roomId)
        .then(joinedMsg => {
          setCurrentRoom(joinedMsg.response.data);
        })
        .catch(err => {
          setError("Failed to join room: " + err);
        });
    }
  }

  return (
    <div id="room-list">
      {error && <p className="error">{error}</p>}
      <div class="rooms-title">Recent Rooms</div>
      {rooms.length === 0 && <p>No rooms available</p>}
      {rooms.map(room => {
        return (
          <div
            id={room.id}
            key={room.id}
            className={
              `room-item ${currentRoom && room.external_id === currentRoom.external_id ?
                'active-room' : ''}`
            }
            data-room-external-id={room.external_id}
            onClick={handleJoinRoom}
          >
          <div className='room-name'>
            {room.name}
          </div>
          <div class="room-status" style={room.isOnline ? {color: 'green'} : {color: 'grey'}}></div>
          </div>
        )
      })}
    </div>
  )
}
