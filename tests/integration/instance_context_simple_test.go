package integration

import (
	"context"
	"testing"
	"time"

	"github.com/birbparty/birb-nest/internal/instance"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInstanceContextSimple tests basic instance context functionality
func TestInstanceContextSimple(t *testing.T) {
	// Skip if no test infrastructure
	if testCache == nil {
		t.Skip("Test infrastructure not initialized")
	}

	ctx := context.Background()
	registry := instance.NewRegistry(testCache)

	// Test basic registration and retrieval
	inst := &instance.Context{
		InstanceID: "test-inst-1",
		GameType:   "minecraft",
		Region:     "us-east-1",
		Status:     instance.StatusActive,
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
		Metadata: map[string]string{
			"owner": "test-user",
		},
	}

	// Register
	err := registry.Register(ctx, inst)
	require.NoError(t, err)

	// Get
	retrieved, err := registry.Get(ctx, "test-inst-1")
	require.NoError(t, err)
	assert.Equal(t, inst.InstanceID, retrieved.InstanceID)
	assert.Equal(t, inst.GameType, retrieved.GameType)

	// Update
	retrieved.Status = instance.StatusPaused
	err = registry.Update(ctx, retrieved)
	require.NoError(t, err)

	// Verify update
	updated, err := registry.Get(ctx, "test-inst-1")
	require.NoError(t, err)
	assert.Equal(t, instance.StatusPaused, updated.Status)

	// List
	all, err := registry.List(ctx, instance.ListFilter{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(all), 1)

	// Delete
	err = registry.Delete(ctx, "test-inst-1")
	require.NoError(t, err)

	// Verify deleted
	_, err = registry.Get(ctx, "test-inst-1")
	assert.Equal(t, instance.ErrInstanceNotFound, err)
}

// TestInstanceDataIsolation tests data isolation between instances
func TestInstanceDataIsolation(t *testing.T) {
	// Skip if no test infrastructure
	if testCache == nil {
		t.Skip("Test infrastructure not initialized")
	}

	ctx := context.Background()

	// Create two instances with different instance IDs
	kb1 := instance.NewKeyBuilder("instance-1")
	kb2 := instance.NewKeyBuilder("instance-2")

	// Set same key for both instances
	key := "shared-key"
	value1 := []byte("instance-1-data")
	value2 := []byte("instance-2-data")

	// Store data for instance 1
	err := testCache.Set(ctx, kb1.CacheKey(key), value1, 0)
	require.NoError(t, err)

	// Store data for instance 2
	err = testCache.Set(ctx, kb2.CacheKey(key), value2, 0)
	require.NoError(t, err)

	// Retrieve data for instance 1
	retrieved1, err := testCache.Get(ctx, kb1.CacheKey(key))
	require.NoError(t, err)
	assert.Equal(t, value1, retrieved1)

	// Retrieve data for instance 2
	retrieved2, err := testCache.Get(ctx, kb2.CacheKey(key))
	require.NoError(t, err)
	assert.Equal(t, value2, retrieved2)

	// Ensure they are different
	assert.NotEqual(t, retrieved1, retrieved2)
}
