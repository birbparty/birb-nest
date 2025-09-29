# WASM Testing Suite for Birb Nest SDK

This directory contains a comprehensive testing suite for the WASM implementation of the Birb Nest SDK. The tests cover unit testing, integration testing, and browser-based testing scenarios.

## Test Files

### 1. **wasm_unit_test.go**
Unit tests for WASM-specific functionality written in Go.

**Coverage includes:**
- WASM transport creation and configuration
- Fetch API wrapper functionality
- Backoff calculation for retries
- Data serialization/deserialization
- Error handling and parsing
- Concurrent request handling
- Context cancellation
- Header handling
- Circuit breaker integration
- Memory leak prevention

**Run with:**
```bash
cd sdk
GOOS=js GOARCH=wasm go test -v wasm_unit_test.go
```

### 2. **wasm_runner.js** (Original)
Basic Node.js test runner for WASM module testing.

**Features:**
- WASM module loading
- Basic API surface testing
- Network error handling
- Promise verification

**Run with:**
```bash
cd sdk
make sdk-wasm  # Build WASM module first
node testdata/wasm_runner.js
```

### 3. **wasm_runner_enhanced.js** (Enhanced)
Comprehensive Node.js test runner with mock HTTP server.

**Features:**
- Mock HTTP server for isolated testing
- Comprehensive data type testing (strings, objects, arrays, special values)
- Error handling scenarios (404, 500, network errors)
- Concurrent operations testing
- Promise behavior verification
- Performance benchmarks
- Large payload handling
- Custom header verification

**Run with:**
```bash
cd sdk
make sdk-wasm  # Build WASM module first
node testdata/wasm_runner_enhanced.js
```

### 4. **browser_test.html**
Browser-based test suite with visual test runner.

**Features:**
- Visual test results with pass/fail indicators
- Mock fetch API for offline testing
- Browser-specific API testing
- Real-time test execution monitoring
- Performance metrics
- Automatic test execution in development

**Run with:**
1. Build the WASM module:
   ```bash
   cd sdk
   make sdk-wasm
   ```

2. Start a local web server:
   ```bash
   # Using Python
   python3 -m http.server 8081 --directory sdk

   # Or using Node.js http-server
   npx http-server sdk -p 8081
   ```

3. Open in browser:
   ```
   http://localhost:8081/testdata/browser_test.html
   ```

## Test Categories

### Unit Tests (Go)
- Transport layer functionality
- Serialization correctness
- Error type preservation
- Concurrent execution safety
- Memory management

### Integration Tests (Node.js)
- End-to-end API functionality
- Network resilience
- Data integrity
- Performance under load
- Error propagation

### Browser Tests (HTML/JS)
- Browser compatibility
- CORS handling
- Browser API integration
- Visual feedback
- Real-world usage scenarios

## Mock Server

The enhanced test runner includes a mock HTTP server that:
- Simulates the Birb Nest API
- Supports configurable responses
- Tracks all requests for verification
- Handles CORS headers
- Supports delayed responses

## Best Practices

1. **Always build WASM before testing:**
   ```bash
   make sdk-wasm
   ```

2. **Run all test suites:**
   ```bash
   # Unit tests
   GOOS=js GOARCH=wasm go test -v ./...
   
   # Node.js tests
   node testdata/wasm_runner_enhanced.js
   
   # Browser tests (manual)
   # Open browser_test.html in a web browser
   ```

3. **Check WASM size:**
   ```bash
   make sdk-size-check
   ```

## Debugging

### Common Issues

1. **WASM module not found:**
   - Ensure you've run `make sdk-wasm`
   - Check the path in test files points to the correct location

2. **Fetch API errors in Node.js:**
   - Ensure `node-fetch` is installed: `npm install node-fetch`
   - Check Node.js version (14+ recommended)

3. **Browser CORS errors:**
   - Use a local web server, not file:// protocol
   - Ensure mock server has proper CORS headers

### Debug Output

- Node.js tests: Add `--verbose` flag or check console output
- Browser tests: Open browser developer console
- Go tests: Use `-v` flag for verbose output

## Performance Benchmarks

The test suite includes performance measurements for:
- Serialization/deserialization overhead
- Concurrent request handling
- Large payload processing
- Memory usage patterns

Results are displayed in test output with timing information.

## Future Enhancements

1. **Automated browser testing** with Puppeteer/Playwright
2. **WebSocket support** testing
3. **Service Worker integration** tests
4. **Cross-browser compatibility** matrix
5. **WASM streaming compilation** tests
6. **Memory profiling** tools
7. **Visual regression testing** for browser UI

## Contributing

When adding new tests:
1. Follow the existing test patterns
2. Include both positive and negative test cases
3. Test edge cases and error conditions
4. Add performance benchmarks for critical paths
5. Document any special setup requirements
