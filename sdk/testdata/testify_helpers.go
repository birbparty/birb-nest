package testdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSuite provides common test setup and utilities
type TestSuite struct {
	T          *testing.T
	Server     *MockServer
	BaseURL    string
	Context    context.Context
	CancelFunc context.CancelFunc
}

// NewTestSuite creates a new test suite with mock server
func NewTestSuite(t *testing.T) *TestSuite {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	server := NewMockServer()

	return &TestSuite{
		T:          t,
		Server:     server,
		BaseURL:    server.URL,
		Context:    ctx,
		CancelFunc: cancel,
	}
}

// Cleanup cleans up test resources
func (ts *TestSuite) Cleanup() {
	if ts.CancelFunc != nil {
		ts.CancelFunc()
	}
	if ts.Server != nil {
		ts.Server.Close()
	}
}

// AssertEventuallyConsistent checks that a condition becomes true within timeout
func AssertEventuallyConsistent(t *testing.T, condition func() bool, timeout time.Duration, tick time.Duration, msgAndArgs ...interface{}) {
	assert.Eventually(t, condition, timeout, tick, msgAndArgs...)
}

// RequireEventuallyConsistent requires that a condition becomes true within timeout
func RequireEventuallyConsistent(t *testing.T, condition func() bool, timeout time.Duration, tick time.Duration, msgAndArgs ...interface{}) {
	require.Eventually(t, condition, timeout, tick, msgAndArgs...)
}

// AssertHTTPError verifies HTTP error responses
func AssertHTTPError(t *testing.T, err error, expectedStatus int, expectedCode string) {
	require.Error(t, err, "Expected HTTP error")

	// Check if it's an APIError
	apiErr, ok := err.(*APIError)
	if !ok {
		// Try to unwrap
		var apiErrPtr *APIError
		if assert.ErrorAs(t, err, &apiErrPtr) {
			apiErr = apiErrPtr
		} else {
			t.Fatalf("Expected APIError, got %T: %v", err, err)
		}
	}

	assert.Equal(t, expectedStatus, apiErr.StatusCode, "Status code mismatch")
	assert.Equal(t, expectedCode, apiErr.Code, "Error code mismatch")
}

// AssertEnhancedError verifies enhanced error properties
func AssertEnhancedError(t *testing.T, err error, expectedType ErrorType, expectedRetryable bool) {
	require.Error(t, err, "Expected error")

	enhancedErr, ok := err.(*Error)
	if !ok {
		var enhancedErrPtr *Error
		if assert.ErrorAs(t, err, &enhancedErrPtr) {
			enhancedErr = enhancedErrPtr
		} else {
			t.Fatalf("Expected *Error, got %T: %v", err, err)
		}
	}

	assert.Equal(t, expectedType, enhancedErr.Type, "Error type mismatch")
	assert.Equal(t, expectedRetryable, enhancedErr.Retryable, "Retryable mismatch")
}

// MockTransport provides a configurable HTTP transport for testing
type MockTransport struct {
	sync.Mutex
	responses   map[string]*MockResponse
	defaultResp *MockResponse
	requests    []*http.Request
	callCount   map[string]int
}

// MockResponse defines a mock HTTP response
type MockResponse struct {
	Status  int
	Body    interface{}
	Error   error
	Delay   time.Duration
	Headers map[string]string
}

// NewMockTransport creates a new mock transport
func NewMockTransport() *MockTransport {
	return &MockTransport{
		responses: make(map[string]*MockResponse),
		callCount: make(map[string]int),
		requests:  make([]*http.Request, 0),
	}
}

// RoundTrip implements http.RoundTripper
func (mt *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	mt.Lock()
	defer mt.Unlock()

	// Record request
	mt.requests = append(mt.requests, req.Clone(context.Background()))

	// Build key for response lookup
	key := fmt.Sprintf("%s %s", req.Method, req.URL.Path)
	mt.callCount[key]++

	// Find response
	resp := mt.responses[key]
	if resp == nil {
		resp = mt.defaultResp
	}
	if resp == nil {
		return nil, fmt.Errorf("no mock response configured for %s", key)
	}

	// Apply delay if configured
	if resp.Delay > 0 {
		time.Sleep(resp.Delay)
	}

	// Return error if configured
	if resp.Error != nil {
		return nil, resp.Error
	}

	// Build response
	httpResp := &http.Response{
		StatusCode: resp.Status,
		Status:     http.StatusText(resp.Status),
		Header:     make(http.Header),
		Request:    req,
	}

	// Add headers
	for k, v := range resp.Headers {
		httpResp.Header.Set(k, v)
	}

	// Set body
	if resp.Body != nil {
		body, err := json.Marshal(resp.Body)
		if err != nil {
			return nil, err
		}
		httpResp.Body = &mockReadCloser{strings.NewReader(string(body))}
		httpResp.ContentLength = int64(len(body))
		httpResp.Header.Set("Content-Type", "application/json")
	}

	return httpResp, nil
}

// SetResponse configures a response for a specific method and path
func (mt *MockTransport) SetResponse(method, path string, resp *MockResponse) {
	mt.Lock()
	defer mt.Unlock()
	key := fmt.Sprintf("%s %s", method, path)
	mt.responses[key] = resp
}

// SetDefaultResponse sets the default response for unmatched requests
func (mt *MockTransport) SetDefaultResponse(resp *MockResponse) {
	mt.Lock()
	defer mt.Unlock()
	mt.defaultResp = resp
}

// GetRequests returns all recorded requests
func (mt *MockTransport) GetRequests() []*http.Request {
	mt.Lock()
	defer mt.Unlock()
	result := make([]*http.Request, len(mt.requests))
	copy(result, mt.requests)
	return result
}

// GetCallCount returns the number of calls for a specific endpoint
func (mt *MockTransport) GetCallCount(method, path string) int {
	mt.Lock()
	defer mt.Unlock()
	key := fmt.Sprintf("%s %s", method, path)
	return mt.callCount[key]
}

// Reset clears all recorded data
func (mt *MockTransport) Reset() {
	mt.Lock()
	defer mt.Unlock()
	mt.requests = mt.requests[:0]
	mt.callCount = make(map[string]int)
}

// mockReadCloser wraps a reader to implement ReadCloser
type mockReadCloser struct {
	*strings.Reader
}

func (m *mockReadCloser) Close() error {
	return nil
}

// APIError represents an API error (for type assertions in tests)
type APIError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Code       string `json:"code"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s (HTTP %d): %s", e.Code, e.StatusCode, e.Message)
}

// Error types for testing
type ErrorType string

const (
	ErrorTypeNetwork     ErrorType = "network"
	ErrorTypeTimeout     ErrorType = "timeout"
	ErrorTypeValidation  ErrorType = "validation"
	ErrorTypeServer      ErrorType = "server"
	ErrorTypeRateLimit   ErrorType = "rate_limit"
	ErrorTypeCircuitOpen ErrorType = "circuit_open"
)

// Error represents an enhanced error
type Error struct {
	Type      ErrorType
	Message   string
	Retryable bool
	Code      string
	wrapped   error
	Context   *ErrorContext
	Details   map[string]interface{}
}

// ErrorContext provides context for errors
type ErrorContext struct {
	URL        string
	Method     string
	RetryCount int
	Duration   time.Duration
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.wrapped
}

// GenerateTestData creates test data of specified size
func GenerateTestData(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte('a' + (i % 26))
	}
	return data
}

// GenerateJSONData creates JSON test data
func GenerateJSONData(numFields int) map[string]interface{} {
	data := make(map[string]interface{})
	for i := 0; i < numFields; i++ {
		key := fmt.Sprintf("field_%d", i)
		switch i % 4 {
		case 0:
			data[key] = fmt.Sprintf("value_%d", i)
		case 1:
			data[key] = i
		case 2:
			data[key] = float64(i) * 1.5
		case 3:
			data[key] = i%2 == 0
		}
	}
	return data
}

// BenchmarkHelper provides utilities for benchmarking
type BenchmarkHelper struct {
	b      *testing.B
	server *MockServer
}

// NewBenchmarkHelper creates a new benchmark helper
func NewBenchmarkHelper(b *testing.B) *BenchmarkHelper {
	server := NewMockServer()
	b.Cleanup(server.Close)
	return &BenchmarkHelper{
		b:      b,
		server: server,
	}
}

// RunBenchmark runs a benchmark with setup and teardown
func (bh *BenchmarkHelper) RunBenchmark(name string, fn func(b *testing.B)) {
	bh.b.Run(name, func(b *testing.B) {
		b.ResetTimer()
		fn(b)
	})
}

// MeasureMemory reports memory allocations for a benchmark
func (bh *BenchmarkHelper) MeasureMemory() {
	bh.b.ReportAllocs()
}

// TableDrivenTest represents a test case for table-driven tests
type TableDrivenTest struct {
	Name    string
	Setup   func()
	Run     func(t *testing.T)
	Cleanup func()
	Skip    bool
	SkipMsg string
}

// RunTableDrivenTests executes table-driven tests
func RunTableDrivenTests(t *testing.T, tests []TableDrivenTest) {
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			if tt.Skip {
				t.Skip(tt.SkipMsg)
			}

			if tt.Setup != nil {
				tt.Setup()
			}

			if tt.Cleanup != nil {
				defer tt.Cleanup()
			}

			tt.Run(t)
		})
	}
}

// ConcurrentTestHelper helps with concurrent testing
type ConcurrentTestHelper struct {
	t         *testing.T
	wg        sync.WaitGroup
	errors    []error
	errorsMux sync.Mutex
}

// NewConcurrentTestHelper creates a new concurrent test helper
func NewConcurrentTestHelper(t *testing.T) *ConcurrentTestHelper {
	return &ConcurrentTestHelper{
		t:      t,
		errors: make([]error, 0),
	}
}

// Run executes a function concurrently
func (cth *ConcurrentTestHelper) Run(numGoroutines int, fn func(id int) error) {
	cth.wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer cth.wg.Done()

			if err := fn(id); err != nil {
				cth.errorsMux.Lock()
				cth.errors = append(cth.errors, fmt.Errorf("goroutine %d: %w", id, err))
				cth.errorsMux.Unlock()
			}
		}(i)
	}
}

// Wait waits for all goroutines to complete and checks for errors
func (cth *ConcurrentTestHelper) Wait() {
	cth.wg.Wait()

	cth.errorsMux.Lock()
	defer cth.errorsMux.Unlock()

	if len(cth.errors) > 0 {
		for _, err := range cth.errors {
			cth.t.Error(err)
		}
		cth.t.Fatalf("Concurrent test failed with %d errors", len(cth.errors))
	}
}

// RaceDetector helps detect race conditions
func RaceDetector(t *testing.T, fn func()) {
	const numGoroutines = 10
	const numIterations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				fn()
			}
		}()
	}

	wg.Wait()
}
