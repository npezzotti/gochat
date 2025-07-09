package stats

import "github.com/stretchr/testify/mock"

type MockStatsUpdater struct {
	mock.Mock
}

func (m *MockStatsUpdater) Incr(name string) {
	m.Called(name)
}
func (m *MockStatsUpdater) Decr(name string) {
	m.Called(name)
}
func (m *MockStatsUpdater) RegisterMetric(name string) {
	m.Called(name)
}
func (m *MockStatsUpdater) Run() {
	m.Called()
}
