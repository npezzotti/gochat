import { useState } from 'react';
import { NavLink, useNavigate } from 'react-router';

export default function LoginForm({ setCurrentUser, setIsAuthenticated, goChatClient }) {
  const [state, setState] = useState('enabled');
  const [error, setError] = useState(null);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const navigate = useNavigate();

  function handleSubmit(e) {
    e.preventDefault();
    setState('disabled');

    if (email === '' || password === '') {
      setError('Please fill in all fields');
      setState('enabled');
      return;
    }

    goChatClient.login(email, password)
      .then(res => {
        setIsAuthenticated(true)
        setCurrentUser(res);
        navigate('/', { replace: true });
      }).catch((err) => {
        setError("Login failed: " + err.message);
      })
      .finally(() => {
        setState('enabled');
      });
  }

  const handleChangeEmail = (e) => {
    setEmail(e.target.value)
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
        <input type="text" name="email" id="email" value={email} onChange={handleChangeEmail} disabled={
          state === 'disabled'
        } />
        <label htmlFor="password">Password</label>
        <input type="password" name="password" id="password" value={password} onChange={handleChangePassword} disabled={
          state === 'disabled'
        } />
        <input type="submit" id="login" value="Login" disabled={
          state === 'disabled'
        } />
      </form>
      <p>
        Don't have an account? <NavLink to="/register">Sign up</NavLink>
      </p>
    </div>
  )
}
