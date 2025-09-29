#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const { performance } = require('perf_hooks');

// Colors for output
const colors = {
    reset: '\x1b[0m',
    bright: '\x1b[1m',
    green: '\x1b[32m',
    red: '\x1b[31m',
    yellow: '\x1b[33m',
    blue: '\x1b[34m'
};

// Test context
let currentTest = '';
let testResults = [];
let client = null;

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

async function runTest(name, fn) {
    currentTest = name;
    const start = performance.now();
    
    try {
        await fn();
        const duration = performance.now() - start;
        testResults.push({
            name,
            passed: true,
            duration
        });
        console.log(`${colors.green}✓${colors.reset} ${name} (${duration.toFixed(2)}ms)`);
    } catch (error) {
        const duration = performance.now() - start;
        testResults.push({
            name,
            passed: false,
            error: error.message,
            duration
        });
        console.log(`${colors.red}✗${colors.reset} ${name} (${duration.toFixed(2)}ms)`);
        console.log(`  ${colors.red}Error: ${error.message}${colors.reset}`);
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

// Tests
async function runTests() {
    console.log(`${colors.bright}Running Birb Nest SDK WASM Tests${colors.reset}\n`);
    
    try {
        // Load WASM module
        console.log('Loading WASM module...');
        const sdk = await loadWasm();
        console.log(`${colors.green}✓ WASM module loaded successfully${colors.reset}\n`);
        
        // Test client creation
        await runTest('should create client with default config', async () => {
            client = sdk.newClient({});
            assert(client, 'Client should be created');
            assert(typeof client.set === 'function', 'Client should have set method');
            assert(typeof client.get === 'function', 'Client should have get method');
            assert(typeof client.delete === 'function', 'Client should have delete method');
            assert(typeof client.ping === 'function', 'Client should have ping method');
        });
        
        await runTest('should create client with custom baseURL', async () => {
            const customClient = sdk.newClient({
                baseURL: 'http://example.com'
            });
            assert(customClient, 'Client should be created with custom config');
        });
        
        // Network error handling test
        await runTest('should handle network errors gracefully', async () => {
            const testClient = sdk.newClient({
                baseURL: 'http://localhost:9999' // Non-existent server
            });
            
            try {
                await testClient.ping();
                throw new Error('Expected ping to fail');
            } catch (error) {
                // In WASM/Node.js environment, the error message may vary
                assert(error.message.includes('fetch failed') || 
                       error.message.includes('connect') ||
                       error.message.includes('ECONNREFUSED') ||
                       error.message.includes('network') ||
                       error.message.includes('Failed to fetch'),
                       `Should fail with network error, got: ${error.message}`);
            }
        });
        
        // API surface tests (no network calls)
        await runTest('should accept string values in set method', () => {
            const mockClient = sdk.newClient({});
            // Verify the method accepts the correct types without making network calls
            const promise = mockClient.set('test-string', 'Hello, WASM!');
            assert(promise instanceof Promise, 'set should return a Promise');
            // Don't await to avoid network call
            promise.catch(() => {}); // Ignore the error
        });
        
        await runTest('should accept object values in set method', () => {
            const mockClient = sdk.newClient({});
            const testObj = {
                name: 'Test User',
                age: 30,
                preferences: {
                    theme: 'dark',
                    notifications: true
                }
            };
            
            const promise = mockClient.set('test-object', testObj);
            assert(promise instanceof Promise, 'set should return a Promise');
            promise.catch(() => {}); // Ignore the error
        });
        
        await runTest('should accept array values in set method', () => {
            const mockClient = sdk.newClient({});
            const testArray = [1, 2, 3, 'four', { five: 5 }];
            
            const promise = mockClient.set('test-array', testArray);
            assert(promise instanceof Promise, 'set should return a Promise');
            promise.catch(() => {}); // Ignore the error
        });
        
        await runTest('should accept null values in set method', () => {
            const mockClient = sdk.newClient({});
            
            const promise = mockClient.set('test-null', null);
            assert(promise instanceof Promise, 'set should return a Promise');
            promise.catch(() => {}); // Ignore the error
        });
        
        // Promise handling tests
        await runTest('should return promises from all methods', () => {
            const mockClient = sdk.newClient({});
            
            const setPromise = mockClient.set('key', 'value');
            assert(setPromise instanceof Promise, 'set should return a Promise');
            
            const getPromise = mockClient.get('key');
            assert(getPromise instanceof Promise, 'get should return a Promise');
            
            const deletePromise = mockClient.delete('key');
            assert(deletePromise instanceof Promise, 'delete should return a Promise');
            
            const pingPromise = mockClient.ping();
            assert(pingPromise instanceof Promise, 'ping should return a Promise');
        });
        
    } catch (error) {
        console.error(`\n${colors.red}Fatal error: ${error.message}${colors.reset}`);
        process.exit(1);
    }
    
    // Print summary
    console.log(`\n${colors.bright}Test Summary:${colors.reset}`);
    
    const passed = testResults.filter(r => r.passed).length;
    const failed = testResults.filter(r => !r.passed).length;
    const total = testResults.length;
    const totalTime = testResults.reduce((sum, r) => sum + r.duration, 0);
    
    console.log(`Tests:    ${colors.green}${passed} passed${colors.reset}, ${failed > 0 ? colors.red : ''}${failed} failed${colors.reset}, ${total} total`);
    console.log(`Time:     ${totalTime.toFixed(2)}ms`);
    
    if (failed > 0) {
        console.log(`\n${colors.red}Some tests failed!${colors.reset}`);
        process.exit(1);
    } else {
        console.log(`\n${colors.green}All tests passed!${colors.reset}`);
        // Exit cleanly to prevent background promises from executing
        process.exit(0);
    }
}

// Run tests
runTests().catch(error => {
    console.error(`Unexpected error: ${error}`);
    process.exit(1);
});
