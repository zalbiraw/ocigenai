// Package ocigenai is a Traefik plugin to proxy requests to OCI Generative AI using Instance Principals.
package ocigenai

// ServingMode represents the serving configuration for Oracle Cloud GenAI.
type ServingMode struct {
	ModelID     string `json:"modelId"`
	ServingType string `json:"servingType"`
}

// StreamOptions configures streaming behavior for chat requests.
type StreamOptions struct {
	IsIncludeUsage bool `json:"isIncludeUsage"`
}

// ChatRequest represents a chat completion request to Oracle Cloud GenAI.
type ChatRequest struct {
	MaxTokens        int           `json:"maxTokens"`
	Temperature      float64       `json:"temperature"`
	FrequencyPenalty float64       `json:"frequencyPenalty"`
	PresencePenalty  float64       `json:"presencePenalty"`
	TopP             float64       `json:"topP"`
	TopK             int           `json:"topK"`
	IsStream         bool          `json:"isStream"`
	StreamOptions    StreamOptions `json:"streamOptions"`
	ChatHistory      []interface{} `json:"chatHistory"`
	Message          string        `json:"message"`
	APIFormat        string        `json:"apiFormat"`
}

// OracleCloudRequest represents the complete request structure for Oracle Cloud GenAI.
type OracleCloudRequest struct {
	CompartmentID string      `json:"compartmentId"`
	ServingMode   ServingMode `json:"servingMode"`
	ChatRequest   ChatRequest `json:"chatRequest"`
}

// MetadataResponse represents the metadata response from Oracle Cloud.
type MetadataResponse struct {
	CertPem         string `json:"certPem"`
	IntermediatePem string `json:"intermediatePem"`
	KeyPem          string `json:"keyPem"`
}
