// Package config provides configuration management for the OCI GenAI proxy plugin.
// It handles plugin configuration with sensible defaults and validation.
package config

import (
	"fmt"
)

// Config represents the plugin configuration with all available options.
// These settings control the behavior of the OCI GenAI proxy and provide
// default values for AI model parameters.
type Config struct {
	// CompartmentID is the OCI compartment ID where the GenAI service is located.
	// This is required and must be provided in the plugin configuration.
	CompartmentID string `json:"compartmentId,omitempty"`

	// MaxTokens is the default maximum number of tokens to generate.
	// This can be overridden by individual requests.
	MaxTokens int `json:"maxTokens,omitempty"`

	// Temperature controls the randomness of the AI responses.
	// Range: 0.0 (deterministic) to 2.0 (very random). Default: 1.0
	Temperature float64 `json:"temperature,omitempty"`

	// TopP controls nucleus sampling for response generation.
	// Range: 0.0 (most focused) to 1.0 (least focused). Default: 0.75
	TopP float64 `json:"topP,omitempty"`

	// FrequencyPenalty reduces repetition based on token frequency.
	// Range: -2.0 to 2.0. Default: 0.0 (no penalty)
	FrequencyPenalty float64 `json:"frequencyPenalty,omitempty"`

	// PresencePenalty reduces repetition based on token presence.
	// Range: -2.0 to 2.0. Default: 0.0 (no penalty)
	PresencePenalty float64 `json:"presencePenalty,omitempty"`

	// TopK limits the number of highest probability tokens to consider.
	// 0 means no limit. Default: 0
	TopK int `json:"topK,omitempty"`
}

// New creates a new configuration with sensible defaults.
// These defaults are based on common use cases and provide a good starting point.
func New() *Config {
	return &Config{
		MaxTokens:        600,  // Reasonable default for most conversations
		Temperature:      1.0,  // Balanced creativity and coherence
		TopP:             0.75, // Good balance of diversity and focus
		FrequencyPenalty: 0.0,  // No repetition penalty by default
		PresencePenalty:  0.0,  // No presence penalty by default
		TopK:             0,    // No token limit by default
	}
}

// Validate checks if the configuration is valid and returns an error if not.
// Currently, it only validates that the required CompartmentID is provided.
func (c *Config) Validate() error {
	if c.CompartmentID == "" {
		return fmt.Errorf("compartmentId is required and cannot be empty")
	}

	// Additional validation could be added here for parameter ranges
	if c.Temperature < 0.0 || c.Temperature > 2.0 {
		return fmt.Errorf("temperature must be between 0.0 and 2.0, got %f", c.Temperature)
	}

	if c.TopP < 0.0 || c.TopP > 1.0 {
		return fmt.Errorf("topP must be between 0.0 and 1.0, got %f", c.TopP)
	}

	if c.FrequencyPenalty < -2.0 || c.FrequencyPenalty > 2.0 {
		return fmt.Errorf("frequencyPenalty must be between -2.0 and 2.0, got %f", c.FrequencyPenalty)
	}

	if c.PresencePenalty < -2.0 || c.PresencePenalty > 2.0 {
		return fmt.Errorf("presencePenalty must be between -2.0 and 2.0, got %f", c.PresencePenalty)
	}

	if c.MaxTokens < 1 {
		return fmt.Errorf("maxTokens must be greater than 0, got %d", c.MaxTokens)
	}

	if c.TopK < 0 {
		return fmt.Errorf("topK must be non-negative, got %d", c.TopK)
	}

	return nil
}
