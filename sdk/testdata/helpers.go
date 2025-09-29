package testdata

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

// AssertEqual checks if two values are equal
func AssertEqual(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	if !reflect.DeepEqual(expected, actual) {
		if len(msgAndArgs) > 0 {
			t.Errorf("%s: expected %v, got %v", msgAndArgs[0], expected, actual)
		} else {
			t.Errorf("expected %v, got %v", expected, actual)
		}
	}
}

// AssertNotEqual checks if two values are not equal
func AssertNotEqual(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	if reflect.DeepEqual(expected, actual) {
		if len(msgAndArgs) > 0 {
			t.Errorf("%s: expected values to be different, but both are %v", msgAndArgs[0], actual)
		} else {
			t.Errorf("expected values to be different, but both are %v", actual)
		}
	}
}

// AssertNil checks if a value is nil
func AssertNil(t *testing.T, object interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	if !isNil(object) {
		if len(msgAndArgs) > 0 {
			t.Errorf("%s: expected nil, got %v", msgAndArgs[0], object)
		} else {
			t.Errorf("expected nil, got %v", object)
		}
	}
}

// AssertNotNil checks if a value is not nil
func AssertNotNil(t *testing.T, object interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	if isNil(object) {
		if len(msgAndArgs) > 0 {
			t.Errorf("%s: expected non-nil value", msgAndArgs[0])
		} else {
			t.Errorf("expected non-nil value")
		}
	}
}

// AssertTrue checks if a value is true
func AssertTrue(t *testing.T, value bool, msgAndArgs ...interface{}) {
	t.Helper()
	if !value {
		if len(msgAndArgs) > 0 {
			t.Errorf("%s: expected true, got false", msgAndArgs[0])
		} else {
			t.Errorf("expected true, got false")
		}
	}
}

// AssertFalse checks if a value is false
func AssertFalse(t *testing.T, value bool, msgAndArgs ...interface{}) {
	t.Helper()
	if value {
		if len(msgAndArgs) > 0 {
			t.Errorf("%s: expected false, got true", msgAndArgs[0])
		} else {
			t.Errorf("expected false, got true")
		}
	}
}

// AssertContains checks if a string contains a substring
func AssertContains(t *testing.T, s, contains string, msgAndArgs ...interface{}) {
	t.Helper()
	if !stringContains(s, contains) {
		if len(msgAndArgs) > 0 {
			t.Errorf("%s: expected %q to contain %q", msgAndArgs[0], s, contains)
		} else {
			t.Errorf("expected %q to contain %q", s, contains)
		}
	}
}

// AssertJSONEqual checks if two JSON values are equal
func AssertJSONEqual(t *testing.T, expected, actual json.RawMessage, msgAndArgs ...interface{}) {
	t.Helper()

	var expectedObj, actualObj interface{}
	if err := json.Unmarshal(expected, &expectedObj); err != nil {
		t.Fatalf("failed to unmarshal expected JSON: %v", err)
	}
	if err := json.Unmarshal(actual, &actualObj); err != nil {
		t.Fatalf("failed to unmarshal actual JSON: %v", err)
	}

	if !reflect.DeepEqual(expectedObj, actualObj) {
		if len(msgAndArgs) > 0 {
			t.Errorf("%s: JSON values not equal\nexpected: %s\nactual: %s", msgAndArgs[0], expected, actual)
		} else {
			t.Errorf("JSON values not equal\nexpected: %s\nactual: %s", expected, actual)
		}
	}
}

// AssertDuration checks if a duration is within expected range
func AssertDuration(t *testing.T, actual, expected, tolerance time.Duration, msgAndArgs ...interface{}) {
	t.Helper()
	diff := actual - expected
	if diff < 0 {
		diff = -diff
	}
	if diff > tolerance {
		if len(msgAndArgs) > 0 {
			t.Errorf("%s: duration %v not within %v of expected %v", msgAndArgs[0], actual, tolerance, expected)
		} else {
			t.Errorf("duration %v not within %v of expected %v", actual, tolerance, expected)
		}
	}
}

// RequireNoError fails the test if an error occurred
func RequireNoError(t *testing.T, err error, msgAndArgs ...interface{}) {
	t.Helper()
	if err != nil {
		if len(msgAndArgs) > 0 {
			t.Fatalf("%s: unexpected error: %v", msgAndArgs[0], err)
		} else {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

// RequireError fails the test if no error occurred
func RequireError(t *testing.T, err error, msgAndArgs ...interface{}) {
	t.Helper()
	if err == nil {
		if len(msgAndArgs) > 0 {
			t.Fatalf("%s: expected error but got nil", msgAndArgs[0])
		} else {
			t.Fatalf("expected error but got nil")
		}
	}
}

// WithTimeout creates a context with timeout
func WithTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, timeout)
}

// isNil checks if a value is nil (handles nil interface values)
func isNil(object interface{}) bool {
	if object == nil {
		return true
	}

	value := reflect.ValueOf(object)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return value.IsNil()
	}
	return false
}

// stringContains is a simple substring check
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// WaitForCondition waits for a condition to be true
func WaitForCondition(t *testing.T, condition func() bool, timeout time.Duration, interval time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(interval)
	}

	t.Fatalf("condition not met within %v", timeout)
}

// RunConcurrently runs a function concurrently with specified goroutines
func RunConcurrently(t *testing.T, numGoroutines int, fn func(id int)) {
	t.Helper()

	done := make(chan struct{}, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() {
				done <- struct{}{}
			}()
			fn(id)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
		case <-time.After(30 * time.Second):
			t.Fatalf("goroutine %d did not complete within timeout", i)
		}
	}
}

// MeasureTime measures the execution time of a function
func MeasureTime(fn func()) time.Duration {
	start := time.Now()
	fn()
	return time.Since(start)
}

// GenerateTestKey generates a unique test key
func GenerateTestKey(prefix string) string {
	return prefix + "-" + time.Now().Format("20060102-150405.000")
}

// CompareJSON compares two JSON strings ignoring formatting
func CompareJSON(t *testing.T, expected, actual string) bool {
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
