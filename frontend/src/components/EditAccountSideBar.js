import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";
import { faArrowLeft } from '@fortawesome/free-solid-svg-icons'

import { useState } from 'react';
import goChatClient from "../gochat";


export default function EditAccountSideBar({ currentUser, setCurrentUser, setShowEditAccount }) {
  const [error, setError] = useState(null);
  const [username, setUsername] = useState(currentUser.username);
  const [password, setPassword] = useState('');
  
  function handleSubmit(e) {
    e.preventDefault();

    const form = e.target;
    const username = form.username.value;
    const password = form.password.value;
  
    if (username === '' || password === '') {
      setError('Please fill in all fields');
      return;
    }

    goChatClient.updateAccount(username, password)
      .then(user => {
        setCurrentUser(user)
        setError(null);
        form.reset();
      }).catch((err) => {
        setError("Failed to update account: " + err);
      });
  }
  
  const handleChangeUsername = (e) => {
    setUsername(e.target.value)
  }

  const handleChangePassword = (e) => {
    setPassword(e.target.value)
  }

  return (
    <>
      <div className="actions-header">
        <button id="close-btn" className="icon-button" aria-label="Close" onClick={() => {setShowEditAccount(false)}}>
          <FontAwesomeIcon icon={faArrowLeft} />
        </button>
        <h2>Account</h2>
      </div>

      <div className="account-info">
        <form id="update-acct-form" className="sidebar-form" onSubmit={handleSubmit}>
          {error !== null ?
            <p id="error-message" className="error">{error}</p>
            : ''}
          <label htmlFor="email">Email</label>
          <input type="text" id="email" name="email" value={currentUser.email_address} aria-label="Email" readOnly disabled />
          <label htmlFor="username">Username</label>
          <input type="text" id="username" name="username" value={username} aria-label="Username" onChange={handleChangeUsername} />
          <label htmlFor="password">Password</label>
          <input type="password" id="passsword" name="password" value={password} placeholder="**********" required="" autoComplete="on" aria-label="Password" onChange={handleChangePassword}/>
          <input type="submit" value="Update" />
        </form>
      </div>
    </>
  )
}
