import RoomListHeader from './RoomListHeader'
import JoinRoomForm from './JoinRoomForm'
import RoomList from './RoomList'
import EditAccountSideBar from './EditAccountSideBar';
import AddRoomForm from './AddRoomForm';

import { useState } from 'react';

export default function Sidebar({ currentUser, setCurrentUser, currentRoom, setCurrentRoom, rooms, setRooms, wsClient }) {
  const [showAddUser, setShowAddUser] = useState(false);
  const [showEditAccount, setShowEditAccount] = useState(false);

  function handleJoinRoomSuccess(joinedRoom) {
    setRooms(prevRooms =>
      prevRooms.map(room =>
        room.external_id === joinedRoom.external_id
          ? { ...room, is_online: true }
          : room
      )
    );
    setCurrentRoom(joinedRoom);
  }

  if (showAddUser) {
    return (
      <div className="sidebar">
        <AddRoomForm setShowAddUser={setShowAddUser} rooms={rooms} setRooms={setRooms} setCurrentRoom={setCurrentRoom} wsClient={wsClient} handleJoinRoomSuccess={handleJoinRoomSuccess} />
      </div>
    )
  } else if (showEditAccount) {
    return (
      <div className="sidebar">
        <EditAccountSideBar currentUser={currentUser} setCurrentUser={setCurrentUser} setShowEditAccount={setShowEditAccount} />
      </div>
    )
  } else {
    return (
      <div className="sidebar">
        <RoomListHeader currentUser={currentUser} setShowAddUser={setShowAddUser} setShowEditAccount={setShowEditAccount} />
        <JoinRoomForm rooms={rooms} setRooms={setRooms} currentRoom={currentRoom} setCurrentRoom={setCurrentRoom} wsClient={wsClient} handleJoinRoomSuccess={handleJoinRoomSuccess} />
        <RoomList currentRoom={currentRoom} setCurrentRoom={setCurrentRoom} rooms={rooms} wsClient={wsClient} handleJoinRoomSuccess={handleJoinRoomSuccess} />
      </div>
    )
  }
}
