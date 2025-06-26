package database

import (
	"github.com/stretchr/testify/mock"
)

type MockGoChatRepository struct {
	mock.Mock
}

func (m *MockGoChatRepository) CreateAccount(accountParams CreateAccountParams) (User, error) {
	args := m.Called(accountParams)
	return args.Get(0).(User), args.Error(1)
}
func (m *MockGoChatRepository) UpdateAccount(params UpdateAccountParams) (User, error) {
	args := m.Called(params)
	return args.Get(0).(User), args.Error(1)
}
func (m *MockGoChatRepository) GetAccountById(userId int) (User, error) {
	args := m.Called(userId)
	return args.Get(0).(User), args.Error(1)
}
func (m *MockGoChatRepository) GetAccountByEmail(email string) (User, error) {
	args := m.Called(email)
	return args.Get(0).(User), args.Error(1)
}
func (m *MockGoChatRepository) GetRoomByExternalId(externalId string) (Room, error) {
	args := m.Called(externalId)
	return args.Get(0).(Room), args.Error(1)
}
func (m *MockGoChatRepository) GetRoomWithSubscribers(roomId int) (*Room, error) {
	args := m.Called(roomId)
	if room, ok := args.Get(0).(*Room); ok {
		return room, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockGoChatRepository) CreateRoom(params CreateRoomParams) (Room, error) {
	args := m.Called(params)
	return args.Get(0).(Room), args.Error(1)
}
func (m *MockGoChatRepository) DeleteRoom(id int) error {
	args := m.Called(id)
	return args.Error(0)
}
func (m *MockGoChatRepository) CreateSubscription(userId, roomId int) (Subscription, error) {
	args := m.Called(userId, roomId)
	return args.Get(0).(Subscription), args.Error(1)
}
func (m *MockGoChatRepository) SubscriptionExists(account_id, room_id int) bool {
	args := m.Called(account_id, room_id)
	return args.Bool(0)
}
func (m *MockGoChatRepository) ListSubscriptions(account_id int) ([]Subscription, error) {
	args := m.Called(account_id)
	return args.Get(0).([]Subscription), args.Error(1)
}
func (m *MockGoChatRepository) DeleteSubscription(accountId, roomId int) error {
	args := m.Called(accountId, roomId)
	return args.Error(0)
}
func (m *MockGoChatRepository) UpdateLastReadSeqId(userId, roomId, seqId int) error {
	args := m.Called(userId, roomId, seqId)
	return args.Error(0)
}
func (m *MockGoChatRepository) CreateMessage(msg Message) error {
	args := m.Called(msg)
	return args.Error(0)
}
func (m *MockGoChatRepository) UpdateRoomOnMessage(msg Message) error {
	args := m.Called(msg)
	return args.Error(0)
}
func (m *MockGoChatRepository) GetSubscribersByRoomId(roomId int) ([]User, error) {
	args := m.Called(roomId)
	return args.Get(0).([]User), args.Error(1)
}
func (m *MockGoChatRepository) GetMessages(roomId, since, before, limit int) ([]Message, error) {
	args := m.Called(roomId, since, before, limit)
	return args.Get(0).([]Message), args.Error(1)
}
