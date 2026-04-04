// Package mocks provides reusable mock implementations for NATS interfaces.
// Use these mocks in unit tests across all services.
package mocks

import (
	"context"

	natsgo "github.com/nats-io/nats.go"
	natsModel "github.com/Mapex-Solutions/mapexGoKit/infrastructure/nats"
	"github.com/stretchr/testify/mock"
)

// ============================================================================
//                              MOCK FANOUT
// ============================================================================

// MockFanout implements natsModel.Fanout interface for testing.
// Use this mock for services that publish FANOUT messages.
//
// Example usage:
//
//	mockFanout := new(mocks.MockFanout)
//	mockFanout.On("PublishFanout", ctx, "subject", mock.Anything).Return(nil)
type MockFanout struct {
	mock.Mock
}

// PublishFanout mocks the PublishFanout method.
func (m *MockFanout) PublishFanout(ctx context.Context, subject string, data []byte) error {
	args := m.Called(ctx, subject, data)
	return args.Error(0)
}

// SubscribeFanout mocks the SubscribeFanout method.
func (m *MockFanout) SubscribeFanout(stream, serviceName, subject string, handler natsModel.FanoutHandler) (*natsModel.FanoutSubscription, error) {
	args := m.Called(stream, serviceName, subject, handler)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*natsModel.FanoutSubscription), args.Error(1)
}

// EnsureFanoutStream mocks the EnsureFanoutStream method.
func (m *MockFanout) EnsureFanoutStream(config natsModel.FanoutStreamConfig) error {
	args := m.Called(config)
	return args.Error(0)
}

// Compile-time check
var _ natsModel.Fanout = (*MockFanout)(nil)

// ============================================================================
//                            MOCK SUBSCRIBER
// ============================================================================

// MockSubscriber implements natsModel.Subscriber interface for testing.
// Use this mock for consumers that use Subscribe pattern.
//
// Example usage:
//
//	mockSubscriber := new(mocks.MockSubscriber)
//	mockSubscriber.On("Subscribe", mock.Anything).Return(func() error { return nil }, nil)
type MockSubscriber struct {
	mock.Mock
}

// Subscribe mocks the Subscribe method.
func (m *MockSubscriber) Subscribe(config natsModel.SubscribeConfig) (func() error, error) {
	args := m.Called(config)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(func() error), args.Error(1)
}

// Compile-time check
var _ natsModel.Subscriber = (*MockSubscriber)(nil)

// ============================================================================
//                            MOCK PUBLISHER
// ============================================================================

// MockPublisher implements natsModel.Publisher interface for testing.
// Use this mock for services that publish standard messages.
//
// Example usage:
//
//	mockPublisher := new(mocks.MockPublisher)
//	mockPublisher.On("Publish", mock.Anything).Return(nil)
type MockPublisher struct {
	mock.Mock
}

// Publish mocks the Publish method.
func (m *MockPublisher) Publish(config natsModel.PublishConfig) error {
	args := m.Called(config)
	return args.Error(0)
}

// Compile-time check
var _ natsModel.Publisher = (*MockPublisher)(nil)

// ============================================================================
//                            MOCK FETCHER
// ============================================================================

// MockFetcher implements natsModel.Fetcher interface for testing.
// Use this mock for consumers that use Fetch pattern.
//
// Example usage:
//
//	mockFetcher := new(mocks.MockFetcher)
//	mockFetcher.On("Fetch", mock.Anything).Return(func() error { return nil }, nil)
type MockFetcher struct {
	mock.Mock
}

// Fetch mocks the Fetch method.
func (m *MockFetcher) Fetch(config natsModel.FetchConfig) (func() error, error) {
	args := m.Called(config)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(func() error), args.Error(1)
}

// Compile-time check
var _ natsModel.Fetcher = (*MockFetcher)(nil)

// ============================================================================
//                       MOCK CONNECTION PROVIDER
// ============================================================================

// MockConnectionProvider implements natsModel.ConnectionProvider interface for testing.
// Use this mock for consumers that need direct NATS connection (e.g., Auth Callout).
//
// Example usage:
//
//	mockConnProvider := new(mocks.MockConnectionProvider)
//	mockConnProvider.On("GetConn").Return(nil) // or return a real connection for integration tests
type MockConnectionProvider struct {
	mock.Mock
}

// GetConn mocks the GetConn method.
func (m *MockConnectionProvider) GetConn() *natsgo.Conn {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*natsgo.Conn)
}

// Compile-time check
var _ natsModel.ConnectionProvider = (*MockConnectionProvider)(nil)

// ============================================================================
//                          MOCK CORE PUBLISHER
// ============================================================================

// MockCorePublisher implements natsModel.CorePublisher interface for testing.
// Use this mock for high-throughput consumers that use fire-and-forget publishing.
//
// Example usage:
//
//	mockCore := new(mocks.MockCorePublisher)
//	mockCore.On("PublishCore", mock.Anything).Return(nil)
//	mockCore.On("FlushConnection").Return(nil)
type MockCorePublisher struct {
	mock.Mock
}

// PublishCore mocks the PublishCore method.
func (m *MockCorePublisher) PublishCore(config natsModel.PublishCoreConfig) error {
	args := m.Called(config)
	return args.Error(0)
}

// FlushConnection mocks the FlushConnection method.
func (m *MockCorePublisher) FlushConnection() error {
	args := m.Called()
	return args.Error(0)
}

// Compile-time check
var _ natsModel.CorePublisher = (*MockCorePublisher)(nil)

// ============================================================================
//                         MOCK COMBINED (BUS)
// ============================================================================

// MockBus implements all NATS interfaces for testing.
// Use this when you need a mock that implements multiple interfaces.
// Prefer using specific mocks (MockFanout, MockSubscriber) when possible.
//
// Example usage:
//
//	mockBus := new(mocks.MockBus)
//	mockBus.On("PublishFanout", ctx, "subject", mock.Anything).Return(nil)
//	mockBus.On("Subscribe", mock.Anything).Return(func() error { return nil }, nil)
type MockBus struct {
	mock.Mock
}

// PublishFanout mocks the PublishFanout method.
func (m *MockBus) PublishFanout(ctx context.Context, subject string, data []byte) error {
	args := m.Called(ctx, subject, data)
	return args.Error(0)
}

// SubscribeFanout mocks the SubscribeFanout method.
func (m *MockBus) SubscribeFanout(stream, serviceName, subject string, handler natsModel.FanoutHandler) (*natsModel.FanoutSubscription, error) {
	args := m.Called(stream, serviceName, subject, handler)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*natsModel.FanoutSubscription), args.Error(1)
}

// EnsureFanoutStream mocks the EnsureFanoutStream method.
func (m *MockBus) EnsureFanoutStream(config natsModel.FanoutStreamConfig) error {
	args := m.Called(config)
	return args.Error(0)
}

// Subscribe mocks the Subscribe method.
func (m *MockBus) Subscribe(config natsModel.SubscribeConfig) (func() error, error) {
	args := m.Called(config)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(func() error), args.Error(1)
}

// Publish mocks the Publish method.
func (m *MockBus) Publish(config natsModel.PublishConfig) error {
	args := m.Called(config)
	return args.Error(0)
}

// Fetch mocks the Fetch method.
func (m *MockBus) Fetch(config natsModel.FetchConfig) (func() error, error) {
	args := m.Called(config)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(func() error), args.Error(1)
}

// GetConn mocks the GetConn method.
func (m *MockBus) GetConn() *natsgo.Conn {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*natsgo.Conn)
}

// PublishCore mocks the PublishCore method.
func (m *MockBus) PublishCore(config natsModel.PublishCoreConfig) error {
	args := m.Called(config)
	return args.Error(0)
}

// FlushConnection mocks the FlushConnection method.
func (m *MockBus) FlushConnection() error {
	args := m.Called()
	return args.Error(0)
}

// Compile-time checks for MockBus
var _ natsModel.Fanout = (*MockBus)(nil)
var _ natsModel.Subscriber = (*MockBus)(nil)
var _ natsModel.Publisher = (*MockBus)(nil)
var _ natsModel.Fetcher = (*MockBus)(nil)
var _ natsModel.ConnectionProvider = (*MockBus)(nil)
var _ natsModel.CorePublisher = (*MockBus)(nil)

// ============================================================================
//                        MOCK KEY VALUE STORE
// ============================================================================

// MockKeyValueStore implements natsModel.KeyValueStore interface for testing.
// Use this mock for services that use NATS KV for state persistence (e.g., workflow runtime).
//
// Example usage:
//
//	mockKV := new(mocks.MockKeyValueStore)
//	mockKV.On("Get", "inst:123").Return(&natsModel.KVEntry{Value: data, Revision: 5}, nil)
//	mockKV.On("Update", "inst:123", mock.Anything, uint64(5)).Return(uint64(6), nil)
type MockKeyValueStore struct {
	mock.Mock
}

// Get mocks the Get method.
func (m *MockKeyValueStore) Get(key string) (*natsModel.KVEntry, error) {
	args := m.Called(key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*natsModel.KVEntry), args.Error(1)
}

// Put mocks the Put method.
func (m *MockKeyValueStore) Put(key string, value []byte) (uint64, error) {
	args := m.Called(key, value)
	return args.Get(0).(uint64), args.Error(1)
}

// Create mocks the Create method.
func (m *MockKeyValueStore) Create(key string, value []byte) (uint64, error) {
	args := m.Called(key, value)
	return args.Get(0).(uint64), args.Error(1)
}

// Update mocks the Update method (CAS).
func (m *MockKeyValueStore) Update(key string, value []byte, expectedRevision uint64) (uint64, error) {
	args := m.Called(key, value, expectedRevision)
	return args.Get(0).(uint64), args.Error(1)
}

// Delete mocks the Delete method.
func (m *MockKeyValueStore) Delete(key string) error {
	args := m.Called(key)
	return args.Error(0)
}

// Purge mocks the Purge method.
func (m *MockKeyValueStore) Purge(key string) error {
	args := m.Called(key)
	return args.Error(0)
}

// Keys mocks the Keys method.
func (m *MockKeyValueStore) Keys() ([]string, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

// Bucket mocks the Bucket method.
func (m *MockKeyValueStore) Bucket() string {
	args := m.Called()
	return args.String(0)
}

// Compile-time check
var _ natsModel.KeyValueStore = (*MockKeyValueStore)(nil)
