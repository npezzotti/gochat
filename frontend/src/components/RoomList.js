import { useState } from 'react';

export default function RoomList({ currentRoom, setCurrentRoom, rooms, wsClient, handleJoinRoomSuccess }) {
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
              handleJoinRoomSuccess(joinedMsg.response.data);
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
          handleJoinRoomSuccess(joinedMsg.response.data);
        })
        .catch(err => {
          setError("Failed to join room: " + err);
        });
    }
  }

  function isActiveRoom(room) {
    return currentRoom && room.external_id === currentRoom.external_id;
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
              `room-item ${isActiveRoom(room) ?
                'active-room' : ''}`
            }
            data-room-external-id={room.external_id}
            onClick={handleJoinRoom}
          >
            <div className='room-item-info' key={room.id}>
              <div className='room-name'>
                {room.name}
              </div>
              {room.seq_id && room.last_read_seq_id && room.seq_id - room.last_read_seq_id > 0 ?
                <div className='unread-badge'>{room.seq_id - room.last_read_seq_id > 9 ? '9+' : room.seq_id - room.last_read_seq_id}</div> :
                ''}
            </div>
            <div className='room-item-meta'>
              <div class={`room-status ${room.is_online ? 'online' : 'offline'}`}></div>
            </div>
          </div>
        )
      })}
    </div>
  )
}
