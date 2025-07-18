// Package transform handles the conversion between OpenAI API format and Oracle Cloud GenAI format.
// It provides functionality to transform OpenAI ChatCompletion requests into the format
// expected by Oracle Cloud's Generative AI service.
package transform

import (
	"log"
	"time"

	"github.com/zalbiraw/ocigenai/internal/config"
	"github.com/zalbiraw/ocigenai/pkg/types"
)

// Transformer handles the conversion between different API formats.
type Transformer struct {
	config *config.Config
}

// New creates a new transformer with the given configuration.
func New(cfg *config.Config) *Transformer {
	return &Transformer{
		config: cfg,
	}
}

// ToOracleCloudRequest converts an OpenAI ChatCompletion request to Oracle Cloud GenAI format.
// It extracts the last message as the prompt and applies configuration defaults where needed.
//
// The transformation process:
// 1. Extracts the last message from the conversation as the main prompt
// 2. Uses OpenAI request parameters if provided, otherwise falls back to config defaults
// 3. Constructs the Oracle Cloud request structure with proper serving mode and chat parameters.
func (t *Transformer) ToOracleCloudRequest(openAIReq types.ChatCompletionRequest) types.OracleCloudRequest {
	log.Printf("[Transform] Starting transformation for model: %s", openAIReq.Model)
	start := time.Now()

	// Extract the last message as the prompt
	// In a typical conversation, the last message is what we want to respond to
	message := ""
	if len(openAIReq.Messages) > 0 {
		message = openAIReq.Messages[len(openAIReq.Messages)-1].Content
		log.Printf("[Transform] Extracted message from %d messages, length: %d chars", len(openAIReq.Messages), len(message))
	} else {
		log.Printf("[Transform] No messages found in request")
	}

	// Use OpenAI request values if provided, otherwise use config defaults
	// This allows per-request customization while maintaining sensible defaults

	maxTokens := t.config.MaxTokens
	if openAIReq.MaxTokens != 0 {
		maxTokens = openAIReq.MaxTokens
		log.Printf("[Transform] Using request maxTokens: %d", maxTokens)
	} else {
		log.Printf("[Transform] Using config maxTokens: %d", maxTokens)
	}

	temperature := t.config.Temperature
	if openAIReq.Temperature != 0 {
		temperature = float64(openAIReq.Temperature)
		log.Printf("[Transform] Using request temperature: %f", temperature)
	} else {
		log.Printf("[Transform] Using config temperature: %f", temperature)
	}

	topP := t.config.TopP
	if openAIReq.TopP != 0 {
		topP = float64(openAIReq.TopP)
		log.Printf("[Transform] Using request topP: %f", topP)
	} else {
		log.Printf("[Transform] Using config topP: %f", topP)
	}

	frequencyPenalty := t.config.FrequencyPenalty
	if openAIReq.FrequencyPenalty != 0 {
		frequencyPenalty = float64(openAIReq.FrequencyPenalty)
		log.Printf("[Transform] Using request frequencyPenalty: %f", frequencyPenalty)
	} else {
		log.Printf("[Transform] Using config frequencyPenalty: %f", frequencyPenalty)
	}

	presencePenalty := t.config.PresencePenalty
	if openAIReq.PresencePenalty != 0 {
		presencePenalty = float64(openAIReq.PresencePenalty)
		log.Printf("[Transform] Using request presencePenalty: %f", presencePenalty)
	} else {
		log.Printf("[Transform] Using config presencePenalty: %f", presencePenalty)
	}

	topK := t.config.TopK
	log.Printf("[Transform] Using config topK: %d", topK)

	// Construct the Oracle Cloud request structure
	oracleReq := types.OracleCloudRequest{
		CompartmentID: t.config.CompartmentID,
		ServingMode: types.ServingMode{
			ModelID:     openAIReq.Model,
			ServingType: "ON_DEMAND", // Standard serving type for OCI GenAI
		},
		ChatRequest: types.ChatRequest{
			MaxTokens:        maxTokens,
			Temperature:      temperature,
			FrequencyPenalty: frequencyPenalty,
			PresencePenalty:  presencePenalty,
			TopP:             topP,
			TopK:             topK,
			IsStream:         false, // Currently not supporting streaming
			StreamOptions: types.StreamOptions{
				IsIncludeUsage: false,
			},
			ChatHistory: []interface{}{}, // Empty for now, could be enhanced to include conversation history
			Message:     message,
			APIFormat:   "COHERE", // Default API format for OCI GenAI
		},
	}

	log.Printf("[Transform] Transformation completed in %v, compartment: %s", time.Since(start), t.config.CompartmentID)
	return oracleReq
}
