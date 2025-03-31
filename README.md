# Todo

* Populate room name in header
* When user unsubscribes from room
* * HTTP request removes subscription
* * Websocket server removes users sessions
* * Websocket server event sent for unsub
* Frontend for Room details view
* Frontend for edit account
* Implement logout in frontend
* Delete user

## Nice to have

* Upload files- will require separate API endpoint, local storage, and updating structure of chat message type


## WS client event types

User subscribes to a room
User unsubscribes to a room
User leaves a room
User publishes a message to a room

## WS server event types

Message published to room user to which user is subscribed
User modified a room details

