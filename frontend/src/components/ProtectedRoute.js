import { useLocation, Navigate, Outlet } from 'react-router';
import { useEffect } from 'react';
import goChatClient from '../gochat';

export default function ProtectedRoute({ isAuthenticated, setIsAuthenticated, setCurrentUser, isLoading, setIsLoading }) {
  const location = useLocation();

  useEffect(() => {
    const checkAuth = () => {
      goChatClient.session()
        .then(data => {
          if (data) {
            setIsAuthenticated(true);
            setCurrentUser(data);
          } else {
            setIsAuthenticated(false);
          }
        })
        .catch(error => {
          console.error("Failed to check session: " + error);
          setIsAuthenticated(false);
        })
        .finally(() => {
          setIsLoading(false);
        });
    }

    checkAuth();
  }, [setCurrentUser, setIsAuthenticated, setIsLoading]);

  if (isLoading) {
    return <div>Loading...</div>;
  }

  return isAuthenticated ? <Outlet /> : <Navigate to='/login' state={{ from: location }} replace />;
}
