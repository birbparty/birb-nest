package api

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockDatabase struct {
	mock.Mock
}

func (m *MockDatabase) Get(ctx context.Context, key string) ([]byte, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockDatabase) Set(ctx context.Context, key string, value []byte) error {
	args := m.Called(ctx, key, value)
	return args.Error(0)
}

func (m *MockDatabase) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockDatabase) Exists(ctx context.Context, key string) (bool, error) {
	args := m.Called(ctx, key)
	return args.Bool(0), args.Error(1)
}

func (m *MockDatabase) Health(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockDatabase) Close() error {
	args := m.Called()
	return args.Error(0)
}

// Instance-aware methods

func (m *MockDatabase) GetWithInstance(ctx context.Context, key, instanceID string) ([]byte, error) {
	args := m.Called(ctx, key, instanceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockDatabase) SetWithInstance(ctx context.Context, key, instanceID string, value []byte) error {
	args := m.Called(ctx, key, instanceID, value)
	return args.Error(0)
}

func (m *MockDatabase) DeleteWithInstance(ctx context.Context, key, instanceID string) error {
	args := m.Called(ctx, key, instanceID)
	return args.Error(0)
}

func (m *MockDatabase) ExistsWithInstance(ctx context.Context, key, instanceID string) (bool, error) {
	args := m.Called(ctx, key, instanceID)
	return args.Bool(0), args.Error(1)
}

// Context-aware methods

func (m *MockDatabase) GetFromContext(ctx context.Context, key string) ([]byte, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockDatabase) SetFromContext(ctx context.Context, key string, value []byte) error {
	args := m.Called(ctx, key, value)
	return args.Error(0)
}

func (m *MockDatabase) DeleteFromContext(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockDatabase) ExistsFromContext(ctx context.Context, key string) (bool, error) {
	args := m.Called(ctx, key)
	return args.Bool(0), args.Error(1)
}

func TestAsyncWriter_Write(t *testing.T) {
	mockDB := new(MockDatabase)
	writer := NewAsyncWriter(mockDB, 100, 1)
	defer writer.Shutdown()

	// Setup expectation
	mockDB.On("SetWithInstance", mock.Anything, "test-key", "primary", []byte("test-value")).Return(nil)

	// Write
	writer.Write("test-key", []byte("test-value"), "primary")

	// Wait for async processing
	time.Sleep(100 * time.Millisecond)

	// Verify
	mockDB.AssertExpectations(t)
}

func TestAsyncWriter_QueueFull(t *testing.T) {
	mockDB := new(MockDatabase)
	writer := NewAsyncWriter(mockDB, 1, 0) // Small queue, no workers
	defer writer.Shutdown()

	// Fill the queue
	writer.Write("key1", []byte("value1"), "primary")
	writer.Write("key2", []byte("value2"), "primary") // Should be dropped

	// No database calls expected since no workers
	mockDB.AssertNotCalled(t, "SetWithInstance")
}

func TestAsyncWriter_Stats(t *testing.T) {
	mockDB := new(MockDatabase)
	writer := NewAsyncWriter(mockDB, 100, 5)
	defer writer.Shutdown()

	stats := writer.Stats()
	assert.Equal(t, 100, stats.QueueCapacity)
	assert.Equal(t, 5, stats.WorkerCount)
	assert.Equal(t, 0, stats.QueueDepth)
}

func TestAsyncWriter_Retry(t *testing.T) {
	mockDB := new(MockDatabase)
	writer := NewAsyncWriter(mockDB, 100, 1)
	defer writer.Shutdown()

	// First call fails, second succeeds
	mockDB.On("SetWithInstance", mock.Anything, "retry-key", "primary", []byte("retry-value")).Return(assert.AnError).Once()
	mockDB.On("SetWithInstance", mock.Anything, "retry-key", "primary", []byte("retry-value")).Return(nil).Once()

	// Write
	writer.Write("retry-key", []byte("retry-value"), "primary")

	// Wait for async processing and retry
	time.Sleep(2 * time.Second)

	// Verify both calls were made
	mockDB.AssertExpectations(t)
	mockDB.AssertNumberOfCalls(t, "SetWithInstance", 2)
}
