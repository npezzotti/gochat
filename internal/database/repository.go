package database

type GoChatRepository interface {
	CreateAccount(accountParams CreateAccountParams) (User, error)
	UpdateAccount(params UpdateAccountParams) (User, error)
	GetAccountById(userId int) (User, error)
	GetAccountByEmail(email string) (User, error)
	GetRoomByExternalId(externalId string) (Room, error)
	GetRoomWithSubscribers(roomId int) (*Room, error)
	CreateRoom(params CreateRoomParams) (Room, error)
	DeleteRoom(id int) error
	CreateSubscription(userId, roomId int) (Subscription, error)
	SubscriptionExists(account_id, room_id int) bool
	ListSubscriptions(account_id int) ([]Subscription, error)
	DeleteSubscription(accountId, roomId int) error
	UpdateLastReadSeqId(userId, roomId, seqId int) error
	CreateMessage(msg Message) error
	UpdateRoomOnMessage(msg Message) error
	GetSubscribersByRoomId(roomId int) ([]User, error)
	GetMessages(roomId, since, before, limit int) ([]Message, error)
}
