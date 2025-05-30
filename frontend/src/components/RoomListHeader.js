import { FontAwesomeIcon } from '@fortawesome/react-fontawesome'
import { faGear, faPlus } from '@fortawesome/free-solid-svg-icons'
import { useState } from 'react';
import { useNavigate } from 'react-router';

import goChatClient from '../gochat';

export default function RoomListHeader({ currentUser, setShowAddUser, setShowEditAccount }) {
  const [showDropdownContent, setShowDropdownContent] = useState(false);
  const navigate = useNavigate()

  const toggleDropdownContent = () => {
    setShowDropdownContent(!showDropdownContent);
  }

  const handleLogout = () => {
    goChatClient.logout()
      .then(() => {
        navigate('/login', {replace: true})
      })
      .catch(err => {
        console.log("Error logging out: ", err)
      })
  }

  return (
    <div className="sidebar-header">
      <h2>{currentUser.username}</h2>
      <div className="menu-icons">
        <button className="icon-button" >
          <FontAwesomeIcon id="add-room-btn" icon={faPlus} onClick={setShowAddUser} />
        </button>
        <div id="account-opts-btn" className="dropdown">
          <button className="icon-button" >
            <FontAwesomeIcon icon={faGear} onClick={toggleDropdownContent} />
          </button>
          <div className="dropdown-content" style={{ display: showDropdownContent ? 'block' : 'none' }}>
            <a id="account" onClick={() => { setShowEditAccount(true) }}>Account</a>
            <a id="logout-btn" onClick={handleLogout}>Logout</a>
          </div>
        </div>
      </div>
    </div>
  )
}
