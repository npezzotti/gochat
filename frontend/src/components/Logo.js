import logo from '../logo.png';

export default function Logo() {
  return (
    <div className="chat-container">
      <div className="logo-container">
        <img className="logo" src={logo} alt="go-chat" />
      </div>
    </div>
  )
}