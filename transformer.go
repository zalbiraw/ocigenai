// Package ocigenai is a Traefik plugin to proxy requests to OCI Generative AI using Instance Principals.
package ocigenai

import "github.com/sashabaranov/go-openai"

func (p *OCIGenAIProxy) transformRequest(openAIReq openai.ChatCompletionRequest) OracleCloudRequest {
	// Extract the last message as the prompt
	message := ""
	if len(openAIReq.Messages) > 0 {
		message = openAIReq.Messages[len(openAIReq.Messages)-1].Content
	}

	// Use OpenAI request values if provided, otherwise use config defaults
	maxTokens := p.config.MaxTokens
	if openAIReq.MaxTokens != 0 {
		maxTokens = openAIReq.MaxTokens
	}

	temperature := p.config.Temperature
	if openAIReq.Temperature != 0 {
		temperature = float64(openAIReq.Temperature)
	}

	topP := p.config.TopP
	if openAIReq.TopP != 0 {
		topP = float64(openAIReq.TopP)
	}

	frequencyPenalty := p.config.FrequencyPenalty
	if openAIReq.FrequencyPenalty != 0 {
		frequencyPenalty = float64(openAIReq.FrequencyPenalty)
	}

	presencePenalty := p.config.PresencePenalty
	if openAIReq.PresencePenalty != 0 {
		presencePenalty = float64(openAIReq.PresencePenalty)
	}

	topK := p.config.TopK

	return OracleCloudRequest{
		CompartmentID: p.config.CompartmentId,
		ServingMode: ServingMode{
			ModelID:     openAIReq.Model,
			ServingType: "ON_DEMAND",
		},
		ChatRequest: ChatRequest{
			MaxTokens:        maxTokens,
			Temperature:      temperature,
			FrequencyPenalty: frequencyPenalty,
			PresencePenalty:  presencePenalty,
			TopP:             topP,
			TopK:             topK,
			IsStream:         false,
			StreamOptions: StreamOptions{
				IsIncludeUsage: false,
			},
			ChatHistory: []interface{}{},
			Message:     message,
			APIFormat:   "COHERE",
		},
	}
}
