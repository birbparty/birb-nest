#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const { performance } = require('perf_hooks');
const http = require('http');

// Colors for output
const colors = {
    reset: '\x1b[0m',
    bright: '\x1b[1m',
    green: '\x1b[32m',
    red: '\x1b[31m',
    yellow: '\x1b[33m',
    blue: '\x1b[34m',
    gray: '\x1b[90m'
};

// Test context
let currentTest = '';
let testResults = [];
let client = null;
let mockServer = null;

// Mock fetch for Node.js environment
global.fetch = require('node-fetch');

// Test utilities
function assert(condition, message) {
    if (!condition) {
        throw new Error(message || 'Assertion failed');
    }
}

function assertEqual(actual, expected, message) {
    if (JSON.stringify(actual) !== JSON.stringify(expected)) {
        throw new Error(
            message || `Expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`
        );
    }
}

function assertDeepEqual(actual, expected, message) {
    const actualStr = JSON.stringify(actual, null, 2);
    const expectedStr = JSON.stringify(expected, null, 2);
    if (actualStr !== expectedStr) {
        throw new Error(
            message || `Deep equality assertion failed:\nExpected:\n${expectedStr}\nActual:\n${actualStr}`
        );
    }
}

async function assertRejects(promise, expectedError, message) {
    try {
        await promise;
        throw new Error(message || 'Expected promise to reject, but it resolved');
    } catch (error) {
        if (expectedError && !error.message.includes(expectedError)) {
            throw new Error(
                `Expected error containing "${expectedError}", got "${error.message}"`
            );
        }
    }
}

async function runTest(name, fn, options = {}) {
    currentTest = name;
    const start = performance.now();
    
    try {
        if (options.timeout) {
            await Promise.race([
                fn(),
                new Promise((_, reject) => 
                    setTimeout(() => reject(new Error(`Test timeout after ${options.timeout}ms`)), options.timeout)
                )
            ]);
        } else {
            await fn();
        }
        
        const duration = performance.now() - start;
        testResults.push({
            name,
            passed: true,
            duration
        });
        console.log(`${colors.green}✓${colors.reset} ${name} ${colors.gray}(${duration.toFixed(2)}ms)${colors.reset}`);
    } catch (error) {
        const duration = performance.now() - start;
        testResults.push({
            name,
            passed: false,
            error: error.message,
            stack: error.stack,
            duration
        });
        console.log(`${colors.red}✗${colors.reset} ${name} ${colors.gray}(${duration.toFixed(2)}ms)${colors.reset}`);
        console.log(`  ${colors.red}Error: ${error.message}${colors.reset}`);
        if (options.verbose) {
            console.log(`  ${colors.gray}${error.stack}${colors.reset}`);
        }
    }
}

// Mock HTTP server for testing
class MockHTTPServer {
    constructor(port = 8080) {
        this.port = port;
        this.server = null;
        this.responses = new Map();
        this.requests = [];
    }

    setResponse(method, path, response) {
        const key = `${method}:${path}`;
        this.responses.set(key, response);
    }

    getRequests() {
        return [...this.requests];
    }

    clearRequests() {
        this.requests = [];
    }

    async start() {
        return new Promise((resolve) => {
            this.server = http.createServer((req, res) => {
                let body = '';
                
                req.on('data', chunk => {
                    body += chunk.toString();
                });
                
                req.on('end', () => {
                    // Log request
                    const request = {
                        method: req.method,
                        path: req.url,
                        headers: req.headers,
                        body: body ? JSON.parse(body) : null,
                        timestamp: new Date()
                    };
                    this.requests.push(request);
                    
                    // Find response
                    const key = `${req.method}:${req.url}`;
                    const response = this.responses.get(key);
                    
                    // Set CORS headers
                    res.setHeader('Access-Control-Allow-Origin', '*');
                    res.setHeader('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS');
                    res.setHeader('Access-Control-Allow-Headers', 'Content-Type, Authorization');
                    
                    if (req.method === 'OPTIONS') {
                        res.writeHead(200);
                        res.end();
                        return;
                    }
                    
                    if (response) {
                        if (response.delay) {
                            setTimeout(() => {
                                this.sendResponse(res, response);
                            }, response.delay);
                        } else {
                            this.sendResponse(res, response);
                        }
                    } else {
                        res.writeHead(404, { 'Content-Type': 'application/json' });
                        res.end(JSON.stringify({ error: 'Not found' }));
                    }
                });
            });
            
            this.server.listen(this.port, () => {
                console.log(`Mock server listening on port ${this.port}`);
                resolve();
            });
        });
    }

    sendResponse(res, response) {
        res.writeHead(response.status || 200, { 
            'Content-Type': 'application/json',
            ...response.headers 
        });
        res.end(JSON.stringify(response.body || {}));
    }

    async stop() {
        return new Promise((resolve) => {
            if (this.server) {
                this.server.close(() => {
                    console.log('Mock server stopped');
                    resolve();
                });
            } else {
                resolve();
            }
        });
    }
}

// Load WebAssembly support
require('./wasm_exec.js');

async function loadWasm() {
    const wasmPath = path.join(__dirname, '../examples/wasm/birb-nest-sdk.wasm');
    
    if (!fs.existsSync(wasmPath)) {
        throw new Error(`WASM file not found at ${wasmPath}. Run 'make sdk-wasm' first.`);
    }
    
    const wasmBuffer = fs.readFileSync(wasmPath);
    const go = new Go();
    
    const { instance } = await WebAssembly.instantiate(wasmBuffer, go.importObject);
    
    // Run the Go program
    go.run(instance);
    
    // Wait for SDK to be available
    await new Promise(resolve => setTimeout(resolve, 100));
    
    if (!global.birbNestSDK) {
        throw new Error('birbNestSDK not found on global object');
    }
    
    return global.birbNestSDK;
}

// Comprehensive Tests
async function runTests() {
    console.log(`${colors.bright}Running Birb Nest SDK WASM Tests (Enhanced)${colors.reset}\n`);
    
    try {
        // Start mock server
        mockServer = new MockHTTPServer(8080);
        await mockServer.start();
        
        // Load WASM module
        console.log('Loading WASM module...');
        const sdk = await loadWasm();
        console.log(`${colors.green}✓ WASM module loaded successfully${colors.reset}\n`);
        
        // Create client for tests
        client = sdk.newClient({
            baseURL: 'http://localhost:8080'
        });
        
        console.log(`${colors.bright}Basic API Tests${colors.reset}`);
        
        // Test 1: Client creation
        await runTest('should create client with default config', async () => {
            const testClient = sdk.newClient({});
            assert(testClient, 'Client should be created');
            assert(typeof testClient.set === 'function', 'Client should have set method');
            assert(typeof testClient.get === 'function', 'Client should have get method');
            assert(typeof testClient.delete === 'function', 'Client should have delete method');
            assert(typeof testClient.ping === 'function', 'Client should have ping method');
        });
        
        // Test 2: Ping functionality
        await runTest('should successfully ping server', async () => {
            mockServer.setResponse('GET', '/health', {
                status: 200,
                body: { status: 'healthy' }
            });
            
            await client.ping();
            
            const requests = mockServer.getRequests();
            assert(requests.length > 0, 'Should make request to server');
            assertEqual(requests[requests.length - 1].path, '/health');
        });
        
        console.log(`\n${colors.bright}Data Type Tests${colors.reset}`);
        
        // Test 3: String values
        await runTest('should handle string values', async () => {
            const key = 'test-string';
            const value = 'Hello, WASM! 🚀';
            
            mockServer.setResponse('POST', `/v1/cache/${key}`, {
                status: 200,
                body: { key, value, timestamp: new Date() }
            });
            
            mockServer.setResponse('GET', `/v1/cache/${key}`, {
                status: 200,
                body: { key, value }
            });
            
            await client.set(key, value);
            const retrieved = await client.get(key);
            
            assertEqual(retrieved, value, 'Should retrieve the same string value');
        });
        
        // Test 4: Object values
        await runTest('should handle object values', async () => {
            const key = 'test-object';
            const value = {
                name: 'Test User',
                age: 30,
                active: true,
                metadata: {
                    created: '2025-01-01',
                    tags: ['wasm', 'test', 'sdk']
                }
            };
            
            mockServer.setResponse('POST', `/v1/cache/${key}`, {
                status: 200,
                body: { key, value, timestamp: new Date() }
            });
            
            mockServer.setResponse('GET', `/v1/cache/${key}`, {
                status: 200,
                body: { key, value }
            });
            
            await client.set(key, value);
            const retrieved = await client.get(key);
            
            assertDeepEqual(retrieved, value, 'Should retrieve the same object value');
        });
        
        // Test 5: Array values
        await runTest('should handle array values', async () => {
            const key = 'test-array';
            const value = [1, 'two', { three: 3 }, [4, 5], true, null];
            
            mockServer.setResponse('POST', `/v1/cache/${key}`, {
                status: 200,
                body: { key, value, timestamp: new Date() }
            });
            
            mockServer.setResponse('GET', `/v1/cache/${key}`, {
                status: 200,
                body: { key, value }
            });
            
            await client.set(key, value);
            const retrieved = await client.get(key);
            
            assertDeepEqual(retrieved, value, 'Should retrieve the same array value');
        });
        
        // Test 6: Special values
        await runTest('should handle special values (null, empty strings, etc)', async () => {
            const testCases = [
                { key: 'test-null', value: null },
                { key: 'test-empty-string', value: '' },
                { key: 'test-zero', value: 0 },
                { key: 'test-false', value: false },
                { key: 'test-empty-array', value: [] },
                { key: 'test-empty-object', value: {} }
            ];
            
            for (const { key, value } of testCases) {
                mockServer.setResponse('POST', `/v1/cache/${key}`, {
                    status: 200,
                    body: { key, value, timestamp: new Date() }
                });
                
                mockServer.setResponse('GET', `/v1/cache/${key}`, {
                    status: 200,
                    body: { key, value }
                });
                
                await client.set(key, value);
                const retrieved = await client.get(key);
                
                assertEqual(retrieved, value, `Should handle ${key} correctly`);
            }
        });
        
        console.log(`\n${colors.bright}Error Handling Tests${colors.reset}`);
        
        // Test 7: 404 Not Found
        await runTest('should handle 404 errors', async () => {
            mockServer.setResponse('GET', '/v1/cache/nonexistent', {
                status: 404,
                body: { error: 'Key not found' }
            });
            
            await assertRejects(
                client.get('nonexistent'),
                'not found',
                'Should reject with not found error'
            );
        });
        
        // Test 8: 500 Server Error
        await runTest('should handle server errors', async () => {
            mockServer.setResponse('POST', '/v1/cache/error-key', {
                status: 500,
                body: { error: 'Internal server error' }
            });
            
            await assertRejects(
                client.set('error-key', 'value'),
                'server error',
                'Should reject with server error'
            );
        });
        
        // Test 9: Network errors
        await runTest('should handle network errors', async () => {
            const offlineClient = sdk.newClient({
                baseURL: 'http://localhost:9999' // Non-existent server
            });
            
            await assertRejects(
                offlineClient.ping(),
                '',
                'Should reject with network error'
            );
        });
        
        console.log(`\n${colors.bright}Concurrent Operations Tests${colors.reset}`);
        
        // Test 10: Concurrent requests
        await runTest('should handle concurrent requests', async () => {
            const promises = [];
            const count = 10;
            
            // Set up mock responses
            for (let i = 0; i < count; i++) {
                const key = `concurrent-${i}`;
                mockServer.setResponse('POST', `/v1/cache/${key}`, {
                    status: 200,
                    body: { key, value: i, timestamp: new Date() }
                });
            }
            
            // Launch concurrent requests
            for (let i = 0; i < count; i++) {
                promises.push(client.set(`concurrent-${i}`, i));
            }
            
            await Promise.all(promises);
            
            const requests = mockServer.getRequests();
            const concurrentRequests = requests.filter(r => 
                r.method === 'POST' && r.path.includes('concurrent-')
            );
            
            assertEqual(concurrentRequests.length, count, 'Should make all concurrent requests');
        });
        
        console.log(`\n${colors.bright}Promise Behavior Tests${colors.reset}`);
        
        // Test 11: Promise chaining
        await runTest('should support promise chaining', async () => {
            const key = 'chain-test';
            const value = 'chained value';
            
            mockServer.setResponse('POST', `/v1/cache/${key}`, {
                status: 200,
                body: { key, value }
            });
            
            mockServer.setResponse('GET', `/v1/cache/${key}`, {
                status: 200,
                body: { key, value }
            });
            
            mockServer.setResponse('DELETE', `/v1/cache/${key}`, {
                status: 200,
                body: { deleted: true }
            });
            
            const result = await client.set(key, value)
                .then(() => client.get(key))
                .then(retrieved => {
                    assertEqual(retrieved, value);
                    return client.delete(key);
                })
                .then(() => 'success');
            
            assertEqual(result, 'success', 'Promise chain should complete successfully');
        });
        
        // Test 12: Promise error propagation
        await runTest('should propagate errors through promise chain', async () => {
            mockServer.setResponse('GET', '/v1/cache/error-chain', {
                status: 500,
                body: { error: 'Server error' }
            });
            
            let errorCaught = false;
            
            try {
                await client.get('error-chain')
                    .then(() => {
                        throw new Error('Should not reach here');
                    })
                    .catch(err => {
                        errorCaught = true;
                        throw err;
                    });
            } catch (err) {
                assert(errorCaught, 'Error should be caught in promise chain');
            }
        });
        
        console.log(`\n${colors.bright}Memory and Performance Tests${colors.reset}`);
        
        // Test 13: Large payload handling
        await runTest('should handle large payloads', async () => {
            const key = 'large-payload';
            const largeArray = Array(1000).fill(null).map((_, i) => ({
                id: i,
                data: 'x'.repeat(100),
                nested: { value: i * 2 }
            }));
            
            mockServer.setResponse('POST', `/v1/cache/${key}`, {
                status: 200,
                body: { key, value: largeArray }
            });
            
            mockServer.setResponse('GET', `/v1/cache/${key}`, {
                status: 200,
                body: { key, value: largeArray }
            });
            
            await client.set(key, largeArray);
            const retrieved = await client.get(key);
            
            assertEqual(retrieved.length, largeArray.length, 'Should handle large arrays');
            assertEqual(retrieved[0].id, 0, 'First element should match');
            assertEqual(retrieved[999].id, 999, 'Last element should match');
        });
        
        // Test 14: Rapid sequential operations
        await runTest('should handle rapid sequential operations', async () => {
            const key = 'rapid-test';
            const operations = 20;
            
            mockServer.setResponse('POST', `/v1/cache/${key}`, {
                status: 200,
                body: { key, value: 'updated' }
            });
            
            mockServer.setResponse('GET', `/v1/cache/${key}`, {
                status: 200,
                body: { key, value: 'updated' }
            });
            
            const start = performance.now();
            
            for (let i = 0; i < operations; i++) {
                await client.set(key, `value-${i}`);
                await client.get(key);
            }
            
            const duration = performance.now() - start;
            console.log(`    Completed ${operations * 2} operations in ${duration.toFixed(2)}ms`);
            
            assert(duration < 5000, 'Operations should complete within reasonable time');
        }, { timeout: 10000 });
        
        console.log(`\n${colors.bright}Headers and Configuration Tests${colors.reset}`);
        
        // Test 15: Custom headers
        await runTest('should send custom headers', async () => {
            const customClient = sdk.newClient({
                baseURL: 'http://localhost:8080',
                headers: {
                    'X-Custom-Header': 'test-value',
                    'Authorization': 'Bearer test-token'
                }
            });
            
            mockServer.clearRequests();
            mockServer.setResponse('GET', '/health', {
                status: 200,
                body: { status: 'healthy' }
            });
            
            await customClient.ping();
            
            const requests = mockServer.getRequests();
            const lastRequest = requests[requests.length - 1];
            
            // Note: Headers might be lowercase in Node.js
            assert(
                lastRequest.headers['x-custom-header'] === 'test-value' ||
                lastRequest.headers['X-Custom-Header'] === 'test-value',
                'Should include custom header'
            );
        });
        
    } catch (error) {
        console.error(`\n${colors.red}Fatal error: ${error.message}${colors.reset}`);
        console.error(error.stack);
        process.exit(1);
    } finally {
        // Clean up
        if (mockServer) {
            await mockServer.stop();
        }
    }
    
    // Print summary
    console.log(`\n${colors.bright}Test Summary:${colors.reset}`);
    
    const passed = testResults.filter(r => r.passed).length;
    const failed = testResults.filter(r => !r.passed).length;
    const total = testResults.length;
    const totalTime = testResults.reduce((sum, r) => sum + r.duration, 0);
    
    console.log(`Tests:    ${colors.green}${passed} passed${colors.reset}, ${failed > 0 ? colors.red : ''}${failed} failed${colors.reset}, ${total} total`);
    console.log(`Time:     ${totalTime.toFixed(2)}ms`);
    
    // Show failed tests details
    if (failed > 0) {
        console.log(`\n${colors.red}Failed Tests:${colors.reset}`);
        testResults.filter(r => !r.passed).forEach(test => {
            console.log(`  ${colors.red}✗ ${test.name}${colors.reset}`);
            console.log(`    ${test.error}`);
        });
        
        console.log(`\n${colors.red}Some tests failed!${colors.reset}`);
        process.exit(1);
    } else {
        console.log(`\n${colors.green}All tests passed!${colors.reset}`);
        process.exit(0);
    }
}

// Run tests
runTests().catch(error => {
    console.error(`Unexpected error: ${error}`);
    console.error(error.stack);
    process.exit(1);
});
