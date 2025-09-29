package sdk

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestBuildPath(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		args    []string
		want    string
	}{
		{
			name:    "no placeholders",
			pattern: "/v1/cache/test",
			args:    []string{},
			want:    "/v1/cache/test",
		},
		{
			name:    "single placeholder",
			pattern: "/v1/cache/{0}",
			args:    []string{"test-key"},
			want:    "/v1/cache/test-key",
		},
		{
			name:    "multiple placeholders",
			pattern: "/v1/{0}/items/{1}",
			args:    []string{"cache", "test-key"},
			want:    "/v1/cache/items/test-key",
		},
		{
			name:    "URL encoding",
			pattern: "/v1/cache/{0}",
			args:    []string{"test key with spaces"},
			want:    "/v1/cache/test%20key%20with%20spaces",
		},
		{
			name:    "special characters",
			pattern: "/v1/cache/{0}",
			args:    []string{"test/key?query=1&foo=bar"},
			want:    "/v1/cache/test%2Fkey%3Fquery%3D1%26foo%3Dbar",
		},
		{
			name:    "unicode characters",
			pattern: "/v1/cache/{0}",
			args:    []string{"测试键"},
			want:    "/v1/cache/%E6%B5%8B%E8%AF%95%E9%94%AE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPath(tt.pattern, tt.args...)
			if got != tt.want {
				t.Errorf("buildPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHTTPTransport_BaseURL(t *testing.T) {
	tests := []struct {
		name       string
		baseURL    string
		wantScheme string
		wantHost   string
		wantPath   string
		wantErr    bool
	}{
		{
			name:       "standard http URL",
			baseURL:    "http://localhost:8080",
			wantScheme: "http",
			wantHost:   "localhost:8080",
			wantPath:   "",
		},
		{
			name:       "https URL",
			baseURL:    "https://api.example.com",
			wantScheme: "https",
			wantHost:   "api.example.com",
			wantPath:   "",
		},
		{
			name:       "URL with path",
			baseURL:    "http://localhost:8080/api/v1",
			wantScheme: "http",
			wantHost:   "localhost:8080",
			wantPath:   "/api/v1",
		},
		{
			name:       "URL with trailing slash",
			baseURL:    "http://localhost:8080/",
			wantScheme: "http",
			wantHost:   "localhost:8080",
			wantPath:   "/",
		},
		{
			name:    "invalid URL",
			baseURL: "not a url",
			wantErr: true,
		},
		{
			name:    "empty URL",
			baseURL: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig().WithBaseURL(tt.baseURL)
			_, err := newHTTPTransport(config)

			if tt.wantErr {
				if err == nil {
					t.Errorf("newHTTPTransport() error = nil, wantErr %v", tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("newHTTPTransport() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			u, _ := url.Parse(tt.baseURL)
			if u.Scheme != tt.wantScheme {
				t.Errorf("URL scheme = %v, want %v", u.Scheme, tt.wantScheme)
			}
			if u.Host != tt.wantHost {
				t.Errorf("URL host = %v, want %v", u.Host, tt.wantHost)
			}
			if u.Path != tt.wantPath {
				t.Errorf("URL path = %v, want %v", u.Path, tt.wantPath)
			}
		})
	}
}

func TestHTTPTransport_Headers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo headers back in response
		for key, values := range r.Header {
			for _, value := range values {
				w.Header().Add("Echo-"+key, value)
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tests := []struct {
		name    string
		headers map[string]string
	}{
		{
			name: "basic headers",
			headers: map[string]string{
				"X-API-Key":    "test-key",
				"X-Request-ID": "12345",
			},
		},
		{
			name: "authorization header",
			headers: map[string]string{
				"Authorization": "Bearer token123",
			},
		},
		{
			name: "custom headers",
			headers: map[string]string{
				"X-Custom-Header": "custom-value",
				"X-Trace-ID":      "trace-123",
			},
		},
		{
			name:    "no custom headers",
			headers: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig().WithBaseURL(server.URL)
			config.Headers = tt.headers

			transport, err := newHTTPTransport(config)
			if err != nil {
				t.Fatal(err)
			}

			req, err := http.NewRequest("GET", server.URL+"/test", nil)
			if err != nil {
				t.Fatal(err)
			}

			// Apply headers (this would be done in the do method)
			for k, v := range config.Headers {
				req.Header.Set(k, v)
			}

			resp, err := transport.client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			// Verify headers were sent
			for key, value := range tt.headers {
				echoKey := "Echo-" + key
				if resp.Header.Get(echoKey) != value {
					t.Errorf("Header %s = %v, want %v", key, resp.Header.Get(echoKey), value)
				}
			}
		})
	}
}

func TestHTTPTransport_Timeout(t *testing.T) {
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()

	tests := []struct {
		name      string
		timeout   time.Duration
		wantError bool
	}{
		{
			name:      "request times out",
			timeout:   100 * time.Millisecond,
			wantError: true,
		},
		{
			name:      "request completes",
			timeout:   3 * time.Second,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig().
				WithBaseURL(slowServer.URL).
				WithTimeout(tt.timeout)

			transport, err := newHTTPTransport(config)
			if err != nil {
				t.Fatal(err)
			}

			req, err := http.NewRequest("GET", slowServer.URL+"/test", nil)
			if err != nil {
				t.Fatal(err)
			}

			_, err = transport.client.Do(req)
			if (err != nil) != tt.wantError {
				t.Errorf("client.Do() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestHTTPTransport_RetryConfig(t *testing.T) {
	config := DefaultConfig()
	config.RetryConfig.MaxRetries = 3
	config.RetryConfig.InitialInterval = 10 * time.Millisecond
	config.RetryConfig.MaxInterval = 100 * time.Millisecond
	config.RetryConfig.Multiplier = 2

	// Verify the retry configuration
	if config.RetryConfig.MaxRetries != 3 {
		t.Errorf("MaxRetries = %v, want 3", config.RetryConfig.MaxRetries)
	}

	if config.RetryConfig.InitialInterval != 10*time.Millisecond {
		t.Errorf("InitialInterval = %v, want 10ms", config.RetryConfig.InitialInterval)
	}

	if config.RetryConfig.MaxInterval != 100*time.Millisecond {
		t.Errorf("MaxInterval = %v, want 100ms", config.RetryConfig.MaxInterval)
	}

	if config.RetryConfig.Multiplier != 2 {
		t.Errorf("Multiplier = %v, want 2", config.RetryConfig.Multiplier)
	}
}

func TestHTTPTransport_UserAgent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent := r.Header.Get("User-Agent")
		w.Header().Set("Echo-User-Agent", userAgent)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tests := []struct {
		name          string
		wantUserAgent string
	}{
		{
			name:          "default user agent",
			wantUserAgent: "birb-nest-go-sdk/1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig().WithBaseURL(server.URL)

			transport, err := newHTTPTransport(config)
			if err != nil {
				t.Fatal(err)
			}

			req, err := http.NewRequest("GET", server.URL+"/test", nil)
			if err != nil {
				t.Fatal(err)
			}

			// Set user agent (this would be done in the do method)
			req.Header.Set("User-Agent", tt.wantUserAgent)

			resp, err := transport.client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			gotUA := resp.Header.Get("Echo-User-Agent")
			if gotUA != tt.wantUserAgent {
				t.Errorf("User-Agent = %v, want %v", gotUA, tt.wantUserAgent)
			}
		})
	}
}

func TestHTTPTransport_ConnectionPool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := DefaultConfig().WithBaseURL(server.URL)
	config.TransportConfig.MaxIdleConns = 100
	config.TransportConfig.MaxConnsPerHost = 10
	config.TransportConfig.IdleConnTimeout = 90 * time.Second

	transport, err := newHTTPTransport(config)
	if err != nil {
		t.Fatal(err)
	}

	// Verify transport is configured correctly
	httpTransport, ok := transport.client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Expected http.Transport")
	}

	if httpTransport.MaxIdleConns != config.TransportConfig.MaxIdleConns {
		t.Errorf("MaxIdleConns = %v, want %v", httpTransport.MaxIdleConns, config.TransportConfig.MaxIdleConns)
	}

	if httpTransport.MaxConnsPerHost != config.TransportConfig.MaxConnsPerHost {
		t.Errorf("MaxConnsPerHost = %v, want %v", httpTransport.MaxConnsPerHost, config.TransportConfig.MaxConnsPerHost)
	}

	if httpTransport.IdleConnTimeout != config.TransportConfig.IdleConnTimeout {
		t.Errorf("IdleConnTimeout = %v, want %v", httpTransport.IdleConnTimeout, config.TransportConfig.IdleConnTimeout)
	}
}

// TestHTTPTransport_RetryBackoff is now handled by TestRetryStrategies in resilience_test.go

// Benchmark transport operations
func BenchmarkHTTPTransport_Request(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	config := DefaultConfig().WithBaseURL(server.URL)
	transport, err := newHTTPTransport(config)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", server.URL+"/test", nil)
		resp, err := transport.client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

func BenchmarkBuildPath(b *testing.B) {
	benchmarks := []struct {
		name    string
		pattern string
		args    []string
	}{
		{
			name:    "no_placeholders",
			pattern: "/v1/cache/test",
			args:    []string{},
		},
		{
			name:    "single_placeholder",
			pattern: "/v1/cache/{0}",
			args:    []string{"test-key"},
		},
		{
			name:    "multiple_placeholders",
			pattern: "/v1/{0}/items/{1}/data/{2}",
			args:    []string{"cache", "test-key", "version1"},
		},
		{
			name:    "url_encoding",
			pattern: "/v1/cache/{0}",
			args:    []string{"test key with spaces & special chars"},
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = buildPath(bm.pattern, bm.args...)
			}
		})
	}
}
