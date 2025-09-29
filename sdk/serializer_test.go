package sdk

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestSerialize(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    string
		wantErr bool
	}{
		// Basic types
		{
			name:  "string",
			input: "hello world",
			want:  `"hello world"`,
		},
		{
			name:  "empty string",
			input: "",
			want:  `""`,
		},
		{
			name:  "unicode string",
			input: "Hello 世界 🌍",
			want:  `"Hello 世界 🌍"`,
		},
		{
			name:  "integer",
			input: 42,
			want:  `42`,
		},
		{
			name:  "zero integer",
			input: 0,
			want:  `0`,
		},
		{
			name:  "negative integer",
			input: -42,
			want:  `-42`,
		},
		{
			name:  "float",
			input: 3.14159,
			want:  `3.14159`,
		},
		{
			name:  "boolean true",
			input: true,
			want:  `true`,
		},
		{
			name:  "boolean false",
			input: false,
			want:  `false`,
		},
		{
			name:  "nil",
			input: nil,
			want:  `null`,
		},
		// Complex types
		{
			name:  "slice of strings",
			input: []string{"apple", "banana", "cherry"},
			want:  `["apple","banana","cherry"]`,
		},
		{
			name:  "slice of integers",
			input: []int{1, 2, 3, 4, 5},
			want:  `[1,2,3,4,5]`,
		},
		{
			name:  "empty slice",
			input: []string{},
			want:  `[]`,
		},
		{
			name:  "map string to string",
			input: map[string]string{"key": "value", "foo": "bar"},
			want:  `{"foo":"bar","key":"value"}`,
		},
		{
			name:  "empty map",
			input: map[string]string{},
			want:  `{}`,
		},
		{
			name: "struct",
			input: struct {
				Name  string `json:"name"`
				Value int    `json:"value"`
			}{Name: "test", Value: 42},
			want: `{"name":"test","value":42}`,
		},
		{
			name: "struct with omitempty",
			input: struct {
				Name  string `json:"name"`
				Value int    `json:"value,omitempty"`
			}{Name: "test", Value: 0},
			want: `{"name":"test"}`,
		},
		{
			name: "nested struct",
			input: struct {
				Level1 struct {
					Level2 string `json:"level2"`
				} `json:"level1"`
			}{
				Level1: struct {
					Level2 string `json:"level2"`
				}{
					Level2: "nested",
				},
			},
			want: `{"level1":{"level2":"nested"}}`,
		},
		// Special cases
		{
			name:  "json.RawMessage",
			input: json.RawMessage(`{"nested": "json", "count": 5}`),
			want:  `{"nested": "json", "count": 5}`,
		},
		{
			name:  "time.Time",
			input: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			want:  `"2023-01-01T12:00:00Z"`,
		},
		{
			name:  "byte slice",
			input: []byte("hello"),
			want:  `"aGVsbG8="`, // base64 encoded
		},
		// String that looks like JSON
		{
			name:  "string that looks like JSON object",
			input: `{"key": "value"}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "string that looks like JSON array",
			input: `[1, 2, 3]`,
			want:  `[1, 2, 3]`,
		},
		{
			name:  "string that is not valid JSON",
			input: `{invalid json}`,
			want:  `"{invalid json}"`,
		},
		// Error cases
		{
			name:    "channel (non-serializable)",
			input:   make(chan int),
			wantErr: true,
		},
		{
			name:    "function (non-serializable)",
			input:   func() {},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := serialize(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("serialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && string(got) != tt.want {
				t.Errorf("serialize() = %v, want %v", string(got), tt.want)
			}
		})
	}
}

func TestDeserialize(t *testing.T) {
	tests := []struct {
		name    string
		data    json.RawMessage
		target  interface{}
		want    interface{}
		wantErr bool
	}{
		// Basic types
		{
			name:   "string",
			data:   json.RawMessage(`"hello world"`),
			target: new(string),
			want:   "hello world",
		},
		{
			name:   "integer",
			data:   json.RawMessage(`42`),
			target: new(int),
			want:   42,
		},
		{
			name:   "float",
			data:   json.RawMessage(`3.14159`),
			target: new(float64),
			want:   3.14159,
		},
		{
			name:   "boolean",
			data:   json.RawMessage(`true`),
			target: new(bool),
			want:   true,
		},
		// Slices
		{
			name:   "slice of strings",
			data:   json.RawMessage(`["apple","banana","cherry"]`),
			target: new([]string),
			want:   []string{"apple", "banana", "cherry"},
		},
		// Maps
		{
			name:   "map",
			data:   json.RawMessage(`{"key":"value"}`),
			target: new(map[string]string),
			want:   map[string]string{"key": "value"},
		},
		// Structs
		{
			name: "struct",
			data: json.RawMessage(`{"name":"test","value":42}`),
			target: new(struct {
				Name  string `json:"name"`
				Value int    `json:"value"`
			}),
			want: struct {
				Name  string `json:"name"`
				Value int    `json:"value"`
			}{Name: "test", Value: 42},
		},
		// Special case: json.RawMessage target
		{
			name:   "json.RawMessage target",
			data:   json.RawMessage(`{"raw":"message"}`),
			target: new(json.RawMessage),
			want:   json.RawMessage(`{"raw":"message"}`),
		},
		// Error cases
		{
			name:    "empty data",
			data:    json.RawMessage(``),
			target:  new(string),
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			data:    json.RawMessage(`{invalid}`),
			target:  new(map[string]string),
			wantErr: true,
		},
		{
			name:    "type mismatch",
			data:    json.RawMessage(`"string"`),
			target:  new(int),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := deserialize(tt.data, tt.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("deserialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				// Dereference the pointer to compare values
				got := dereferenceValue(tt.target)
				if !compareJSON(t, toJSON(tt.want), toJSON(got)) {
					t.Errorf("deserialize() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestBuildCacheRequest(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		ttl      *time.Duration
		metadata map[string]interface{}
		wantTTL  *int
		wantErr  bool
	}{
		{
			name:  "simple value no TTL",
			value: "test-value",
		},
		{
			name:    "with TTL",
			value:   "test-value",
			ttl:     durationPtr(30 * time.Second),
			wantTTL: intPtr(30),
		},
		{
			name:    "with TTL rounding",
			value:   "test-value",
			ttl:     durationPtr(45*time.Second + 500*time.Millisecond),
			wantTTL: intPtr(45), // Should round down
		},
		{
			name:  "with metadata",
			value: "test-value",
			metadata: map[string]interface{}{
				"source": "test",
				"user":   "123",
			},
		},
		{
			name: "complex value with TTL and metadata",
			value: struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			}{ID: 1, Name: "Test"},
			ttl:      durationPtr(time.Hour),
			wantTTL:  intPtr(3600),
			metadata: map[string]interface{}{"type": "complex"},
		},
		{
			name:    "non-serializable value",
			value:   make(chan int),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := buildCacheRequest(tt.value, tt.ttl, tt.metadata)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildCacheRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				// Check TTL
				if tt.wantTTL != nil {
					if req.TTL == nil || *req.TTL != *tt.wantTTL {
						t.Errorf("buildCacheRequest() TTL = %v, want %v", req.TTL, tt.wantTTL)
					}
				} else if req.TTL != nil {
					t.Errorf("buildCacheRequest() TTL = %v, want nil", req.TTL)
				}

				// Check metadata
				if tt.metadata != nil {
					for k, v := range tt.metadata {
						if req.Metadata[k] != v {
							t.Errorf("buildCacheRequest() metadata[%s] = %v, want %v", k, req.Metadata[k], v)
						}
					}
				}

				// Verify the value can be deserialized back
				var decoded interface{}
				if err := json.Unmarshal(req.Value, &decoded); err != nil {
					t.Errorf("buildCacheRequest() produced invalid JSON: %v", err)
				}
			}
		})
	}
}

func TestParseAPIError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       []byte
		wantMsg    string
		wantCode   string
	}{
		{
			name:       "valid error response",
			statusCode: 404,
			body:       []byte(`{"error": "Not found", "code": "NOT_FOUND", "details": "Key does not exist"}`),
			wantMsg:    "Not found",
			wantCode:   "NOT_FOUND",
		},
		{
			name:       "error without code",
			statusCode: 500,
			body:       []byte(`{"error": "Internal server error"}`),
			wantMsg:    "Internal server error",
			wantCode:   "",
		},
		{
			name:       "invalid JSON",
			statusCode: 400,
			body:       []byte(`Invalid JSON response`),
			wantMsg:    "Invalid JSON response",
			wantCode:   "",
		},
		{
			name:       "empty body",
			statusCode: 503,
			body:       []byte{},
			wantMsg:    "HTTP 503 error",
			wantCode:   "",
		},
		{
			name:       "nested error structure",
			statusCode: 422,
			body:       []byte(`{"error": "Validation failed", "code": "VALIDATION_ERROR", "details": "Field 'name' is required"}`),
			wantMsg:    "Validation failed",
			wantCode:   "VALIDATION_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseAPIError(tt.statusCode, tt.body)
			apiErr, ok := err.(*APIError)
			if !ok {
				t.Errorf("parseAPIError() returned wrong type: %T", err)
				return
			}

			if apiErr.StatusCode != tt.statusCode {
				t.Errorf("parseAPIError() StatusCode = %v, want %v", apiErr.StatusCode, tt.statusCode)
			}

			if apiErr.Message != tt.wantMsg {
				t.Errorf("parseAPIError() Message = %v, want %v", apiErr.Message, tt.wantMsg)
			}

			if apiErr.Code != tt.wantCode {
				t.Errorf("parseAPIError() Code = %v, want %v", apiErr.Code, tt.wantCode)
			}
		})
	}
}

func TestValidateSerializable(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		wantErr bool
	}{
		// Valid types
		{name: "nil", value: nil, wantErr: false},
		{name: "bool", value: true, wantErr: false},
		{name: "string", value: "test", wantErr: false},
		{name: "int", value: 42, wantErr: false},
		{name: "float64", value: 3.14, wantErr: false},
		{name: "time.Time", value: time.Now(), wantErr: false},
		{name: "[]byte", value: []byte("test"), wantErr: false},
		{name: "json.RawMessage", value: json.RawMessage(`{}`), wantErr: false},
		{name: "slice", value: []string{"a", "b"}, wantErr: false},
		{name: "map", value: map[string]int{"a": 1}, wantErr: false},
		{name: "struct", value: struct{ Name string }{Name: "test"}, wantErr: false},
		{name: "pointer to struct", value: &struct{ Name string }{Name: "test"}, wantErr: false},

		// Invalid types
		{name: "channel", value: make(chan int), wantErr: true},
		{name: "function", value: func() {}, wantErr: true},
		{name: "complex", value: complex(1, 2), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSerializable(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSerializable() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Benchmark tests
func BenchmarkSerialize(b *testing.B) {
	complexStruct := struct {
		ID        int                    `json:"id"`
		Name      string                 `json:"name"`
		Active    bool                   `json:"active"`
		CreatedAt time.Time              `json:"created_at"`
		Tags      []string               `json:"tags,omitempty"`
		Metadata  map[string]interface{} `json:"metadata,omitempty"`
	}{
		ID:        1,
		Name:      "Test Item",
		Active:    true,
		CreatedAt: time.Now(),
		Tags:      []string{"tag1", "tag2", "tag3"},
		Metadata: map[string]interface{}{
			"version": "1.0",
			"count":   10,
			"nested": map[string]interface{}{
				"key": "value",
			},
		},
	}

	benchmarks := []struct {
		name  string
		value interface{}
	}{
		{"string", "hello world"},
		{"int", 42},
		{"float", 3.14159},
		{"small_struct", struct{ Name string }{Name: "test"}},
		{"complex_struct", complexStruct},
		{"small_slice", []int{1, 2, 3, 4, 5}},
		{"large_slice", make([]int, 1000)},
		{"small_map", map[string]string{"key": "value"}},
		{"interface_map", map[string]interface{}{
			"string": "value",
			"int":    42,
			"float":  3.14,
			"bool":   true,
			"slice":  []int{1, 2, 3},
			"map": map[string]string{
				"nested": "map",
			},
		}},
		{"json_raw_message", json.RawMessage(`{"nested": "json"}`)},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := serialize(bm.value)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkDeserialize(b *testing.B) {
	benchmarks := []struct {
		name   string
		data   json.RawMessage
		target interface{}
	}{
		{
			"string",
			json.RawMessage(`"hello world"`),
			new(string),
		},
		{
			"struct",
			json.RawMessage(`{"name":"test","value":42}`),
			new(struct {
				Name  string `json:"name"`
				Value int    `json:"value"`
			}),
		},
		{
			"slice",
			json.RawMessage(`[1,2,3,4,5]`),
			new([]int),
		},
		{
			"map",
			json.RawMessage(`{"a":1,"b":2,"c":3}`),
			new(map[string]int),
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := deserialize(bm.data, bm.target); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Helper functions
func durationPtr(d time.Duration) *time.Duration {
	return &d
}

func intPtr(i int) *int {
	return &i
}

func dereferenceValue(ptr interface{}) interface{} {
	switch v := ptr.(type) {
	case *string:
		return *v
	case *int:
		return *v
	case *float64:
		return *v
	case *bool:
		return *v
	case *[]string:
		return *v
	case *map[string]string:
		return *v
	case *json.RawMessage:
		return *v
	default:
		// For structs and other types, use reflection
		return reflect.ValueOf(ptr).Elem().Interface()
	}
}

func toJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func compareJSON(t *testing.T, expected, actual string) bool {
	t.Helper()

	var expectedObj, actualObj interface{}

	if err := json.Unmarshal([]byte(expected), &expectedObj); err != nil {
		t.Logf("Failed to unmarshal expected JSON: %v", err)
		return false
	}

	if err := json.Unmarshal([]byte(actual), &actualObj); err != nil {
		t.Logf("Failed to unmarshal actual JSON: %v", err)
		return false
	}

	return reflect.DeepEqual(expectedObj, actualObj)
}
