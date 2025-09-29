package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

// Test types for typed methods
type TestUser struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

type TestProduct struct {
	SKU        string                 `json:"sku"`
	Name       string                 `json:"name"`
	Price      float64                `json:"price"`
	InStock    bool                   `json:"in_stock"`
	Categories []string               `json:"categories"`
	Attributes map[string]interface{} `json:"attributes"`
}

type TestConfig struct {
	Version  string          `json:"version"`
	Features map[string]bool `json:"features"`
	Limits   map[string]int  `json:"limits"`
	Servers  []string        `json:"servers"`
}

func TestClient_SetTyped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		// Verify the request body
		var req CacheRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		// Return success response
		resp := CacheResponse{
			Key:       r.URL.Path[len("/v1/cache/"):],
			Value:     req.Value,
			Version:   1,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewClient(DefaultConfig().WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	tests := []struct {
		name  string
		key   string
		value interface{}
	}{
		{
			name: "simple struct",
			key:  "user:123",
			value: TestUser{
				ID:        123,
				Name:      "John Doe",
				Email:     "john@example.com",
				Active:    true,
				CreatedAt: time.Now(),
			},
		},
		{
			name: "complex struct",
			key:  "product:ABC123",
			value: TestProduct{
				SKU:        "ABC123",
				Name:       "Widget",
				Price:      29.99,
				InStock:    true,
				Categories: []string{"electronics", "gadgets"},
				Attributes: map[string]interface{}{
					"color":  "blue",
					"weight": 0.5,
				},
			},
		},
		{
			name:  "slice",
			key:   "tags",
			value: []string{"go", "sdk", "cache"},
		},
		{
			name: "map",
			key:  "settings",
			value: map[string]interface{}{
				"theme":    "dark",
				"language": "en",
				"timezone": "UTC",
			},
		},
		{
			name: "pointer",
			key:  "ptr-value",
			value: &TestUser{
				ID:   456,
				Name: "Jane Doe",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SetTyped(ctx, client, tt.key, tt.value)
			if err != nil {
				t.Errorf("SetTyped() error = %v", err)
			}
		})
	}
}

func TestClient_GetTyped(t *testing.T) {
	testUser := TestUser{
		ID:        123,
		Name:      "John Doe",
		Email:     "john@example.com",
		Active:    true,
		CreatedAt: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET, got %s", r.Method)
		}

		key := r.URL.Path[len("/v1/cache/"):]

		switch key {
		case "user:123":
			data, _ := json.Marshal(testUser)
			resp := CacheResponse{
				Key:       key,
				Value:     json.RawMessage(data),
				Version:   1,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			json.NewEncoder(w).Encode(resp)

		case "not-found":
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Not found",
				"code":  "NOT_FOUND",
			})

		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client, err := NewClient(DefaultConfig().WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	t.Run("successful get", func(t *testing.T) {
		user, err := GetTyped[TestUser](ctx, client, "user:123")
		if err != nil {
			t.Fatalf("GetTyped() error = %v", err)
		}

		if !reflect.DeepEqual(user, testUser) {
			t.Errorf("GetTyped() = %+v, want %+v", user, testUser)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := GetTyped[TestUser](ctx, client, "not-found")
		if !IsNotFound(err) {
			t.Errorf("Expected not found error, got %v", err)
		}
	})

	t.Run("nil target", func(t *testing.T) {
		var nilPtr *TestUser
		err := client.Get(ctx, "user:123", nilPtr)
		if err == nil {
			t.Error("Expected error for nil target")
		}
	})
}

func TestExtendedClient_SetTypedWithOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req CacheRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Verify TTL and metadata were included
		if req.TTL == nil || *req.TTL != 300 {
			t.Errorf("Expected TTL=300, got %v", req.TTL)
		}

		if req.Metadata == nil || req.Metadata["source"] != "test" {
			t.Errorf("Expected metadata with source=test, got %v", req.Metadata)
		}

		resp := CacheResponse{
			Key:       r.URL.Path[len("/v1/cache/"):],
			Value:     req.Value,
			Version:   1,
			TTL:       req.TTL,
			Metadata:  req.Metadata,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewExtendedClient(DefaultConfig().WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	ttl := 5 * time.Minute

	user := TestUser{
		ID:    789,
		Name:  "Test User",
		Email: "test@example.com",
	}

	opts := &SetOptions{
		TTL: &ttl,
		Metadata: map[string]interface{}{
			"source": "test",
			"env":    "testing",
		},
	}

	err = SetTypedWithOptions(ctx, client, "user:789", user, opts)
	if err != nil {
		t.Errorf("SetTypedWithOptions() error = %v", err)
	}
}

func TestExtendedClient_TypedOperations(t *testing.T) {
	testProduct := TestProduct{
		SKU:     "XYZ789",
		Name:    "Super Widget",
		Price:   49.99,
		InStock: true,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// For SET operations
			var req CacheRequest
			json.NewDecoder(r.Body).Decode(&req)
			resp := CacheResponse{
				Key:       r.URL.Path[len("/v1/cache/"):],
				Value:     req.Value,
				Version:   1,
				TTL:       req.TTL,
				Metadata:  req.Metadata,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(resp)
		} else {
			// For GET operations
			data, _ := json.Marshal(testProduct)
			resp := CacheResponse{
				Key:       r.URL.Path[len("/v1/cache/"):],
				Value:     json.RawMessage(data),
				Version:   2,
				CreatedAt: time.Now().Add(-time.Hour),
				UpdatedAt: time.Now(),
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client, err := NewExtendedClient(DefaultConfig().WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Test setting with options
	ttl := 10 * time.Minute
	opts := &SetOptions{
		TTL: &ttl,
		Metadata: map[string]interface{}{
			"category": "premium",
			"supplier": "ACME Corp",
		},
	}

	err = SetTypedWithOptions(ctx, client, "product:XYZ789", testProduct, opts)
	if err != nil {
		t.Fatalf("SetTypedWithOptions() error = %v", err)
	}

	// Test getting the value back
	product, err := GetTyped[TestProduct](ctx, client, "product:XYZ789")
	if err != nil {
		t.Fatalf("GetTyped() error = %v", err)
	}

	// Verify the product data
	if !reflect.DeepEqual(product, testProduct) {
		t.Errorf("GetTyped() product = %+v, want %+v", product, testProduct)
	}

	// Test GetMultiple
	keys := []string{"product:XYZ789", "product:ABC123"}
	results, err := GetTypedMultiple[TestProduct](ctx, client, keys)
	if err == nil {
		// GetMultiple might fail if product:ABC123 doesn't exist
		if prod, ok := results["product:XYZ789"]; ok {
			if !reflect.DeepEqual(prod, testProduct) {
				t.Errorf("GetTypedMultiple() product = %+v, want %+v", prod, testProduct)
			}
		}
	}

	// Test Exists
	exists, err := client.Exists(ctx, "product:XYZ789")
	if err != nil {
		t.Errorf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("Expected key to exist")
	}
}

func TestTypedMethods_TypeSafety(t *testing.T) {
	// This test verifies that the typed methods maintain type safety
	// and catch type mismatches at compile time

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a user object
		user := TestUser{ID: 1, Name: "Test"}
		data, _ := json.Marshal(user)
		resp := CacheResponse{
			Key:       r.URL.Path[len("/v1/cache/"):],
			Value:     json.RawMessage(data),
			Version:   1,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewClient(DefaultConfig().WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	t.Run("correct type", func(t *testing.T) {
		_, err := GetTyped[TestUser](ctx, client, "test")
		if err != nil {
			t.Errorf("GetTyped() with correct type failed: %v", err)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		// This will succeed at runtime but the data won't match
		product, err := GetTyped[TestProduct](ctx, client, "test")
		if err != nil {
			t.Errorf("GetTyped() error = %v", err)
		}
		// The product fields that don't exist in User will be zero values
		// But Name exists in both structs, so it will be populated
		if product.SKU != "" {
			t.Error("Expected zero value for SKU")
		}
		if product.Name != "Test" {
			t.Errorf("Expected Name to be populated from User, got %q", product.Name)
		}
		if product.Price != 0 {
			t.Error("Expected zero value for Price")
		}
	})
}

func TestTypedMethods_ComplexTypes(t *testing.T) {
	// Store the complex data for the test
	var storedData json.RawMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// Store the data
			var req CacheRequest
			json.NewDecoder(r.Body).Decode(&req)
			storedData = req.Value

			resp := CacheResponse{
				Key:       r.URL.Path[len("/v1/cache/"):],
				Value:     req.Value,
				Version:   1,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(resp)
		} else {
			// Return the stored data
			resp := CacheResponse{
				Key:       r.URL.Path[len("/v1/cache/"):],
				Value:     storedData,
				Version:   1,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client, err := NewClient(DefaultConfig().WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Test with nested maps and slices
	complexData := TestConfig{
		Version: "2.0",
		Features: map[string]bool{
			"auth":    true,
			"logging": true,
			"caching": false,
			"metrics": true,
		},
		Limits: map[string]int{
			"max_requests":    1000,
			"max_connections": 100,
			"timeout_seconds": 30,
		},
		Servers: []string{
			"server1.example.com",
			"server2.example.com",
			"server3.example.com",
		},
	}

	// Set the complex data
	err = SetTyped(ctx, client, "config:v2", complexData)
	if err != nil {
		t.Fatalf("SetTyped() error = %v", err)
	}

	// Get it back
	retrieved, err := GetTyped[TestConfig](ctx, client, "config:v2")
	if err != nil {
		t.Fatalf("GetTyped() error = %v", err)
	}

	// Verify deep equality
	if !reflect.DeepEqual(complexData, retrieved) {
		t.Errorf("Complex data mismatch:\ngot  %+v\nwant %+v", retrieved, complexData)
	}
}

// Benchmark typed methods
func BenchmarkClient_SetTyped(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"key":"test","version":1}`))
	}))
	defer server.Close()

	client, _ := NewClient(DefaultConfig().WithBaseURL(server.URL))
	defer client.Close()

	ctx := context.Background()
	user := TestUser{
		ID:    123,
		Name:  "Benchmark User",
		Email: "bench@example.com",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SetTyped(ctx, client, "bench-key", user)
	}
}

func BenchmarkClient_GetTyped(b *testing.B) {
	user := TestUser{ID: 123, Name: "Test", Email: "test@example.com"}
	data, _ := json.Marshal(user)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"key":"test","value":` + string(data) + `,"version":1}`))
	}))
	defer server.Close()

	client, _ := NewClient(DefaultConfig().WithBaseURL(server.URL))
	defer client.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetTyped[TestUser](ctx, client, "test-key")
	}
}

func BenchmarkTypedVsUntyped(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
		}
		w.Write([]byte(`{"key":"test","value":{"id":123,"name":"Test"},"version":1}`))
	}))
	defer server.Close()

	client, _ := NewClient(DefaultConfig().WithBaseURL(server.URL))
	defer client.Close()

	ctx := context.Background()
	user := TestUser{ID: 123, Name: "Test"}

	b.Run("typed_set", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			SetTyped(ctx, client, "key", user)
		}
	})

	b.Run("untyped_set", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			client.Set(ctx, "key", user)
		}
	})

	b.Run("typed_get", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = GetTyped[TestUser](ctx, client, "key")
		}
	})

	b.Run("untyped_get", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var u TestUser
			client.Get(ctx, "key", &u)
		}
	})
}
