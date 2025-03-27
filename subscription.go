package main

type Subscription struct {
	Id   int   `json:"id"`
	User User  `json:"user"`
	Room *Room `json:"room"`
}
