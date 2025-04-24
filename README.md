# Todo

* Websocket client
* Chat client (w/Rest APIs and Websocket connectionse)
** isAuthenticated
* Frontend notifications
* Delete user

## Nice to have

* Re-establish connection
* Upload files- will require separate API endpoint, local storage, and updating structure of chat message type


## WS client message types

* Join room
* Leave room
* Publish message


## WS server message types

* Presence
* * User is present in a room the user is active in
* * User no longer present in a room the user is active in
* Subscription
* * User subscribed to a room (sent to all users active in room)
* * User unsubscribed from a room (sent to all users active in room)
* Publish
* * Message published to a room (sent to all users active in room)
* * Message published to room user to which user is subscribed to, but not active in (sent to all subscribed users)
* Notification
* * A room which a user is active in was deleted. (sent to all active users)
