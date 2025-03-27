# Todo

* Move subscribe/unsubscribe to HTTP handler
* When user subscribes to a room via form:
* * HTTP request subscribes them to the room
* * HTTP request populates room information and subs
* * WS client sends join message
* When user unsubscribes from room
* * HTTP request removes subscription
* * Websocket server event sent for unsub, frontend cleans up view
* When User join/leaves room
* * WS client sends join/leave message
* Frontend for Room details view
* Frontend for edit account
* Implement logout in frontend
* Delete user

## Nice to have

* Upload files- will require separate API endpoint, local storage, and updating structure of chat message type

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

