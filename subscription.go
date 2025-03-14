package main

type CreateSubscriptionParams struct {
	user *User
	room *Room
}

type Subscription struct {
	user *User
	room *Room
}
