# Todo

* Style join form
* Style create room form
* Fix menu styles
* When user unsubscribes from room
* * HTTP request removes subscription
* * Websocket server removes users sessions
* * Websocket server event sent for unsub
* When user subscribes to a room
* * Websocket event notifies subs and they update their cache
* Frontend for edit account
* Delete user

## Nice to have

* Upload files- will require separate API endpoint, local storage, and updating structure of chat message type


## WS client event types

User subscribes to a room
User unsubscribes to a room
User leaves a room
User publishes a message to a room

## WS server event types

Message published to a room a user is actively subscribed to
Message published to room user to which user is subscribed to, but not active in
User subscribed to a room a user is subscribed to and active in

