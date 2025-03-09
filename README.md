# Todo

* Design structure of messages (user, other users, server)
* Remove header, refactor design to be only chat box with side panel containing list of rooms
* Confirm password in register form
* Server and client side Form validation

## Nice to have
* Multiple different chatrooms
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

