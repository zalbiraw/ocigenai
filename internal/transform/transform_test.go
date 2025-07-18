package transform

import (
	"math"
	"testing"

	"github.com/zalbiraw/ocigenai/internal/config"
	"github.com/zalbiraw/ocigenai/pkg/types"
)

// abs returns the absolute value of a float64
func abs(x float64) float64 {
	return math.Abs(x)
}

func TestNew(t *testing.T) {
	cfg := config.New()
	transformer := New(cfg)

	if transformer == nil {
		t.Fatal("expected transformer to be created")
	}

	if transformer.config != cfg {
		t.Error("expected transformer to use provided config")
	}
}

func TestToOracleCloudRequest_BasicTransformation(t *testing.T) {
	cfg := config.New()
	cfg.CompartmentID = "test-compartment-id"
	cfg.MaxTokens = 500
	cfg.Temperature = 0.8

	transformer := New(cfg)

	openAIReq := types.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []types.ChatCompletionMessage{
			{Role: "user", Content: "Hello, world!"},
		},
	}

	result := transformer.ToOracleCloudRequest(openAIReq)

	// Verify basic structure
	if result.CompartmentID != cfg.CompartmentID {
		t.Errorf("expected compartmentId %s, got %s", cfg.CompartmentID, result.CompartmentID)
	}

	if result.ServingMode.ModelID != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", result.ServingMode.ModelID)
	}

	if result.ServingMode.ServingType != "ON_DEMAND" {
		t.Errorf("expected serving type ON_DEMAND, got %s", result.ServingMode.ServingType)
	}

	if result.ChatRequest.Message != "Hello, world!" {
		t.Errorf("expected message 'Hello, world!', got '%s'", result.ChatRequest.Message)
	}

	if result.ChatRequest.MaxTokens != cfg.MaxTokens {
		t.Errorf("expected maxTokens %d, got %d", cfg.MaxTokens, result.ChatRequest.MaxTokens)
	}

	if result.ChatRequest.Temperature != cfg.Temperature {
		t.Errorf("expected temperature %f, got %f", cfg.Temperature, result.ChatRequest.Temperature)
	}

	if result.ChatRequest.APIFormat != "COHERE" {
		t.Errorf("expected API format COHERE, got %s", result.ChatRequest.APIFormat)
	}
}

func TestToOracleCloudRequest_MultipleMessages(t *testing.T) {
	cfg := config.New()
	cfg.CompartmentID = "test-compartment-id"

	transformer := New(cfg)

	openAIReq := types.ChatCompletionRequest{
		Model: "gpt-3.5-turbo",
		Messages: []types.ChatCompletionMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello!"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "How are you?"},
		},
	}

	result := transformer.ToOracleCloudRequest(openAIReq)

	// Should use the last message as the prompt
	expectedMessage := "How are you?"
	if result.ChatRequest.Message != expectedMessage {
		t.Errorf("expected message '%s', got '%s'", expectedMessage, result.ChatRequest.Message)
	}
}

func TestToOracleCloudRequest_EmptyMessages(t *testing.T) {
	cfg := config.New()
	cfg.CompartmentID = "test-compartment-id"

	transformer := New(cfg)

	openAIReq := types.ChatCompletionRequest{
		Model:    "gpt-4",
		Messages: []types.ChatCompletionMessage{},
	}

	result := transformer.ToOracleCloudRequest(openAIReq)

	if result.ChatRequest.Message != "" {
		t.Errorf("expected empty message, got '%s'", result.ChatRequest.Message)
	}
}

func TestToOracleCloudRequest_OpenAIOverrides(t *testing.T) {
	cfg := config.New()
	cfg.CompartmentID = "test-compartment-id"
	cfg.MaxTokens = 600
	cfg.Temperature = 1.0
	cfg.TopP = 0.75
	cfg.FrequencyPenalty = 0.0
	cfg.PresencePenalty = 0.0

	transformer := New(cfg)

	openAIReq := types.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []types.ChatCompletionMessage{
			{Role: "user", Content: "Test message"},
		},
		MaxTokens:        1000,
		Temperature:      0.5,
		TopP:             0.9,
		FrequencyPenalty: 0.2,
		PresencePenalty:  0.1,
	}

	result := transformer.ToOracleCloudRequest(openAIReq)

	// OpenAI values should override config defaults
	if result.ChatRequest.MaxTokens != 1000 {
		t.Errorf("expected maxTokens 1000, got %d", result.ChatRequest.MaxTokens)
	}

	if result.ChatRequest.Temperature != 0.5 {
		t.Errorf("expected temperature 0.5, got %f", result.ChatRequest.Temperature)
	}

	// Use approximate comparison for floating point values
	if abs(result.ChatRequest.TopP-0.9) > 0.0001 {
		t.Errorf("expected topP 0.9, got %f", result.ChatRequest.TopP)
	}

	if abs(result.ChatRequest.FrequencyPenalty-0.2) > 0.0001 {
		t.Errorf("expected frequencyPenalty 0.2, got %f", result.ChatRequest.FrequencyPenalty)
	}

	if abs(result.ChatRequest.PresencePenalty-0.1) > 0.0001 {
		t.Errorf("expected presencePenalty 0.1, got %f", result.ChatRequest.PresencePenalty)
	}
}

func TestToOracleCloudRequest_ConfigDefaults(t *testing.T) {
	cfg := config.New()
	cfg.CompartmentID = "test-compartment-id"
	cfg.MaxTokens = 800
	cfg.Temperature = 0.7
	cfg.TopP = 0.8
	cfg.FrequencyPenalty = 0.3
	cfg.PresencePenalty = 0.2
	cfg.TopK = 50

	transformer := New(cfg)

	openAIReq := types.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []types.ChatCompletionMessage{
			{Role: "user", Content: "Test message"},
		},
		// No overrides - should use config defaults
	}

	result := transformer.ToOracleCloudRequest(openAIReq)

	if result.ChatRequest.MaxTokens != cfg.MaxTokens {
		t.Errorf("expected maxTokens %d, got %d", cfg.MaxTokens, result.ChatRequest.MaxTokens)
	}

	if result.ChatRequest.Temperature != cfg.Temperature {
		t.Errorf("expected temperature %f, got %f", cfg.Temperature, result.ChatRequest.Temperature)
	}

	if result.ChatRequest.TopP != cfg.TopP {
		t.Errorf("expected topP %f, got %f", cfg.TopP, result.ChatRequest.TopP)
	}

	if result.ChatRequest.FrequencyPenalty != cfg.FrequencyPenalty {
		t.Errorf("expected frequencyPenalty %f, got %f", cfg.FrequencyPenalty, result.ChatRequest.FrequencyPenalty)
	}

	if result.ChatRequest.PresencePenalty != cfg.PresencePenalty {
		t.Errorf("expected presencePenalty %f, got %f", cfg.PresencePenalty, result.ChatRequest.PresencePenalty)
	}

	if result.ChatRequest.TopK != cfg.TopK {
		t.Errorf("expected topK %d, got %d", cfg.TopK, result.ChatRequest.TopK)
	}
}

func TestToOracleCloudRequest_StreamingDefaults(t *testing.T) {
	cfg := config.New()
	cfg.CompartmentID = "test-compartment-id"

	transformer := New(cfg)

	openAIReq := types.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []types.ChatCompletionMessage{
			{Role: "user", Content: "Test message"},
		},
	}

	result := transformer.ToOracleCloudRequest(openAIReq)

	// Verify streaming defaults
	if result.ChatRequest.IsStream != false {
		t.Error("expected IsStream to be false")
	}

	if result.ChatRequest.StreamOptions.IsIncludeUsage != false {
		t.Error("expected IsIncludeUsage to be false")
	}

	// Verify chat history is empty
	if len(result.ChatRequest.ChatHistory) != 0 {
		t.Errorf("expected empty chat history, got %d items", len(result.ChatRequest.ChatHistory))
	}
}
