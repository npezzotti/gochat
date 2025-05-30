import './App.css';
import Main from './components/Main';
import Login from './components/Login';
import Register from './components/Register';
import ProtectedRoute from './components/ProtectedRoute';
import goChatClient from './gochat';

import { BrowserRouter, Navigate, Routes, Route } from 'react-router';
import { useState } from 'react';


function App() {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [currentUser, setCurrentUser] = useState(null);
  const [isLoading, setIsLoading] = useState(true);

  return (
    <BrowserRouter>
      <Routes>
        <Route element={<ProtectedRoute isAuthenticated={isAuthenticated} setIsAuthenticated={setIsAuthenticated} setCurrentUser={setCurrentUser} isLoading={isLoading} setIsLoading={setIsLoading} goChatClient={goChatClient} />}>
            <Route path="/" element={<Main currentUser={currentUser} setCurrentUser={setCurrentUser} />} />
        </Route>
        <Route path="/login" element={<Login setCurrentUser={setCurrentUser} setIsAuthenticated={setIsAuthenticated} goChatClient={goChatClient} />} />
        <Route path="/register" element={<Register />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}

export default App;
