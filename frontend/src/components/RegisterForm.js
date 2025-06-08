import { useState } from 'react';
import { NavLink, useNavigate } from 'react-router';

import goChatClient from '../gochat';

export default function RegisterForm() {
  const [state, setState] = useState('enabled');
  const [error, setError] = useState(null);
  const [email, setEmail] = useState('');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const navigate = useNavigate();

  function handleSubmit(e) {
    e.preventDefault();
    setState('disabled');

    if (email === '' || username === '' || password === '') {
      setError('Please fill in all fields');
      setState('enabled');
      return;
    }

    goChatClient.register(email, username, password)
      .then(_ => {
        navigate('/login', { replace: true });
      })
      .catch((err) => {
        setError("Registration failed: " + err);
      })
      .finally(() => {
        setState('enabled');
      });
  }

  const handleChangeEmail = (e) => {
    setEmail(e.target.value)
  }

  const handleChangeUsername = (e) => {
    setUsername(e.target.value)
  }

  const handleChangePassword = (e) => {
    setPassword(e.target.value)
  }

  return (
    <div>
      <div className="sidebar-header">
        <h1>Sign in</h1>
      </div>
      <form className="sidebar-form" id="login-form" onSubmit={handleSubmit}>
        {error !== null ?
          <p id="error-message" className="error">{error}</p>
          : ''}
        <label htmlFor="email">Email Address</label>
        <input type="text" name="email" id="email" className='sidebar-input' value={email} onChange={handleChangeEmail} disabled={
          state === 'disabled'
        } />
        <label htmlFor="username">Username</label>
        <input type="text" name="username" id="username" className='sidebar-input' value={username} onChange={handleChangeUsername} disabled={
          state === 'disabled'
        } />
        <label htmlFor="password">Password</label>
        <input type="password" name="password" id="password" className='sidebar-input' value={password} onChange={handleChangePassword} disabled={
          state === 'disabled'
        } />
        <input type="submit" value="Register" disabled={
          state === 'disabled'
        } />
      </form>
      <p>
        Already have an account? <NavLink to="/login">Sign in</NavLink>
      </p>
    </div>
  )
}
