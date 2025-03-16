# Todo

* Save messages
* Design structure of messages (user, other users, server)
* Confirm password in register form
* Server and client side Form validation

## Nice to have

* Upload files- will require separate API endpoint, local storage, and updating structure of chat message type

## Postgres init
```
CREATE TABLE accounts(
  id SERIAL PRIMARY KEY,
  username VARCHAR(50) UNIQUE NOT NULL,
  email VARCHAR(50) UNIQUE NOT NULL,
  password_hash VARCHAR(100) NOT NULL,
  created_at TIMESTAMP,
  updated_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS subscriptions(
  id         SERIAL PRIMARY KEY,
  created_at TIMESTAMP(3) NOT NULL,
  updated_at TIMESTAMP(3) NOT NULL,
  account_id INT NOT NULL,
  room_id    INT NOT NULL,
  FOREIGN KEY(account_id) REFERENCES accounts(id),
  FOREIGN KEY(room_id) REFERENCES rooms(id)
);
CREATE UNIQUE INDEX subscriptions_room_user_id ON subscriptions(account_id, room_id);

CREATE TABLE messages(
			id        SERIAL PRIMARY KEY,
			created_at TIMESTAMP(3) NOT NULL,
			updated_at TIMESTAMP(3) NOT NULL,
			seq_id     INT NOT NULL,
			room_id     INT NOT NULL,
			from    INT NOT NULL,
			content   VARCHAR(100),
			FFOREIGN KEY(room_id) REFERENCES rooms(id)
			FFOREIGN KEY(from) REFERENCES accounts(id)
);
CREATE UNIQUE INDEX messages_room_seq_id ON messages(room_id, seq_id);
```

# HTTP

Create User
Login
Get account information
Update account information
User creates a room
User deletes a room

## WS client event types

User subscribes to a room
User unsubscribes to a room
User leaves a room
User publishes a message to a room

## WS server event types

Message published to room user to which user is subscribed
User modified a room details

