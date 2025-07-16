// Package ocigenai is a Traefik plugin to proxy requests to OCI Generative AI using Instance Principals.
package ocigenai

// Oracle Cloud request structures
type ServingMode struct {
	ModelID     string `json:"modelId"`
	ServingType string `json:"servingType"`
}

type StreamOptions struct {
	IsIncludeUsage bool `json:"isIncludeUsage"`
}

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

type OracleCloudRequest struct {
	CompartmentID string      `json:"compartmentId"`
	ServingMode   ServingMode `json:"servingMode"`
	ChatRequest   ChatRequest `json:"chatRequest"`
}

// Oracle Cloud metadata response
type MetadataResponse struct {
	CertPem         string `json:"cert.pem"`
	IntermediatePem string `json:"intermediate.pem"`
	KeyPem          string `json:"key.pem"`
}
