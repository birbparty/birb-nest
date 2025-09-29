package testdata

import (
	"encoding/json"
	"time"
)

// TestData provides common test data for SDK tests
var TestData = struct {
	SimpleString   string
	SimpleInt      int
	SimpleFloat    float64
	SimpleBool     bool
	SimpleTime     time.Time
	ComplexStruct  TestStruct
	NestedStruct   NestedTestStruct
	StringSlice    []string
	IntSlice       []int
	StringMap      map[string]string
	InterfaceMap   map[string]interface{}
	LargeData      LargeTestData
	UnicodeString  string
	EmptyString    string
	ZeroInt        int
	NilPointer     *string
	JSONRawMessage json.RawMessage
}{
	SimpleString:   "test-value",
	SimpleInt:      42,
	SimpleFloat:    3.14159,
	SimpleBool:     true,
	SimpleTime:     time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
	UnicodeString:  "Hello 世界 🌍",
	EmptyString:    "",
	ZeroInt:        0,
	NilPointer:     nil,
	JSONRawMessage: json.RawMessage(`{"nested": "json", "count": 5}`),
}

// TestStruct is a simple test structure
type TestStruct struct {
	ID        int                    `json:"id"`
	Name      string                 `json:"name"`
	Active    bool                   `json:"active"`
	CreatedAt time.Time              `json:"created_at"`
	Tags      []string               `json:"tags,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// NestedTestStruct contains nested structures
type NestedTestStruct struct {
	Level1 struct {
		Level2 struct {
			Level3 struct {
				Value string `json:"value"`
			} `json:"level3"`
		} `json:"level2"`
	} `json:"level1"`
}

// LargeTestData represents a large data structure for performance testing
type LargeTestData struct {
	Items []LargeItem `json:"items"`
}

// LargeItem is a component of large test data
type LargeItem struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Properties  map[string]interface{} `json:"properties"`
	Data        []byte                 `json:"data"`
}

// Initialize test data
func init() {
	// Complex struct
	TestData.ComplexStruct = TestStruct{
		ID:        1,
		Name:      "Test Item",
		Active:    true,
		CreatedAt: TestData.SimpleTime,
		Tags:      []string{"tag1", "tag2", "tag3"},
		Metadata: map[string]interface{}{
			"version": "1.0",
			"count":   10,
			"nested": map[string]interface{}{
				"key": "value",
			},
		},
	}

	// Nested struct
	TestData.NestedStruct.Level1.Level2.Level3.Value = "deeply nested"

	// Slices
	TestData.StringSlice = []string{"apple", "banana", "cherry"}
	TestData.IntSlice = []int{1, 2, 3, 4, 5}

	// Maps
	TestData.StringMap = map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	TestData.InterfaceMap = map[string]interface{}{
		"string": "value",
		"int":    42,
		"float":  3.14,
		"bool":   true,
		"slice":  []int{1, 2, 3},
		"map": map[string]string{
			"nested": "map",
		},
	}

	// Large data (1000 items)
	items := make([]LargeItem, 1000)
	for i := 0; i < 1000; i++ {
		items[i] = LargeItem{
			ID:          generateID(i),
			Name:        generateName(i),
			Description: generateDescription(i),
			Properties: map[string]interface{}{
				"index":    i,
				"squared":  i * i,
				"even":     i%2 == 0,
				"category": generateCategory(i),
			},
			Data: generateData(i),
		}
	}
	TestData.LargeData = LargeTestData{Items: items}
}

// Helper functions for generating test data
func generateID(i int) string {
	return "id-" + string(rune('a'+i%26)) + "-" + string(rune('0'+i%10))
}

func generateName(i int) string {
	names := []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon"}
	return names[i%len(names)] + " " + string(rune('0'+i%10))
}

func generateDescription(i int) string {
	return "This is a test description for item number " + string(rune('0'+i%10)) +
		". It contains some text to make the payload larger."
}

func generateCategory(i int) string {
	categories := []string{"electronics", "books", "clothing", "food", "toys"}
	return categories[i%len(categories)]
}

func generateData(i int) []byte {
	// Generate 100 bytes of data
	data := make([]byte, 100)
	for j := range data {
		data[j] = byte((i + j) % 256)
	}
	return data
}

// ErrorScenarios provides test cases for error handling
var ErrorScenarios = []struct {
	Name       string
	StatusCode int
	ErrorMsg   string
	ErrorCode  string
	Retryable  bool
}{
	{
		Name:       "NotFound",
		StatusCode: 404,
		ErrorMsg:   "Cache entry not found",
		ErrorCode:  "NOT_FOUND",
		Retryable:  false,
	},
	{
		Name:       "BadRequest",
		StatusCode: 400,
		ErrorMsg:   "Invalid request format",
		ErrorCode:  "BAD_REQUEST",
		Retryable:  false,
	},
	{
		Name:       "Unauthorized",
		StatusCode: 401,
		ErrorMsg:   "Authentication required",
		ErrorCode:  "UNAUTHORIZED",
		Retryable:  false,
	},
	{
		Name:       "RateLimited",
		StatusCode: 429,
		ErrorMsg:   "Too many requests",
		ErrorCode:  "RATE_LIMITED",
		Retryable:  true,
	},
	{
		Name:       "InternalServerError",
		StatusCode: 500,
		ErrorMsg:   "Internal server error",
		ErrorCode:  "INTERNAL_ERROR",
		Retryable:  true,
	},
	{
		Name:       "BadGateway",
		StatusCode: 502,
		ErrorMsg:   "Bad gateway",
		ErrorCode:  "BAD_GATEWAY",
		Retryable:  true,
	},
	{
		Name:       "ServiceUnavailable",
		StatusCode: 503,
		ErrorMsg:   "Service temporarily unavailable",
		ErrorCode:  "SERVICE_UNAVAILABLE",
		Retryable:  true,
	},
	{
		Name:       "GatewayTimeout",
		StatusCode: 504,
		ErrorMsg:   "Gateway timeout",
		ErrorCode:  "GATEWAY_TIMEOUT",
		Retryable:  true,
	},
}

// ConcurrencyTestCase represents a test case for concurrent operations
type ConcurrencyTestCase struct {
	Name            string
	NumGoroutines   int
	NumOperations   int
	OperationDelay  time.Duration
	ExpectedResults int
}

// ConcurrencyTestCases provides test cases for concurrent operations
var ConcurrencyTestCases = []ConcurrencyTestCase{
	{
		Name:            "LowConcurrency",
		NumGoroutines:   5,
		NumOperations:   10,
		OperationDelay:  time.Millisecond,
		ExpectedResults: 50,
	},
	{
		Name:            "MediumConcurrency",
		NumGoroutines:   20,
		NumOperations:   50,
		OperationDelay:  time.Millisecond,
		ExpectedResults: 1000,
	},
	{
		Name:            "HighConcurrency",
		NumGoroutines:   100,
		NumOperations:   10,
		OperationDelay:  time.Millisecond,
		ExpectedResults: 1000,
	},
}
