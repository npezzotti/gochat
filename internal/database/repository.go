package database

type GoChatRepository interface {
	Ping() error
	CreateAccount(accountParams CreateAccountParams) (User, error)
	UpdateAccount(params UpdateAccountParams) (User, error)
	GetAccountById(accountId int) (User, error)
	GetAccountByEmail(email string) (User, error)
	GetRoomByExternalId(externalId string) (Room, error)
	GetRoomWithSubscribers(roomId int) (*Room, error)
	CreateRoom(params CreateRoomParams) (Room, error)
	DeleteRoom(id int) error
	CreateSubscription(accountId, roomId int) (Subscription, error)
	SubscriptionExists(accountId, roomId int) bool
	ListSubscriptions(accountId int) ([]Subscription, error)
	DeleteSubscription(accountId, roomId int) error
	UpdateLastReadSeqId(accountId, roomId, seqId int) error
	CreateMessage(msg Message) error
	UpdateRoomOnMessage(msg Message) error
	GetSubscribersByRoomId(roomId int) ([]User, error)
	GetMessages(roomId, since, before, limit int) ([]Message, error)
}
