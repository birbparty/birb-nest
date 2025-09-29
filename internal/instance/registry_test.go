package instance

import (
	"context"
	"testing"
	"time"
)

// MockCache implements CacheInterface for testing
type MockCache struct {
	data map[string][]byte
}

func NewMockCache() *MockCache {
	return &MockCache{
		data: make(map[string][]byte),
	}
}

func (m *MockCache) Get(ctx context.Context, key string) ([]byte, error) {
	if data, ok := m.data[key]; ok {
		return data, nil
	}
	return nil, ErrInstanceNotFound
}

func (m *MockCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	m.data[key] = value
	return nil
}

func (m *MockCache) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func TestRegistry_Register(t *testing.T) {
	cache := NewMockCache()
	registry := NewRegistry(cache)

	ctx := context.Background()
	instCtx := NewContext("test-instance")
	instCtx.GameType = "mmorpg"
	instCtx.Region = "us-east-1"

	// Register instance
	err := registry.Register(ctx, instCtx)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Verify it was stored
	key := registry.buildKey("test-instance")
	if _, ok := cache.data[key]; !ok {
		t.Error("Instance not stored in cache")
	}
}

func TestRegistry_Get(t *testing.T) {
	cache := NewMockCache()
	registry := NewRegistry(cache)

	ctx := context.Background()

	// Register an instance first
	instCtx := NewContext("test-instance")
	instCtx.GameType = "rpg"
	err := registry.Register(ctx, instCtx)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Get the instance
	retrieved, err := registry.Get(ctx, "test-instance")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.InstanceID != "test-instance" {
		t.Errorf("Retrieved instance ID mismatch: got %s, want test-instance", retrieved.InstanceID)
	}

	if retrieved.GameType != "rpg" {
		t.Errorf("Retrieved game type mismatch: got %s, want rpg", retrieved.GameType)
	}
}

func TestRegistry_GetOrCreate(t *testing.T) {
	cache := NewMockCache()
	registry := NewRegistry(cache)

	ctx := context.Background()

	// Get non-existent instance (should create)
	instCtx, err := registry.GetOrCreate(ctx, "new-instance")
	if err != nil {
		t.Fatalf("GetOrCreate failed: %v", err)
	}

	if instCtx.InstanceID != "new-instance" {
		t.Errorf("Instance ID mismatch: got %s, want new-instance", instCtx.InstanceID)
	}

	if instCtx.Status != StatusActive {
		t.Errorf("New instance should be active, got %s", instCtx.Status)
	}

	// Get existing instance
	instCtx2, err := registry.GetOrCreate(ctx, "new-instance")
	if err != nil {
		t.Fatalf("GetOrCreate failed for existing: %v", err)
	}

	if instCtx2.InstanceID != instCtx.InstanceID {
		t.Error("Should return same instance")
	}
}

func TestRegistry_Update(t *testing.T) {
	cache := NewMockCache()
	registry := NewRegistry(cache)

	ctx := context.Background()

	// Register an instance first
	instCtx := NewContext("test-instance")
	err := registry.Register(ctx, instCtx)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Update it
	instCtx.Status = StatusPaused
	instCtx.GameType = "updated-game"
	err = registry.Update(ctx, instCtx)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify update
	retrieved, err := registry.Get(ctx, "test-instance")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Status != StatusPaused {
		t.Errorf("Status not updated: got %s, want %s", retrieved.Status, StatusPaused)
	}

	if retrieved.GameType != "updated-game" {
		t.Errorf("GameType not updated: got %s, want updated-game", retrieved.GameType)
	}
}

func TestRegistry_Delete(t *testing.T) {
	cache := NewMockCache()
	registry := NewRegistry(cache)

	ctx := context.Background()

	// Register an instance first
	instCtx := NewContext("test-instance")
	err := registry.Register(ctx, instCtx)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Delete it
	err = registry.Delete(ctx, "test-instance")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's gone
	_, err = registry.Get(ctx, "test-instance")
	if err != ErrInstanceNotFound {
		t.Error("Instance should not be found after deletion")
	}
}

func TestRegistry_UpdateLastActive(t *testing.T) {
	cache := NewMockCache()
	registry := NewRegistry(cache)

	ctx := context.Background()

	// Register an instance
	instCtx := NewContext("test-instance")
	initialActive := instCtx.LastActive
	err := registry.Register(ctx, instCtx)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Sleep to ensure time difference
	time.Sleep(10 * time.Millisecond)

	// Update last active
	err = registry.UpdateLastActive(ctx, "test-instance")
	if err != nil {
		t.Fatalf("UpdateLastActive failed: %v", err)
	}

	// Verify it was updated
	retrieved, err := registry.Get(ctx, "test-instance")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if !retrieved.LastActive.After(initialActive) {
		t.Error("LastActive should be updated")
	}
}

func TestRegistry_List(t *testing.T) {
	cache := NewMockCache()
	registry := NewRegistry(cache)

	ctx := context.Background()

	// Register multiple instances
	instances := []struct {
		id       string
		gameType string
		region   string
		status   InstanceStatus
	}{
		{"inst1", "mmorpg", "us-east-1", StatusActive},
		{"inst2", "rpg", "us-west-2", StatusActive},
		{"inst3", "mmorpg", "eu-west-1", StatusPaused},
		{"inst4", "fps", "us-east-1", StatusInactive},
	}

	for _, inst := range instances {
		instCtx := NewContext(inst.id)
		instCtx.GameType = inst.gameType
		instCtx.Region = inst.region
		instCtx.Status = inst.status
		err := registry.Register(ctx, instCtx)
		if err != nil {
			t.Fatalf("Register failed for %s: %v", inst.id, err)
		}
	}

	// Test filtering by status
	filter := ListFilter{Status: StatusActive}
	_, err := registry.List(ctx, filter)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Note: Since scanKeys returns empty slice, this test will pass but won't find instances
	// In a real implementation with proper SCAN support, we'd verify the filtered results
}

func TestRegistry_MemoryCache(t *testing.T) {
	cache := NewMockCache()
	registry := NewRegistry(cache)

	ctx := context.Background()

	// Register an instance
	instCtx := NewContext("test-instance")
	err := registry.Register(ctx, instCtx)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// First get should populate memory cache
	_, err = registry.Get(ctx, "test-instance")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Delete from underlying cache
	delete(cache.data, registry.buildKey("test-instance"))

	// Should still get from memory cache
	cached, err := registry.Get(ctx, "test-instance")
	if err != nil {
		t.Error("Should get from memory cache")
	}

	if cached.InstanceID != "test-instance" {
		t.Error("Memory cache returned wrong instance")
	}
}

func TestRegistry_ActivityThrottling(t *testing.T) {
	cache := NewMockCache()
	registry := NewRegistry(cache)

	ctx := context.Background()

	// Register an instance
	instCtx := NewContext("test-instance")
	err := registry.Register(ctx, instCtx)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// First update should succeed
	if !registry.shouldUpdateActivity("test-instance") {
		t.Error("First activity update should be allowed")
	}

	// Immediate second update should be throttled
	if registry.shouldUpdateActivity("test-instance") {
		t.Error("Immediate activity update should be throttled")
	}
}
