import LoginForm from "./LoginForm"
import Logo from "./Logo"


export default function Login({ setCurrentUser, setIsAuthenticated, goChatClient }) {
  return (
    <>
      <div className="sidebar">
        <LoginForm setCurrentUser={setCurrentUser} setIsAuthenticated={setIsAuthenticated} goChatClient={goChatClient} />
      </div>
      <Logo />
    </>
  )
}