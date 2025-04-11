# Todo

* User external IDs
* * Add database table for external Ids
* * Replace all references to Id with external ID
* When user subscribes to a room
* * HTTP request subscribes user
* * Websocket event notifies subs and they add user to subs cache
* When a user unsubscrubes from a room
* * HTTP request removes subscription
* * Websocket server removes users sessions
* * Websocket server event sent for unsub
* When a user actively opens a room
* * Websocket event notifies other active user that the user joined
* When a user is no longer active in room
* * Websocket event notifies other active users that user left
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

