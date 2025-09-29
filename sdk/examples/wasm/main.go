//go:build wasm

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"syscall/js"

	sdk "github.com/birbparty/birb-nest/sdk"
)

// clientWrapper wraps the SDK client for JavaScript access
type clientWrapper struct {
	client sdk.Client
}

func main() {
	// Create global birbNestSDK object
	global := js.Global()
	birbNestSDK := make(map[string]interface{})

	// Register newClient function
	birbNestSDK["newClient"] = js.FuncOf(newClient)

	// Set global object
	global.Set("birbNestSDK", birbNestSDK)

	fmt.Println("Birb Nest SDK WASM loaded!")

	// Keep the program running
	select {}
}

// newClient creates a new SDK client from JavaScript config
func newClient(this js.Value, args []js.Value) interface{} {
	if len(args) != 1 {
		return jsError("newClient requires exactly one argument")
	}

	// Parse config from JavaScript object
	configObj := args[0]
	config := sdk.DefaultConfig()

	if baseURL := configObj.Get("baseURL"); !baseURL.IsUndefined() {
		config.BaseURL = baseURL.String()
	}

	// Create the client
	client, err := sdk.NewClient(config)
	if err != nil {
		return jsError(fmt.Sprintf("failed to create client: %v", err))
	}

	// Create wrapper
	wrapper := &clientWrapper{client: client}

	// Return JavaScript object with methods
	clientObj := make(map[string]interface{})
	clientObj["set"] = js.FuncOf(wrapper.set)
	clientObj["get"] = js.FuncOf(wrapper.get)
	clientObj["delete"] = js.FuncOf(wrapper.delete)
	clientObj["ping"] = js.FuncOf(wrapper.ping)

	return clientObj
}

// set stores a value in the cache
func (w *clientWrapper) set(this js.Value, args []js.Value) interface{} {
	return jsPromise(func() (interface{}, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("set requires exactly two arguments: key and value")
		}

		key := args[0].String()

		// Convert JavaScript value to Go value
		value := jsValueToGo(args[1])

		// Set the value
		err := w.client.Set(context.Background(), key, value)
		if err != nil {
			return nil, err
		}

		return js.Undefined(), nil
	})
}

// get retrieves a value from the cache
func (w *clientWrapper) get(this js.Value, args []js.Value) interface{} {
	return jsPromise(func() (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("get requires exactly one argument: key")
		}

		key := args[0].String()

		// Get the value
		var result interface{}
		err := w.client.Get(context.Background(), key, &result)
		if err != nil {
			return nil, err
		}

		// Convert Go value to JavaScript value
		return goValueToJS(result), nil
	})
}

// delete removes a value from the cache
func (w *clientWrapper) delete(this js.Value, args []js.Value) interface{} {
	return jsPromise(func() (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("delete requires exactly one argument: key")
		}

		key := args[0].String()

		// Delete the value
		err := w.client.Delete(context.Background(), key)
		if err != nil {
			return nil, err
		}

		return js.Undefined(), nil
	})
}

// ping checks server connectivity
func (w *clientWrapper) ping(this js.Value, args []js.Value) interface{} {
	return jsPromise(func() (interface{}, error) {
		err := w.client.Ping(context.Background())
		if err != nil {
			return nil, err
		}

		return js.Undefined(), nil
	})
}

// jsPromise creates a JavaScript promise from a Go function
func jsPromise(fn func() (interface{}, error)) js.Value {
	promise := js.Global().Get("Promise")

	return promise.New(js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		resolve := args[0]
		reject := args[1]

		// Run async
		go func() {
			result, err := fn()
			if err != nil {
				reject.Invoke(jsError(err.Error()))
			} else {
				resolve.Invoke(result)
			}
		}()

		return nil
	}))
}

// jsError creates a JavaScript Error object
func jsError(message string) js.Value {
	errorConstructor := js.Global().Get("Error")
	return errorConstructor.New(message)
}

// jsValueToGo converts a JavaScript value to a Go value
func jsValueToGo(val js.Value) interface{} {
	switch val.Type() {
	case js.TypeNull, js.TypeUndefined:
		return nil
	case js.TypeBoolean:
		return val.Bool()
	case js.TypeNumber:
		return val.Float()
	case js.TypeString:
		return val.String()
	case js.TypeObject:
		// Try to convert to JSON and back to Go
		jsonStr := js.Global().Get("JSON").Call("stringify", val).String()
		var result interface{}
		json.Unmarshal([]byte(jsonStr), &result)
		return result
	default:
		return nil
	}
}

// goValueToJS converts a Go value to a JavaScript value
func goValueToJS(val interface{}) js.Value {
	if val == nil {
		return js.Null()
	}

	// Convert to JSON and parse in JavaScript
	jsonBytes, err := json.Marshal(val)
	if err != nil {
		return js.Null()
	}

	return js.Global().Get("JSON").Call("parse", string(jsonBytes))
}
