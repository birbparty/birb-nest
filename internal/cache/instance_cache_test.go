package cache

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// mockCache is a mock implementation of the Cache interface for testing
type mockCache struct {
	data   map[string][]byte
	closed bool
}

func newMockCache() *mockCache {
	return &mockCache{
		data: make(map[string][]byte),
	}
}

func (m *mockCache) Get(ctx context.Context, key string) ([]byte, error) {
	if m.closed {
		return nil, ErrCacheClosed
	}
	value, ok := m.data[key]
	if !ok {
		return nil, ErrKeyNotFound
	}
	return value, nil
}

func (m *mockCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if m.closed {
		return ErrCacheClosed
	}
	m.data[key] = value
	return nil
}

func (m *mockCache) Delete(ctx context.Context, key string) error {
	if m.closed {
		return ErrCacheClosed
	}
	delete(m.data, key)
	return nil
}

func (m *mockCache) Exists(ctx context.Context, key string) (bool, error) {
	if m.closed {
		return false, ErrCacheClosed
	}
	_, ok := m.data[key]
	return ok, nil
}

func (m *mockCache) GetMultiple(ctx context.Context, keys []string) (map[string][]byte, error) {
	if m.closed {
		return nil, ErrCacheClosed
	}
	results := make(map[string][]byte)
	for _, key := range keys {
		if value, ok := m.data[key]; ok {
			results[key] = value
		}
	}
	return results, nil
}

func (m *mockCache) SetMultiple(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	if m.closed {
		return ErrCacheClosed
	}
	for key, value := range items {
		m.data[key] = value
	}
	return nil
}

func (m *mockCache) DeleteMultiple(ctx context.Context, keys []string) error {
	if m.closed {
		return ErrCacheClosed
	}
	for _, key := range keys {
		delete(m.data, key)
	}
	return nil
}

func (m *mockCache) Ping(ctx context.Context) error {
	if m.closed {
		return ErrCacheClosed
	}
	return nil
}

func (m *mockCache) Close() error {
	m.closed = true
	return nil
}

func TestInstanceCache_BasicOperations(t *testing.T) {
	ctx := context.Background()
	mock := newMockCache()
	instanceID := "inst_1719432000_abc12345"
	ic := NewInstanceCache(mock, instanceID)

	// Test Set and Get
	t.Run("Set and Get", func(t *testing.T) {
		key := "testkey"
		value := []byte("testvalue")

		// Set value
		if err := ic.Set(ctx, key, value, 0); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		// Verify key was transformed
		expectedKey := fmt.Sprintf("instance:%s:cache:%s", instanceID, key)
		if _, ok := mock.data[expectedKey]; !ok {
			t.Errorf("Expected key %s not found in mock cache", expectedKey)
		}

		// Get value
		got, err := ic.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if string(got) != string(value) {
			t.Errorf("Get() = %v, want %v", string(got), string(value))
		}
	})

	// Test Delete
	t.Run("Delete", func(t *testing.T) {
		key := "deletekey"
		value := []byte("deletevalue")

		// Set value first
		ic.Set(ctx, key, value, 0)

		// Delete
		if err := ic.Delete(ctx, key); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		// Verify it's gone
		_, err := ic.Get(ctx, key)
		if err != ErrKeyNotFound {
			t.Errorf("Get() after Delete() error = %v, want %v", err, ErrKeyNotFound)
		}
	})

	// Test Exists
	t.Run("Exists", func(t *testing.T) {
		key := "existskey"
		value := []byte("existsvalue")

		// Check non-existent key
		exists, err := ic.Exists(ctx, key)
		if err != nil {
			t.Fatalf("Exists() error = %v", err)
		}
		if exists {
			t.Error("Exists() = true for non-existent key")
		}

		// Set key
		ic.Set(ctx, key, value, 0)

		// Check existing key
		exists, err = ic.Exists(ctx, key)
		if err != nil {
			t.Fatalf("Exists() error = %v", err)
		}
		if !exists {
			t.Error("Exists() = false for existing key")
		}
	})
}

func TestInstanceCache_MultipleOperations(t *testing.T) {
	ctx := context.Background()
	mock := newMockCache()
	instanceID := "inst_123"
	ic := NewInstanceCache(mock, instanceID)

	// Test SetMultiple and GetMultiple
	t.Run("SetMultiple and GetMultiple", func(t *testing.T) {
		items := map[string][]byte{
			"key1": []byte("value1"),
			"key2": []byte("value2"),
			"key3": []byte("value3"),
		}

		// Set multiple
		if err := ic.SetMultiple(ctx, items, 0); err != nil {
			t.Fatalf("SetMultiple() error = %v", err)
		}

		// Get multiple
		keys := []string{"key1", "key2", "key3", "key4"} // key4 doesn't exist
		results, err := ic.GetMultiple(ctx, keys)
		if err != nil {
			t.Fatalf("GetMultiple() error = %v", err)
		}

		// Verify results
		if len(results) != 3 {
			t.Errorf("GetMultiple() returned %d results, want 3", len(results))
		}

		for key, expectedValue := range items {
			if gotValue, ok := results[key]; !ok {
				t.Errorf("GetMultiple() missing key %s", key)
			} else if string(gotValue) != string(expectedValue) {
				t.Errorf("GetMultiple() key %s = %v, want %v", key, string(gotValue), string(expectedValue))
			}
		}

		// Verify key4 is not in results
		if _, ok := results["key4"]; ok {
			t.Error("GetMultiple() returned non-existent key4")
		}
	})

	// Test DeleteMultiple
	t.Run("DeleteMultiple", func(t *testing.T) {
		// Set up some keys
		items := map[string][]byte{
			"del1": []byte("value1"),
			"del2": []byte("value2"),
			"del3": []byte("value3"),
		}
		ic.SetMultiple(ctx, items, 0)

		// Delete multiple
		deleteKeys := []string{"del1", "del2"}
		if err := ic.DeleteMultiple(ctx, deleteKeys); err != nil {
			t.Fatalf("DeleteMultiple() error = %v", err)
		}

		// Verify del1 and del2 are gone
		for _, key := range deleteKeys {
			exists, _ := ic.Exists(ctx, key)
			if exists {
				t.Errorf("Key %s still exists after DeleteMultiple()", key)
			}
		}

		// Verify del3 still exists
		exists, _ := ic.Exists(ctx, "del3")
		if !exists {
			t.Error("Key del3 was deleted but shouldn't have been")
		}
	})
}

func TestInstanceCache_EmptyInstance(t *testing.T) {
	ctx := context.Background()
	mock := newMockCache()
	ic := NewInstanceCache(mock, "") // Empty instance ID

	// Test that keys are not prefixed
	t.Run("No prefix for empty instance", func(t *testing.T) {
		key := "testkey"
		value := []byte("testvalue")

		// Set value
		if err := ic.Set(ctx, key, value, 0); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		// Verify key was NOT transformed (should be cache:testkey)
		expectedKey := "cache:testkey"
		if _, ok := mock.data[expectedKey]; !ok {
			t.Errorf("Expected key %s not found in mock cache", expectedKey)
		}

		// Verify instance prefix was NOT added
		instanceKey := fmt.Sprintf("instance::cache:%s", key)
		if _, ok := mock.data[instanceKey]; ok {
			t.Error("Found instance-prefixed key when instance ID is empty")
		}
	})
}

func TestInstanceCache_UtilityMethods(t *testing.T) {
	mock := newMockCache()
	instanceID := "inst_456"
	ic := NewInstanceCache(mock, instanceID)

	t.Run("GetInstanceID", func(t *testing.T) {
		if got := ic.GetInstanceID(); got != instanceID {
			t.Errorf("GetInstanceID() = %v, want %v", got, instanceID)
		}
	})

	t.Run("HasInstance", func(t *testing.T) {
		if !ic.HasInstance() {
			t.Error("HasInstance() = false, want true")
		}

		// Test with empty instance
		icEmpty := NewInstanceCache(mock, "")
		if icEmpty.HasInstance() {
			t.Error("HasInstance() = true for empty instance, want false")
		}
	})

	t.Run("KeyBuilder", func(t *testing.T) {
		kb := ic.KeyBuilder()
		if kb == nil {
			t.Fatal("KeyBuilder() returned nil")
		}
		if kb.InstanceID() != instanceID {
			t.Errorf("KeyBuilder().InstanceID() = %v, want %v", kb.InstanceID(), instanceID)
		}
	})
}

func TestInstanceCache_ConnectionMethods(t *testing.T) {
	ctx := context.Background()
	mock := newMockCache()
	ic := NewInstanceCache(mock, "inst_789")

	t.Run("Ping", func(t *testing.T) {
		if err := ic.Ping(ctx); err != nil {
			t.Errorf("Ping() error = %v", err)
		}
	})

	t.Run("Close", func(t *testing.T) {
		if err := ic.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}

		// Verify operations fail after close
		if err := ic.Ping(ctx); err != ErrCacheClosed {
			t.Errorf("Ping() after Close() error = %v, want %v", err, ErrCacheClosed)
		}
	})
}

func TestInstanceCache_IsolationBetweenInstances(t *testing.T) {
	ctx := context.Background()
	mock := newMockCache()

	// Create two instance caches with different instance IDs
	ic1 := NewInstanceCache(mock, "inst_game1")
	ic2 := NewInstanceCache(mock, "inst_game2")

	// Set same key in both instances
	key := "player:123"
	value1 := []byte("game1_data")
	value2 := []byte("game2_data")

	if err := ic1.Set(ctx, key, value1, 0); err != nil {
		t.Fatalf("ic1.Set() error = %v", err)
	}
	if err := ic2.Set(ctx, key, value2, 0); err != nil {
		t.Fatalf("ic2.Set() error = %v", err)
	}

	// Verify each instance gets its own data
	got1, err := ic1.Get(ctx, key)
	if err != nil {
		t.Fatalf("ic1.Get() error = %v", err)
	}
	if string(got1) != string(value1) {
		t.Errorf("ic1.Get() = %v, want %v", string(got1), string(value1))
	}

	got2, err := ic2.Get(ctx, key)
	if err != nil {
		t.Fatalf("ic2.Get() error = %v", err)
	}
	if string(got2) != string(value2) {
		t.Errorf("ic2.Get() = %v, want %v", string(got2), string(value2))
	}

	// Verify the actual keys in mock cache are different
	key1 := fmt.Sprintf("instance:inst_game1:cache:%s", key)
	key2 := fmt.Sprintf("instance:inst_game2:cache:%s", key)

	if _, ok := mock.data[key1]; !ok {
		t.Errorf("Key %s not found in mock cache", key1)
	}
	if _, ok := mock.data[key2]; !ok {
		t.Errorf("Key %s not found in mock cache", key2)
	}

	// Delete from one instance shouldn't affect the other
	if err := ic1.Delete(ctx, key); err != nil {
		t.Fatalf("ic1.Delete() error = %v", err)
	}

	// ic2 should still have its data
	exists, err := ic2.Exists(ctx, key)
	if err != nil {
		t.Fatalf("ic2.Exists() error = %v", err)
	}
	if !exists {
		t.Error("ic2 data was deleted when ic1 deleted its key")
	}
}

func TestInstanceCache_RealWorldScenarios(t *testing.T) {
	ctx := context.Background()
	mock := newMockCache()

	// Scenario: WebAPI creates instances for different game sessions
	t.Run("Game session isolation", func(t *testing.T) {
		// Overworld instance
		overworldCache := NewInstanceCache(mock, "inst_1719432000_overworld")

		// Dungeon instance spawned from overworld
		dungeonCache := NewInstanceCache(mock, "inst_1719432100_dungeon01")

		// Player data in overworld
		overworldCache.Set(ctx, "player:p123:position", []byte(`{"x":100,"y":200,"z":50}`), 0)
		overworldCache.Set(ctx, "player:p123:inventory", []byte(`{"gold":500,"items":["sword","shield"]}`), 0)

		// Player enters dungeon - different position and temporary inventory
		dungeonCache.Set(ctx, "player:p123:position", []byte(`{"x":0,"y":0,"z":0}`), 0)
		dungeonCache.Set(ctx, "player:p123:inventory", []byte(`{"gold":500,"items":["sword","shield","torch"]}`), 0)

		// Verify data isolation
		overworldPos, _ := overworldCache.Get(ctx, "player:p123:position")
		dungeonPos, _ := dungeonCache.Get(ctx, "player:p123:position")

		if string(overworldPos) == string(dungeonPos) {
			t.Error("Position data not isolated between instances")
		}

		// Player leaves dungeon - dungeon instance can be cleaned up
		dungeonCache.DeleteMultiple(ctx, []string{"player:p123:position", "player:p123:inventory"})

		// Overworld data should be intact
		overworldPosAfter, err := overworldCache.Get(ctx, "player:p123:position")
		if err != nil {
			t.Errorf("Overworld data lost: %v", err)
		}
		if string(overworldPosAfter) != string(overworldPos) {
			t.Error("Overworld data changed after dungeon cleanup")
		}
	})
}
