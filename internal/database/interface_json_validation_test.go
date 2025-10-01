package database

import (
	"encoding/json"
	"testing"
)

// TestJSONValidation tests the JSON validation logic used in SetWithInstance
func TestJSONValidation(t *testing.T) {
	tests := []struct {
		name          string
		input         []byte
		shouldBeValid bool
		description   string
	}{
		{
			name:          "valid JSON object",
			input:         []byte(`{"key": "value"}`),
			shouldBeValid: true,
			description:   "Valid JSON object should pass validation",
		},
		{
			name:          "valid JSON array",
			input:         []byte(`["item1", "item2"]`),
			shouldBeValid: true,
			description:   "Valid JSON array should pass validation",
		},
		{
			name:          "valid JSON string",
			input:         []byte(`"hello world"`),
			shouldBeValid: true,
			description:   "Valid JSON string should pass validation",
		},
		{
			name:          "valid JSON number",
			input:         []byte(`42`),
			shouldBeValid: true,
			description:   "Valid JSON number should pass validation",
		},
		{
			name:          "valid JSON boolean",
			input:         []byte(`true`),
			shouldBeValid: true,
			description:   "Valid JSON boolean should pass validation",
		},
		{
			name:          "valid JSON null",
			input:         []byte(`null`),
			shouldBeValid: true,
			description:   "Valid JSON null should pass validation",
		},
		{
			name:          "hex string (like production error)",
			input:         []byte("9821f3fe"),
			shouldBeValid: false,
			description:   "Raw hex string should fail validation (like production errors)",
		},
		{
			name:          "plain text",
			input:         []byte("hello"),
			shouldBeValid: false,
			description:   "Plain text should fail validation",
		},
		{
			name:          "binary data",
			input:         []byte{0x00, 0x01, 0x02, 0x03},
			shouldBeValid: false,
			description:   "Binary data should fail validation",
		},
		{
			name:          "malformed JSON",
			input:         []byte(`{"key": value}`),
			shouldBeValid: false,
			description:   "Malformed JSON should fail validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := json.Valid(tt.input)

			if isValid != tt.shouldBeValid {
				t.Errorf("%s: got valid=%v, want valid=%v for input: %q",
					tt.description, isValid, tt.shouldBeValid, string(tt.input))
			}

			// If invalid, test that marshaling wraps it properly
			if !isValid {
				wrapped, err := json.Marshal(tt.input)
				if err != nil {
					t.Errorf("Failed to marshal invalid input: %v", err)
				}

				// Verify the wrapped version is valid JSON
				if !json.Valid(wrapped) {
					t.Errorf("Wrapped value is not valid JSON: %q", string(wrapped))
				}

				t.Logf("Invalid input %q was wrapped as: %s", string(tt.input), string(wrapped))
			}
		})
	}
}

// TestJSONValidationWithProductionErrors tests specific error cases from production logs
func TestJSONValidationWithProductionErrors(t *testing.T) {
	productionErrors := []string{
		"9821f3fe",
		"e62ec113",
		"6854e8b1",
		"bb3054cc",
		"6cd1cb44",
		"5af6d894",
		"1c38d081",
	}

	for _, errToken := range productionErrors {
		t.Run("production_error_"+errToken, func(t *testing.T) {
			input := []byte(errToken)

			// These should all be invalid JSON
			if json.Valid(input) {
				t.Errorf("Expected %q to be invalid JSON, but it was valid", errToken)
			}

			// Verify wrapping makes them valid
			wrapped, err := json.Marshal(input)
			if err != nil {
				t.Errorf("Failed to wrap %q: %v", errToken, err)
			}

			if !json.Valid(wrapped) {
				t.Errorf("Wrapped version of %q is not valid JSON: %q", errToken, string(wrapped))
			}

			t.Logf("Production error token %q wrapped as: %s", errToken, string(wrapped))
		})
	}
}
