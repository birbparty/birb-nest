# Birb Nest WASM SDK Demo

This example demonstrates how to use the Birb Nest SDK in a web browser using WebAssembly.

## Prerequisites

- Birb Nest API server running (typically on `http://localhost:8080`)
- A web server to serve the demo files (Python's built-in server works great)

## Files

- `main.go` - Go source code that exposes the SDK to JavaScript
- `birb-nest-sdk.wasm` - Compiled WebAssembly module
- `wasm_exec.js` - Go's WebAssembly support library
- `index.html` - Demo web interface

## Running the Demo

1. Make sure the Birb Nest API is running:
   ```bash
   make dev  # From the project root
   ```

2. Build the WASM module (if not already built):
   ```bash
   make sdk-wasm  # From the project root
   ```

3. Start the demo web server:
   ```bash
   make sdk-demo  # From the project root
   # Or manually:
   cd sdk/examples/wasm
   python3 -m http.server 8090
   ```

4. Open your browser to http://localhost:8090

## Using the Demo

The demo provides a simple interface to test all SDK operations:

- **Set Value**: Store a JSON value with a key
- **Get Value**: Retrieve a value by key
- **Delete Value**: Remove a key from the cache
- **Ping Server**: Check server connectivity

### Example Usage

1. Enter a key like `user:123`
2. Enter a JSON value like:
   ```json
   {
     "name": "John Doe",
     "email": "john@example.com",
     "preferences": {
       "theme": "dark",
       "notifications": true
     }
   }
   ```
3. Click "Set Value" to store it
4. Click "Get Value" to retrieve it
5. Click "Delete Value" to remove it

## JavaScript API

The WASM module exposes a global `birbNestSDK` object with the following API:

```javascript
// Create a new client
const client = birbNestSDK.newClient({
  baseURL: 'http://localhost:8080'
});

// All methods return promises

// Set a value
await client.set('key', { some: 'value' });

// Get a value
const value = await client.get('key');

// Delete a value
await client.delete('key');

// Ping the server
await client.ping();
```

## CORS Configuration

For the WASM SDK to work from a browser, the Birb Nest API must be configured to allow CORS requests. The API includes CORS middleware that allows requests from common development origins.

If you're hosting the demo on a different port or domain, you may need to update the CORS configuration in the API.

## Module Size

The current WASM module is approximately 4.2MB, which includes:
- The Go runtime
- The Birb Nest SDK
- JavaScript interop code

This is well within our 10MB target and loads quickly on modern connections.

## Troubleshooting

1. **"Failed to load WASM" error**: Make sure you're serving the files through a web server (not opening index.html directly)

2. **CORS errors**: Ensure the API server is running and configured to accept requests from your origin

3. **"fetch API not available" error**: The browser environment doesn't support the fetch API (very old browsers)

4. **Module not found**: Run `make sdk-wasm` to build the WASM module

## Development

To modify the WASM example:

1. Edit `main.go`
2. Rebuild with `make sdk-wasm`
3. Refresh your browser

The WASM module uses Go's `syscall/js` package to interact with JavaScript. The main.go file shows how to:
- Expose Go functions to JavaScript
- Convert between Go and JavaScript types
- Handle promises for async operations
- Create JavaScript objects and methods
