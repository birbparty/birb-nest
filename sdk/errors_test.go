package sdk

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestAPIError(t *testing.T) {
	tests := []struct {
		name      string
		err       *APIError
		wantError string
		wantIsNF  bool
		wantIsSE  bool
		wantIsCE  bool
		wantRetry bool
	}{
		{
			name: "not found error",
			err: &APIError{
				StatusCode: http.StatusNotFound,
				Message:    "Cache entry not found",
				Code:       "NOT_FOUND",
			},
			wantError: "API error (status 404): Cache entry not found",
			wantIsNF:  true,
			wantIsSE:  false,
			wantIsCE:  true,
			wantRetry: false,
		},
		{
			name: "server error",
			err: &APIError{
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal server error",
				Code:       "INTERNAL_ERROR",
			},
			wantError: "API error (status 500): Internal server error",
			wantIsNF:  false,
			wantIsSE:  true,
			wantIsCE:  false,
			wantRetry: true,
		},
		{
			name: "bad request error",
			err: &APIError{
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid request format",
				Code:       "BAD_REQUEST",
			},
			wantError: "API error (status 400): Invalid request format",
			wantIsNF:  false,
			wantIsSE:  false,
			wantIsCE:  true,
			wantRetry: false,
		},
		{
			name: "rate limit error",
			err: &APIError{
				StatusCode: http.StatusTooManyRequests,
				Message:    "Too many requests",
				Code:       "RATE_LIMITED",
			},
			wantError: "API error (status 429): Too many requests",
			wantIsNF:  false,
			wantIsSE:  false,
			wantIsCE:  true,
			wantRetry: true,
		},
		{
			name: "gateway timeout",
			err: &APIError{
				StatusCode: http.StatusGatewayTimeout,
				Message:    "Gateway timeout",
				Code:       "GATEWAY_TIMEOUT",
			},
			wantError: "API error (status 504): Gateway timeout",
			wantIsNF:  false,
			wantIsSE:  true,
			wantIsCE:  false,
			wantRetry: true,
		},
		{
			name: "error with details",
			err: &APIError{
				StatusCode: http.StatusUnprocessableEntity,
				Message:    "Validation failed",
				Code:       "VALIDATION_ERROR",
				Details:    "Field 'name' is required",
			},
			wantError: "API error (status 422): Validation failed - Field 'name' is required",
			wantIsNF:  false,
			wantIsSE:  false,
			wantIsCE:  true,
			wantRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Error() method
			if got := tt.err.Error(); got != tt.wantError {
				t.Errorf("Error() = %v, want %v", got, tt.wantError)
			}

			// Test IsNotFound()
			if got := tt.err.IsNotFound(); got != tt.wantIsNF {
				t.Errorf("IsNotFound() = %v, want %v", got, tt.wantIsNF)
			}

			// Test IsServerError()
			if got := tt.err.IsServerError(); got != tt.wantIsSE {
				t.Errorf("IsServerError() = %v, want %v", got, tt.wantIsSE)
			}

			// Test IsClientError()
			if got := tt.err.IsClientError(); got != tt.wantIsCE {
				t.Errorf("IsClientError() = %v, want %v", got, tt.wantIsCE)
			}

			// Test IsRetryable()
			if got := tt.err.IsRetryable(); got != tt.wantRetry {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.wantRetry)
			}
		})
	}
}

func TestNetworkError(t *testing.T) {
	baseErr := errors.New("connection refused")
	netErr := &NetworkError{
		Op:  "GET /v1/cache/test",
		Err: baseErr,
	}

	// Test Error() method
	want := "network error during GET /v1/cache/test: connection refused"
	if got := netErr.Error(); got != want {
		t.Errorf("Error() = %v, want %v", got, want)
	}

	// Test Unwrap()
	if got := netErr.Unwrap(); got != baseErr {
		t.Errorf("Unwrap() = %v, want %v", got, baseErr)
	}

	// Test IsRetryable()
	if !netErr.IsRetryable() {
		t.Error("IsRetryable() = false, want true")
	}
}

func TestTimeoutError(t *testing.T) {
	timeoutErr := &TimeoutError{
		Op: "POST /v1/cache/test",
	}

	// Test Error() method
	want := "timeout during POST /v1/cache/test"
	if got := timeoutErr.Error(); got != want {
		t.Errorf("Error() = %v, want %v", got, want)
	}

	// Test IsRetryable()
	if !timeoutErr.IsRetryable() {
		t.Error("IsRetryable() = false, want true")
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "ErrNotFound",
			err:  ErrNotFound,
			want: true,
		},
		{
			name: "APIError with 404",
			err: &APIError{
				StatusCode: http.StatusNotFound,
				Message:    "Not found",
			},
			want: true,
		},
		{
			name: "APIError with NOT_FOUND code",
			err: &APIError{
				StatusCode: http.StatusNotFound,
				Code:       "NOT_FOUND",
			},
			want: true,
		},
		{
			name: "Other APIError",
			err: &APIError{
				StatusCode: http.StatusBadRequest,
				Message:    "Bad request",
			},
			want: false,
		},
		{
			name: "NetworkError",
			err:  &NetworkError{Op: "test", Err: errors.New("error")},
			want: false,
		},
		{
			name: "Generic error",
			err:  errors.New("some error"),
			want: false,
		},
		{
			name: "Wrapped ErrNotFound",
			err:  fmt.Errorf("wrapped: %w", ErrNotFound),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFound(tt.err); got != tt.want {
				t.Errorf("IsNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "ErrTimeout",
			err:  ErrTimeout,
			want: true,
		},
		{
			name: "ErrServerError",
			err:  ErrServerError,
			want: true,
		},
		{
			name: "Server error (500)",
			err: &APIError{
				StatusCode: http.StatusInternalServerError,
			},
			want: true,
		},
		{
			name: "Bad gateway (502)",
			err: &APIError{
				StatusCode: http.StatusBadGateway,
			},
			want: true,
		},
		{
			name: "Service unavailable (503)",
			err: &APIError{
				StatusCode: http.StatusServiceUnavailable,
			},
			want: true,
		},
		{
			name: "Gateway timeout (504)",
			err: &APIError{
				StatusCode: http.StatusGatewayTimeout,
			},
			want: true,
		},
		{
			name: "Rate limited (429)",
			err: &APIError{
				StatusCode: http.StatusTooManyRequests,
			},
			want: true,
		},
		{
			name: "Request timeout (408)",
			err: &APIError{
				StatusCode: http.StatusRequestTimeout,
			},
			want: true,
		},
		{
			name: "Client error (400)",
			err: &APIError{
				StatusCode: http.StatusBadRequest,
			},
			want: false,
		},
		{
			name: "Not found (404)",
			err: &APIError{
				StatusCode: http.StatusNotFound,
			},
			want: false,
		},
		{
			name: "NetworkError",
			err:  &NetworkError{Op: "test", Err: errors.New("connection refused")},
			want: true,
		},
		{
			name: "TimeoutError",
			err:  &TimeoutError{Op: "test"},
			want: true,
		},
		{
			name: "Generic error",
			err:  errors.New("some error"),
			want: false,
		},
		{
			name: "Wrapped ErrTimeout",
			err:  fmt.Errorf("wrapped: %w", ErrTimeout),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.want {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorConstants(t *testing.T) {
	// Test that error constants are defined and have expected messages
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "ErrInvalidConfig",
			err:  ErrInvalidConfig,
			want: "invalid configuration",
		},
		{
			name: "ErrNotFound",
			err:  ErrNotFound,
			want: "key not found",
		},
		{
			name: "ErrTimeout",
			err:  ErrTimeout,
			want: "request timeout",
		},
		{
			name: "ErrServerError",
			err:  ErrServerError,
			want: "server error",
		},
		{
			name: "ErrInvalidResponse",
			err:  ErrInvalidResponse,
			want: "invalid response from server",
		},
		{
			name: "ErrContextCanceled",
			err:  ErrContextCanceled,
			want: "context canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseAPIError_EdgeCases(t *testing.T) {
	// This test complements the one in serializer_test.go
	// with additional edge cases
	tests := []struct {
		name       string
		statusCode int
		body       []byte
		wantType   bool // true if we expect an APIError type
	}{
		{
			name:       "valid JSON error",
			statusCode: 400,
			body:       []byte(`{"error": "Bad request", "code": "BAD_REQUEST"}`),
			wantType:   true,
		},
		{
			name:       "empty body",
			statusCode: 500,
			body:       []byte{},
			wantType:   true,
		},
		{
			name:       "non-JSON body",
			statusCode: 502,
			body:       []byte("Bad Gateway"),
			wantType:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseAPIError(tt.statusCode, tt.body)
			_, ok := err.(*APIError)
			if ok != tt.wantType {
				t.Errorf("parseAPIError() returned type %T, want APIError = %v", err, tt.wantType)
			}
		})
	}
}

// Benchmark error creation and checking
func BenchmarkAPIError_Error(b *testing.B) {
	err := &APIError{
		StatusCode: 404,
		Message:    "Not found",
		Code:       "NOT_FOUND",
		Details:    "The requested resource was not found",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.Error()
	}
}

func BenchmarkIsNotFound(b *testing.B) {
	errors := []error{
		nil,
		ErrNotFound,
		&APIError{StatusCode: 404},
		&NetworkError{Op: "test", Err: ErrNotFound},
		errors.New("random error"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, err := range errors {
			_ = IsNotFound(err)
		}
	}
}

func BenchmarkIsRetryable(b *testing.B) {
	errors := []error{
		nil,
		ErrTimeout,
		&APIError{StatusCode: 500},
		&APIError{StatusCode: 400},
		&NetworkError{Op: "test", Err: ErrTimeout},
		errors.New("random error"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, err := range errors {
			_ = IsRetryable(err)
		}
	}
}
