package instance

import (
	"strings"
	"testing"
)

func TestNewKeyBuilder(t *testing.T) {
	tests := []struct {
		name       string
		instanceID string
		wantID     string
	}{
		{
			name:       "with instance ID",
			instanceID: "inst_1719432000_abc12345",
			wantID:     "inst_1719432000_abc12345",
		},
		{
			name:       "empty instance ID",
			instanceID: "",
			wantID:     "",
		},
		{
			name:       "whitespace trimming",
			instanceID: "  inst_123  ",
			wantID:     "inst_123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kb := NewKeyBuilder(tt.instanceID)
			if kb.InstanceID() != tt.wantID {
				t.Errorf("NewKeyBuilder() instanceID = %v, want %v", kb.InstanceID(), tt.wantID)
			}
		})
	}
}

func TestKeyBuilder_BuildKey(t *testing.T) {
	tests := []struct {
		name       string
		instanceID string
		components []string
		want       string
	}{
		{
			name:       "with instance - single component",
			instanceID: "inst_123",
			components: []string{"cache", "user123"},
			want:       "instance:inst_123:cache:user123",
		},
		{
			name:       "with instance - multiple components",
			instanceID: "inst_123",
			components: []string{"table", "users", "row", "456"},
			want:       "instance:inst_123:table:users:row:456",
		},
		{
			name:       "empty instance - backward compatibility",
			instanceID: "",
			components: []string{"cache", "user123"},
			want:       "cache:user123",
		},
		{
			name:       "webapi format instance ID",
			instanceID: "inst_1719432000_abc12345",
			components: []string{"index", "users_by_name", "john"},
			want:       "instance:inst_1719432000_abc12345:index:users_by_name:john",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kb := NewKeyBuilder(tt.instanceID)
			got := kb.BuildKey(tt.components...)
			if got != tt.want {
				t.Errorf("BuildKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeyBuilder_ParseKey(t *testing.T) {
	tests := []struct {
		name           string
		key            string
		wantInstanceID string
		wantComponents []string
	}{
		{
			name:           "instance key - simple",
			key:            "instance:inst_123:cache:user456",
			wantInstanceID: "inst_123",
			wantComponents: []string{"cache", "user456"},
		},
		{
			name:           "instance key - complex",
			key:            "instance:inst_1719432000_abc12345:table:users:row:789",
			wantInstanceID: "inst_1719432000_abc12345",
			wantComponents: []string{"table", "users", "row", "789"},
		},
		{
			name:           "non-instance key",
			key:            "cache:user123",
			wantInstanceID: "",
			wantComponents: []string{"cache", "user123"},
		},
		{
			name:           "empty key",
			key:            "",
			wantInstanceID: "",
			wantComponents: []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kb := NewKeyBuilder("")
			gotInstanceID, gotComponents := kb.ParseKey(tt.key)

			if gotInstanceID != tt.wantInstanceID {
				t.Errorf("ParseKey() instanceID = %v, want %v", gotInstanceID, tt.wantInstanceID)
			}

			if len(gotComponents) != len(tt.wantComponents) {
				t.Errorf("ParseKey() components length = %v, want %v", len(gotComponents), len(tt.wantComponents))
			} else {
				for i, comp := range gotComponents {
					if comp != tt.wantComponents[i] {
						t.Errorf("ParseKey() component[%d] = %v, want %v", i, comp, tt.wantComponents[i])
					}
				}
			}
		})
	}
}

func TestKeyBuilder_BuildPattern(t *testing.T) {
	tests := []struct {
		name       string
		instanceID string
		prefix     string
		want       string
	}{
		{
			name:       "with instance - no prefix",
			instanceID: "inst_123",
			prefix:     "",
			want:       "instance:inst_123:*",
		},
		{
			name:       "with instance - with prefix",
			instanceID: "inst_123",
			prefix:     "cache",
			want:       "instance:inst_123:cache*",
		},
		{
			name:       "empty instance - no prefix",
			instanceID: "",
			prefix:     "",
			want:       "*",
		},
		{
			name:       "empty instance - with prefix",
			instanceID: "",
			prefix:     "cache",
			want:       "cache*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kb := NewKeyBuilder(tt.instanceID)
			got := kb.BuildPattern(tt.prefix)
			if got != tt.want {
				t.Errorf("BuildPattern() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeyBuilder_IsInstanceKey(t *testing.T) {
	tests := []struct {
		name       string
		instanceID string
		key        string
		want       bool
	}{
		{
			name:       "matching instance key",
			instanceID: "inst_123",
			key:        "instance:inst_123:cache:data",
			want:       true,
		},
		{
			name:       "different instance key",
			instanceID: "inst_123",
			key:        "instance:inst_456:cache:data",
			want:       false,
		},
		{
			name:       "non-instance key with instance builder",
			instanceID: "inst_123",
			key:        "cache:data",
			want:       false,
		},
		{
			name:       "empty instance - non-instance key",
			instanceID: "",
			key:        "cache:data",
			want:       true,
		},
		{
			name:       "empty instance - instance key",
			instanceID: "",
			key:        "instance:inst_123:cache:data",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kb := NewKeyBuilder(tt.instanceID)
			got := kb.IsInstanceKey(tt.key)
			if got != tt.want {
				t.Errorf("IsInstanceKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeyBuilder_StripInstance(t *testing.T) {
	tests := []struct {
		name       string
		instanceID string
		key        string
		want       string
	}{
		{
			name:       "matching instance key",
			instanceID: "inst_123",
			key:        "instance:inst_123:cache:data",
			want:       "cache:data",
		},
		{
			name:       "different instance key",
			instanceID: "inst_123",
			key:        "instance:inst_456:cache:data",
			want:       "instance:inst_456:cache:data",
		},
		{
			name:       "non-instance key",
			instanceID: "inst_123",
			key:        "cache:data",
			want:       "cache:data",
		},
		{
			name:       "empty instance",
			instanceID: "",
			key:        "instance:inst_123:cache:data",
			want:       "instance:inst_123:cache:data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kb := NewKeyBuilder(tt.instanceID)
			got := kb.StripInstance(tt.key)
			if got != tt.want {
				t.Errorf("StripInstance() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeyBuilder_Helpers(t *testing.T) {
	kb := NewKeyBuilder("inst_123")

	tests := []struct {
		name string
		fn   func() string
		want string
	}{
		{
			name: "TableKey",
			fn:   func() string { return kb.TableKey("users", "456") },
			want: "instance:inst_123:table:users:row:456",
		},
		{
			name: "IndexKey",
			fn:   func() string { return kb.IndexKey("users_by_name", "john") },
			want: "instance:inst_123:index:users_by_name:john",
		},
		{
			name: "CacheKey",
			fn:   func() string { return kb.CacheKey("session", "user123") },
			want: "instance:inst_123:cache:session:user123",
		},
		{
			name: "SchemaKey",
			fn:   func() string { return kb.SchemaKey("users") },
			want: "instance:inst_123:schema:users",
		},
		{
			name: "EventLogKey",
			fn:   func() string { return kb.EventLogKey("1234567890") },
			want: "instance:inst_123:eventlog:1234567890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn()
			if got != tt.want {
				t.Errorf("%s() = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestKeyBuilder_BackwardCompatibility(t *testing.T) {
	// Test that empty instance ID maintains backward compatibility
	kb := NewKeyBuilder("")

	// Should not add instance prefix
	key := kb.BuildKey("cache", "user123")
	if strings.HasPrefix(key, "instance:") {
		t.Errorf("Empty instance should not add prefix, got: %v", key)
	}

	// Should accept non-instance keys
	if !kb.IsInstanceKey("cache:user123") {
		t.Error("Empty instance should accept non-instance keys")
	}

	// Should reject instance-prefixed keys
	if kb.IsInstanceKey("instance:inst_123:cache:user123") {
		t.Error("Empty instance should reject instance-prefixed keys")
	}
}

func TestKeyBuilder_RealWorldScenarios(t *testing.T) {
	// Test with webapi-style instance IDs
	tests := []struct {
		name       string
		instanceID string
		scenario   string
		expected   string
	}{
		{
			name:       "overworld instance",
			instanceID: "inst_1719432000_abc12345",
			scenario:   "player data cache",
			expected:   "instance:inst_1719432000_abc12345:cache:player:p123",
		},
		{
			name:       "dungeon instance",
			instanceID: "inst_1719432100_xyz98765",
			scenario:   "monster spawn table",
			expected:   "instance:inst_1719432100_xyz98765:table:monster_spawns:row:m456",
		},
		{
			name:       "cross-instance query protection",
			instanceID: "inst_1719432000_abc12345",
			scenario:   "trying to access another instance",
			expected:   "false", // IsInstanceKey should return false
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kb := NewKeyBuilder(tt.instanceID)

			switch tt.scenario {
			case "player data cache":
				got := kb.CacheKey("player", "p123")
				if got != tt.expected {
					t.Errorf("Player cache key = %v, want %v", got, tt.expected)
				}
			case "monster spawn table":
				got := kb.TableKey("monster_spawns", "m456")
				if got != tt.expected {
					t.Errorf("Monster table key = %v, want %v", got, tt.expected)
				}
			case "trying to access another instance":
				otherKey := "instance:inst_9999999999_other:cache:data"
				got := kb.IsInstanceKey(otherKey)
				if got {
					t.Error("Should not allow access to other instance keys")
				}
			}
		})
	}
}
