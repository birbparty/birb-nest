//go:build wasm

package sdk

import (
	"testing"
)

// TestWASMBuild verifies that the SDK compiles successfully for WASM target
func TestWASMBuild(t *testing.T) {
	// This test exists primarily to ensure WASM compilation works
	// The actual functionality is tested via the Node.js test runner

	// Verify that we can create a client config
	config := DefaultConfig()
	if config.BaseURL == "" {
		t.Error("DefaultConfig should set a base URL")
	}

	// Verify client creation doesn't panic
	client, err := NewClient(config)
	if err != nil {
		t.Errorf("Failed to create client: %v", err)
	}

	if client == nil {
		t.Error("Client should not be nil")
	}
}

// TestWASMClientInterface verifies the client implements the expected interface
func TestWASMClientInterface(t *testing.T) {
	// This test verifies that the WASM client implements all required methods
	config := DefaultConfig()
	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Verify client implements the Client interface
	var _ Client = client

	// Test that we can create an extended client too
	extClient, err := NewExtendedClient(config)
	if err != nil {
		t.Fatalf("Failed to create extended client: %v", err)
	}

	// Verify extended client implements the ExtendedClient interface
	var _ ExtendedClient = extClient
}
