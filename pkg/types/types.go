// Package types defines the data structures used throughout the OCI GenAI proxy plugin.
package types

// ChatCompletionMessage represents a message in a chat completion conversation.
type ChatCompletionMessage struct {
	// Role is the role of the author of this message (e.g., "user", "assistant", "system")
	Role string `json:"role"`

	// Content is the content of the message
	Content string `json:"content"`
}

// ChatCompletionRequest represents a request to the OpenAI chat completion API.
type ChatCompletionRequest struct {
	// Model is the ID of the model to use
	Model string `json:"model"`

	// Messages is a list of messages comprising the conversation so far
	Messages []ChatCompletionMessage `json:"messages"`

	// MaxTokens is the maximum number of tokens to generate in the chat completion
	MaxTokens int `json:"maxTokens,omitempty"`

	// Temperature controls randomness (0.0 = deterministic, 2.0 = very random)
	Temperature float32 `json:"temperature,omitempty"`

	// TopP controls nucleus sampling
	TopP float32 `json:"topP,omitempty"`

	// FrequencyPenalty reduces repetition of tokens based on their frequency
	FrequencyPenalty float32 `json:"frequencyPenalty,omitempty"`

	// PresencePenalty reduces repetition of tokens based on their presence
	PresencePenalty float32 `json:"presencePenalty,omitempty"`
}

// ServingMode represents the serving configuration for Oracle Cloud GenAI.
// It specifies which model to use and how it should be served.
type ServingMode struct {
	// ModelID is the identifier of the AI model to use (e.g., "gpt-4", "claude-3")
	ModelID string `json:"modelId"`

	// ServingType specifies how the model is served (typically "ON_DEMAND")
	ServingType string `json:"servingType"`
}

// StreamOptions configures streaming behavior for chat requests.
// This controls whether the response should include usage statistics.
type StreamOptions struct {
	// IsIncludeUsage determines if usage statistics should be included in streaming responses
	IsIncludeUsage bool `json:"isIncludeUsage"`
}

// ChatRequest represents a chat completion request to Oracle Cloud GenAI.
// It contains all the parameters needed to generate a response from the AI model.
type ChatRequest struct {
	// MaxTokens is the maximum number of tokens to generate in the response
	MaxTokens int `json:"maxTokens"`

	// Temperature controls randomness in the response (0.0 = deterministic, 1.0 = very random)
	Temperature float64 `json:"temperature"`

	// FrequencyPenalty reduces repetition of tokens based on their frequency in the text
	FrequencyPenalty float64 `json:"frequencyPenalty"`

	// PresencePenalty reduces repetition of tokens based on whether they appear in the text
	PresencePenalty float64 `json:"presencePenalty"`

	// TopP controls nucleus sampling (0.0 = most focused, 1.0 = least focused)
	TopP float64 `json:"topP"`

	// TopK limits the number of highest probability tokens to consider
	TopK int `json:"topK"`

	// IsStream determines if the response should be streamed
	IsStream bool `json:"isStream"`

	// StreamOptions configures streaming behavior
	StreamOptions StreamOptions `json:"streamOptions"`

	// ChatHistory contains previous messages in the conversation
	ChatHistory []interface{} `json:"chatHistory"`

	// Message is the current user message to process
	Message string `json:"message"`

	// APIFormat specifies the API format to use (e.g., "COHERE")
	APIFormat string `json:"apiFormat"`
}

// OracleCloudRequest represents the complete request structure for Oracle Cloud GenAI.
// This is the final format that gets sent to the OCI GenAI service.
type OracleCloudRequest struct {
	// CompartmentID is the OCI compartment where the GenAI service is located
	CompartmentID string `json:"compartmentId"`

	// ServingMode specifies the model and serving configuration
	ServingMode ServingMode `json:"servingMode"`

	// ChatRequest contains the actual chat parameters and message
	ChatRequest ChatRequest `json:"chatRequest"`
}

// InstanceMetadata represents the metadata response from Oracle Cloud Instance Metadata Service.
// This contains the certificates and private key needed for Instance Principal authentication.
type InstanceMetadata struct {
	// CertPem is the instance certificate in PEM format
	CertPem string `json:"certPem"`

	// IntermediatePem is the intermediate certificate in PEM format
	IntermediatePem string `json:"intermediatePem"`

	// KeyPem is the private key in PEM format
	KeyPem string `json:"keyPem"`
}
