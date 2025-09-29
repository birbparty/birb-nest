package instance

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestNewContext(t *testing.T) {
	instanceID := "test-instance-123"
	ctx := NewContext(instanceID)

	if ctx.InstanceID != instanceID {
		t.Errorf("expected InstanceID %s, got %s", instanceID, ctx.InstanceID)
	}

	if ctx.Status != StatusActive {
		t.Errorf("expected status %s, got %s", StatusActive, ctx.Status)
	}

	if ctx.GameType != "default" {
		t.Errorf("expected GameType 'default', got %s", ctx.GameType)
	}

	if ctx.Region != "default" {
		t.Errorf("expected Region 'default', got %s", ctx.Region)
	}

	if ctx.Metadata == nil {
		t.Error("expected Metadata to be initialized")
	}

	if ctx.ResourceQuota == nil {
		t.Error("expected ResourceQuota to be initialized")
	}
}

func TestContext_IsActive(t *testing.T) {
	tests := []struct {
		name   string
		status InstanceStatus
		want   bool
	}{
		{"active", StatusActive, true},
		{"inactive", StatusInactive, false},
		{"migrating", StatusMigrating, false},
		{"deleting", StatusDeleting, false},
		{"paused", StatusPaused, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &Context{Status: tt.status}
			if got := ctx.IsActive(); got != tt.want {
				t.Errorf("IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContext_CanAcceptRequests(t *testing.T) {
	tests := []struct {
		name   string
		status InstanceStatus
		want   bool
	}{
		{"active", StatusActive, true},
		{"inactive", StatusInactive, false},
		{"migrating", StatusMigrating, true},
		{"deleting", StatusDeleting, false},
		{"paused", StatusPaused, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &Context{Status: tt.status}
			if got := ctx.CanAcceptRequests(); got != tt.want {
				t.Errorf("CanAcceptRequests() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContext_UpdateLastActive(t *testing.T) {
	ctx := NewContext("test-instance")
	initialTime := ctx.LastActive

	// Sleep to ensure time difference
	time.Sleep(10 * time.Millisecond)
	ctx.UpdateLastActive()

	if !ctx.LastActive.After(initialTime) {
		t.Error("expected LastActive to be updated")
	}
}

func TestContext_Clone(t *testing.T) {
	original := NewContext("test-instance")
	original.GameType = "mmorpg"
	original.Region = "us-east-1"
	original.Status = StatusMigrating
	original.Metadata["custom"] = "value"
	original.ResourceQuota.MaxMemoryMB = 16384

	cloned := original.Clone()

	// Verify clone is equal
	if cloned.InstanceID != original.InstanceID {
		t.Error("InstanceID not cloned correctly")
	}
	if cloned.GameType != original.GameType {
		t.Error("GameType not cloned correctly")
	}
	if cloned.Region != original.Region {
		t.Error("Region not cloned correctly")
	}
	if cloned.Status != original.Status {
		t.Error("Status not cloned correctly")
	}
	if cloned.Metadata["custom"] != "value" {
		t.Error("Metadata not cloned correctly")
	}
	if cloned.ResourceQuota.MaxMemoryMB != 16384 {
		t.Error("ResourceQuota not cloned correctly")
	}

	// Verify independence
	cloned.Metadata["custom"] = "changed"
	if original.Metadata["custom"] != "value" {
		t.Error("Clone modification affected original")
	}
}

func TestContext_MarshalUnmarshal(t *testing.T) {
	original := NewContext("test-instance")
	original.GameType = "mmorpg"
	original.Region = "us-west-2"
	original.Metadata["version"] = "1.0.0"

	// Marshal
	data, err := original.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary failed: %v", err)
	}

	// Unmarshal
	restored := &Context{}
	if err := restored.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}

	// Verify
	if restored.InstanceID != original.InstanceID {
		t.Error("InstanceID not restored correctly")
	}
	if restored.GameType != original.GameType {
		t.Error("GameType not restored correctly")
	}
	if restored.Region != original.Region {
		t.Error("Region not restored correctly")
	}
	if restored.Metadata["version"] != "1.0.0" {
		t.Error("Metadata not restored correctly")
	}
}

func TestContext_Validate(t *testing.T) {
	tests := []struct {
		name      string
		ctx       *Context
		wantError bool
	}{
		{
			name:      "valid context",
			ctx:       NewContext("test-instance"),
			wantError: false,
		},
		{
			name:      "empty instance ID",
			ctx:       &Context{InstanceID: ""},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ctx.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestDefaultResourceQuota(t *testing.T) {
	quota := DefaultResourceQuota()

	if quota.MaxMemoryMB != 8192 {
		t.Errorf("expected MaxMemoryMB 8192, got %d", quota.MaxMemoryMB)
	}
	if quota.MaxStorageGB != 100 {
		t.Errorf("expected MaxStorageGB 100, got %d", quota.MaxStorageGB)
	}
	if quota.MaxCPUCores != 4 {
		t.Errorf("expected MaxCPUCores 4, got %d", quota.MaxCPUCores)
	}
	if quota.MaxConcurrent != 10000 {
		t.Errorf("expected MaxConcurrent 10000, got %d", quota.MaxConcurrent)
	}
}

func TestInjectExtractContext(t *testing.T) {
	instCtx := NewContext("test-instance")
	ctx := context.Background()

	// Inject
	ctx = InjectContext(ctx, instCtx)

	// Extract
	extracted, ok := ExtractContext(ctx)
	if !ok {
		t.Fatal("failed to extract context")
	}

	if extracted.InstanceID != instCtx.InstanceID {
		t.Errorf("extracted InstanceID %s does not match original %s", extracted.InstanceID, instCtx.InstanceID)
	}
}

func TestExtractInstanceID(t *testing.T) {
	tests := []struct {
		name     string
		setupCtx func() context.Context
		wantID   string
	}{
		{
			name: "with instance context",
			setupCtx: func() context.Context {
				return InjectContext(context.Background(), NewContext("test-instance"))
			},
			wantID: "test-instance",
		},
		{
			name: "without instance context",
			setupCtx: func() context.Context {
				return context.Background()
			},
			wantID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx()
			if got := ExtractInstanceID(ctx); got != tt.wantID {
				t.Errorf("ExtractInstanceID() = %v, want %v", got, tt.wantID)
			}
		})
	}
}

func TestInstanceError(t *testing.T) {
	err := &InstanceError{Code: "TEST_ERROR", Message: "test error message"}
	if err.Error() != "test error message" {
		t.Errorf("Error() = %v, want %v", err.Error(), "test error message")
	}
}

func TestContext_JSONSerialization(t *testing.T) {
	ctx := NewContext("test-instance")
	ctx.GameType = "rpg"
	ctx.Region = "eu-west-1"
	ctx.Metadata["key"] = "value"

	// Marshal to JSON
	data, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// Unmarshal from JSON
	var restored Context
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// Verify fields
	if restored.InstanceID != ctx.InstanceID {
		t.Errorf("InstanceID mismatch: got %s, want %s", restored.InstanceID, ctx.InstanceID)
	}
	if restored.GameType != ctx.GameType {
		t.Errorf("GameType mismatch: got %s, want %s", restored.GameType, ctx.GameType)
	}
	if restored.Region != ctx.Region {
		t.Errorf("Region mismatch: got %s, want %s", restored.Region, ctx.Region)
	}
	if restored.Metadata["key"] != ctx.Metadata["key"] {
		t.Errorf("Metadata mismatch: got %s, want %s", restored.Metadata["key"], ctx.Metadata["key"])
	}
}
