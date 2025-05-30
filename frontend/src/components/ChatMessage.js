export default function ChatMessage({ message, currentUser, currentRoom }) {
  const isCurrentUser = message.user_id === currentUser.id;
  const user = currentRoom.subscribers.find(sub => sub.user_id === message.user_id); // Find the user in the subscribers list
  const username = user ? user.username : 'Unknown';

  function formatTimestamp(timestamp) {
    return new Date(timestamp).toLocaleTimeString()
  }

  return (
    <div className={`chat-message ${isCurrentUser ? 'user' : ''}`} data-message-id={message.id} data-message-seq-id={message.seq_id}><div className="meta">{username} • {formatTimestamp(message.timestamp)}</div>{message.content}</div>
  )
}
