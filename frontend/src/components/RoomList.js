import { useState } from 'react';

export default function RoomList({ currentRoom, setCurrentRoom, rooms, wsClient }) {
  const [error, setError] = useState(null);

  const handleJoinRoom = e => {
    if (error) {
      setError(null);
    }

    const roomId = parseInt(e.target.id);
    if (roomId === currentRoom?.id) {
      return; // Already in this room, do nothing
    }

    if (currentRoom) {
      wsClient.leaveRoom(currentRoom.id)
        .then(_ => {
          setCurrentRoom(null)
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
      {rooms.length === 0 && <p>No rooms available</p>}
      {rooms.map(room => {
        return (
          <div
            key={room.id}
            id={room.id}
            data-room-external-id={room.external_id}
            className={
              `room ${currentRoom && room.external_id === currentRoom.external_id ?
                'active-room' : ''}`
            }
            onClick={handleJoinRoom}>
            {room.name}
          </div>
        )
      })}
    </div>
  )
}
