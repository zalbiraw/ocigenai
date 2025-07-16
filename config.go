// Package ocigenai is a Traefik plugin to proxy requests to OCI Generative AI using Instance Principals.
package ocigenai

// Config the plugin configuration.
type Config struct {
	CompartmentID    string  `json:"compartmentId,omitempty"`
	MaxTokens        int     `json:"maxTokens,omitempty"`
	Temperature      float64 `json:"temperature,omitempty"`
	TopP             float64 `json:"topP,omitempty"`
	FrequencyPenalty float64 `json:"frequencyPenalty,omitempty"`
	PresencePenalty  float64 `json:"presencePenalty,omitempty"`
	TopK             int     `json:"topK,omitempty"`
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		MaxTokens:        600,
		Temperature:      1.0,
		TopP:             0.75,
		FrequencyPenalty: 0.0,
		PresencePenalty:  0.0,
		TopK:             0,
	}
}
