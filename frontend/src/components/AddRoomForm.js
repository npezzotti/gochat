import { useState } from 'react';
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";
import { faArrowLeft } from '@fortawesome/free-solid-svg-icons'

import goChatClient from '../gochat';

export default function AddRoomForm({ setShowAddUser, rooms, setRooms, setCurrentRoom, wsClient, handleJoinRoomSuccess }) {
  const [error, setError] = useState(null);

  const handleAddRoom = e => {
    e.preventDefault();
    const form = e.target;
    const name = form.name.value;
    const description = form.description.value;

    if (name === '' || description === '') {
      alert('Please fill in all fields');
      return;
    }
    goChatClient.createRoom(name, description)
      .then(room => {
        setRooms([...rooms, room])
        wsClient.joinRoom(room.external_id)
          .then(joinedMsg => {
            handleJoinRoomSuccess(joinedMsg.response.data);
            setShowAddUser(false);
          })
          .catch(err => {
            throw new Error("Failed to join room: " + err);
          });
      })
      .catch(err => {
        setError("Failed to create room: " + err);
      })
  }

  return (
    <>
      <div className="actions-header">
        <button className="icon-button" onClick={() => { setShowAddUser(false) }}>
          <FontAwesomeIcon icon={faArrowLeft} />
        </button>
        <h2>New Chat Room</h2>
      </div>
      {error !== null ?
        <p className="error">{error}</p>
        : ''}
      <form className="sidebar-form" onSubmit={handleAddRoom}>
        <label htmlFor="name">Name:</label>
        <input type="text" id="name" name="name" className='sidebar-input' />
        <label htmlFor="description">Description:</label>
        <input type="text" id="description" name="description" className='sidebar-input' />
        <input type="submit" value="Create Room" />
      </form>
    </>
  )
}
