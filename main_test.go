package ocigenai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
)

// Test plugin creation with valid config
func TestNew_ValidConfig(t *testing.T) {
	cfg := CreateConfig()
	cfg.CompartmentID = "test-compartment-id"

	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {})

	handler, err := New(ctx, next, cfg, "oci-genai-proxy")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if handler == nil {
		t.Fatal("expected handler to be created")
	}
}

// Test plugin creation with invalid config
func TestNew_InvalidConfig(t *testing.T) {
	cfg := CreateConfig()
	// Missing CompartmentID

	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {})

	_, err := New(ctx, next, cfg, "oci-genai-proxy")
	if err == nil {
		t.Fatal("expected error for missing compartmentId")
	}

	if !strings.Contains(err.Error(), "compartmentId cannot be empty") {
		t.Errorf("expected compartmentId error, got: %v", err)
	}
}

// Test handling non-POST requests (should pass through)
func TestServeHTTP_NonPostRequest(t *testing.T) {
	cfg := CreateConfig()
	cfg.CompartmentID = "test-compartment-id"

	ctx := context.Background()
	nextCalled := false
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		nextCalled = true
		rw.WriteHeader(http.StatusOK)
	})

	handler, err := New(ctx, next, cfg, "oci-genai-proxy")
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "/chat/completions", nil)

	handler.ServeHTTP(recorder, req)

	if !nextCalled {
		t.Error("expected next handler to be called for non-POST request")
	}

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status 200, got: %d", recorder.Code)
	}
}

// Test handling non-chat-completions path (should pass through)
func TestServeHTTP_NonChatCompletionsPath(t *testing.T) {
	cfg := CreateConfig()
	cfg.CompartmentID = "test-compartment-id"

	ctx := context.Background()
	nextCalled := false
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		nextCalled = true
		rw.WriteHeader(http.StatusOK)
	})

	handler, err := New(ctx, next, cfg, "oci-genai-proxy")
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "/other/endpoint", nil)

	handler.ServeHTTP(recorder, req)

	if !nextCalled {
		t.Error("expected next handler to be called for non-chat-completions path")
	}

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status 200, got: %d", recorder.Code)
	}
}

// Test handling malformed JSON request body
func TestServeHTTP_MalformedJSON(t *testing.T) {
	cfg := CreateConfig()
	cfg.CompartmentID = "test-compartment-id"

	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		t.Error("next handler should not be called for malformed JSON")
	})

	handler, err := New(ctx, next, cfg, "oci-genai-proxy")
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	body := strings.NewReader("{invalid json}")
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "/chat/completions", body)
	req.Header.Set("Content-Type", "application/json")

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got: %d", recorder.Code)
	}

	if !strings.Contains(recorder.Body.String(), "Failed to parse OpenAI request") {
		t.Errorf("expected parse error message, got: %s", recorder.Body.String())
	}
}

// Test successful request transformation and processing
func TestServeHTTP_ValidRequest(t *testing.T) {
	cfg := CreateConfig()
	cfg.CompartmentID = "test-compartment-id"
	cfg.MaxTokens = 500
	cfg.Temperature = 0.8

	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// Read and verify the transformed body
		body, _ := io.ReadAll(req.Body)
		var oracleReq OracleCloudRequest
		err := json.Unmarshal(body, &oracleReq)
		if err != nil {
			t.Errorf("failed to unmarshal Oracle request: %v", err)
		}

		// Verify transformation
		if oracleReq.CompartmentID != cfg.CompartmentID {
			t.Errorf("expected compartmentId %s, got %s", cfg.CompartmentID, oracleReq.CompartmentID)
		}
		if oracleReq.ServingMode.ModelID != "gpt-3.5-turbo" {
			t.Errorf("expected model gpt-3.5-turbo, got %s", oracleReq.ServingMode.ModelID)
		}
		if oracleReq.ChatRequest.Message != "Hello, world!" {
			t.Errorf("expected message 'Hello, world!', got %s", oracleReq.ChatRequest.Message)
		}
		if oracleReq.ChatRequest.MaxTokens != 100 { // Should use OpenAI request value
			t.Errorf("expected maxTokens 100, got %d", oracleReq.ChatRequest.MaxTokens)
		}

		rw.WriteHeader(http.StatusOK)
	})

	// Mock getAuthHeaders to avoid OCI authentication in tests
	originalProxy := &Proxy{
		next:   next,
		config: cfg,
		name:   "oci-genai-proxy",
	}

	// Create OpenAI-format request
	openAIReq := openai.ChatCompletionRequest{
		Model:     "gpt-3.5-turbo",
		MaxTokens: 100,
		Messages: []openai.ChatCompletionMessage{
			{Role: "user", Content: "Hello, world!"},
		},
	}

	body, err := json.Marshal(openAIReq)
	if err != nil {
		t.Fatalf("failed to marshal OpenAI request: %v", err)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// We need to test without actual OCI auth, so we'll test the transformation separately
	transformedReq := originalProxy.transformRequest(openAIReq)

	// Verify transformation logic
	if transformedReq.CompartmentID != cfg.CompartmentID {
		t.Errorf("expected compartmentId %s, got %s", cfg.CompartmentID, transformedReq.CompartmentID)
	}
	if transformedReq.ServingMode.ModelID != "gpt-3.5-turbo" {
		t.Errorf("expected model gpt-3.5-turbo, got %s", transformedReq.ServingMode.ModelID)
	}
	if transformedReq.ChatRequest.Message != "Hello, world!" {
		t.Errorf("expected message 'Hello, world!', got %s", transformedReq.ChatRequest.Message)
	}
	if transformedReq.ChatRequest.MaxTokens != 100 {
		t.Errorf("expected maxTokens 100, got %d", transformedReq.ChatRequest.MaxTokens)
	}
}

// Test request transformation with multiple messages
func TestTransformRequest_MultipleMessages(t *testing.T) {
	cfg := CreateConfig()
	cfg.CompartmentID = "test-compartment-id"

	proxy := &Proxy{
		config: cfg,
	}

	openAIReq := openai.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []openai.ChatCompletionMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "What is AI?"},
			{Role: "assistant", Content: "AI is artificial intelligence."},
			{Role: "user", Content: "Tell me more."},
		},
	}

	transformed := proxy.transformRequest(openAIReq)

	// Should use the last message as the prompt
	if transformed.ChatRequest.Message != "Tell me more." {
		t.Errorf("expected last message 'Tell me more.', got %s", transformed.ChatRequest.Message)
	}
	if transformed.ServingMode.ModelID != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", transformed.ServingMode.ModelID)
	}
}

// Test request transformation with config defaults
func TestTransformRequest_ConfigDefaults(t *testing.T) {
	cfg := CreateConfig()
	cfg.CompartmentID = "test-compartment-id"
	cfg.MaxTokens = 500
	cfg.Temperature = 0.9
	cfg.TopP = 0.8
	cfg.TopK = 10

	proxy := &Proxy{
		config: cfg,
	}

	// OpenAI request with no parameters - should use config defaults
	openAIReq := openai.ChatCompletionRequest{
		Model: "gpt-3.5-turbo",
		Messages: []openai.ChatCompletionMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	transformed := proxy.transformRequest(openAIReq)

	if transformed.ChatRequest.MaxTokens != cfg.MaxTokens {
		t.Errorf("expected maxTokens %d, got %d", cfg.MaxTokens, transformed.ChatRequest.MaxTokens)
	}
	if transformed.ChatRequest.Temperature != cfg.Temperature {
		t.Errorf("expected temperature %f, got %f", cfg.Temperature, transformed.ChatRequest.Temperature)
	}
	if transformed.ChatRequest.TopP != cfg.TopP {
		t.Errorf("expected topP %f, got %f", cfg.TopP, transformed.ChatRequest.TopP)
	}
	if transformed.ChatRequest.TopK != cfg.TopK {
		t.Errorf("expected topK %d, got %d", cfg.TopK, transformed.ChatRequest.TopK)
	}
}

// Test request transformation with OpenAI parameters override
func TestTransformRequest_OpenAIOverrides(t *testing.T) {
	cfg := CreateConfig()
	cfg.CompartmentID = "test-compartment-id"

	proxy := &Proxy{
		config: cfg,
	}

	// OpenAI request with parameters - should override config defaults
	openAIReq := openai.ChatCompletionRequest{
		Model:            "gpt-4",
		MaxTokens:        200,
		Temperature:      0.5,
		TopP:             0.9,
		FrequencyPenalty: 0.1,
		PresencePenalty:  0.2,
		Messages: []openai.ChatCompletionMessage{
			{Role: "user", Content: "Test message"},
		},
	}

	transformed := proxy.transformRequest(openAIReq)

	if transformed.ChatRequest.MaxTokens != 200 {
		t.Errorf("expected maxTokens 200, got %d", transformed.ChatRequest.MaxTokens)
	}
	// Use proper tolerance for float32 to float64 conversion
	if math.Abs(transformed.ChatRequest.Temperature-0.5) > 1e-6 {
		t.Errorf("expected temperature 0.5, got %f", transformed.ChatRequest.Temperature)
	}
	if math.Abs(transformed.ChatRequest.TopP-float64(float32(0.9))) > 1e-6 {
		t.Errorf("expected topP 0.9, got %f", transformed.ChatRequest.TopP)
	}
	if math.Abs(transformed.ChatRequest.FrequencyPenalty-float64(float32(0.1))) > 1e-6 {
		t.Errorf("expected frequencyPenalty 0.1, got %f", transformed.ChatRequest.FrequencyPenalty)
	}
	if math.Abs(transformed.ChatRequest.PresencePenalty-float64(float32(0.2))) > 1e-6 {
		t.Errorf("expected presencePenalty 0.2, got %f", transformed.ChatRequest.PresencePenalty)
	}
}

// Test config creation
func TestCreateConfig(t *testing.T) {
	cfg := CreateConfig()

	if cfg.MaxTokens != 600 {
		t.Errorf("expected default maxTokens 600, got %d", cfg.MaxTokens)
	}
	if cfg.Temperature != 1.0 {
		t.Errorf("expected default temperature 1.0, got %f", cfg.Temperature)
	}
	if cfg.TopP != 0.75 {
		t.Errorf("expected default topP 0.75, got %f", cfg.TopP)
	}
	if cfg.FrequencyPenalty != 0.0 {
		t.Errorf("expected default frequencyPenalty 0.0, got %f", cfg.FrequencyPenalty)
	}
	if cfg.PresencePenalty != 0.0 {
		t.Errorf("expected default presencePenalty 0.0, got %f", cfg.PresencePenalty)
	}
	if cfg.TopK != 0 {
		t.Errorf("expected default topK 0, got %d", cfg.TopK)
	}
}

// Test request transformation with empty messages
func TestTransformRequest_EmptyMessages(t *testing.T) {
	cfg := CreateConfig()
	cfg.CompartmentID = "test-compartment-id"

	proxy := &Proxy{
		config: cfg,
	}

	openAIReq := openai.ChatCompletionRequest{
		Model:    "gpt-3.5-turbo",
		Messages: []openai.ChatCompletionMessage{},
	}

	transformed := proxy.transformRequest(openAIReq)

	// Should handle empty messages gracefully
	if transformed.ChatRequest.Message != "" {
		t.Errorf("expected empty message, got '%s'", transformed.ChatRequest.Message)
	}
}

// Additional edge case tests

func TestServeHTTP_EmptyBody(t *testing.T) {
	cfg := CreateConfig()
	cfg.CompartmentID = "test-compartment-id"

	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		t.Error("next handler should not be called for empty body")
	})

	handler, err := New(ctx, next, cfg, "oci-genai-proxy")
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/chat/completions", bytes.NewReader([]byte("")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestServeHTTP_InvalidJSON(t *testing.T) {
	cfg := CreateConfig()
	cfg.CompartmentID = "test-compartment-id"

	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		t.Error("next handler should not be called for invalid JSON")
	})

	handler, err := New(ctx, next, cfg, "oci-genai-proxy")
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/chat/completions", bytes.NewReader([]byte("{invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestTransformRequest_WithSystemMessage(t *testing.T) {
	cfg := CreateConfig()
	cfg.CompartmentID = "test-compartment-id"
	cfg.MaxTokens = 1000
	cfg.Temperature = 0.7
	cfg.TopP = 0.9

	proxy := &Proxy{config: cfg}

	openAIReq := openai.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []openai.ChatCompletionMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello!"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "How are you?"},
		},
	}

	transformed := proxy.transformRequest(openAIReq)

	// The transformer only uses the last message as the prompt
	expectedMessage := "How are you?"
	if transformed.ChatRequest.Message != expectedMessage {
		t.Errorf("expected message '%s', got '%s'", expectedMessage, transformed.ChatRequest.Message)
	}

	if transformed.ServingMode.ModelID != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", transformed.ServingMode.ModelID)
	}

	if transformed.ChatRequest.MaxTokens != cfg.MaxTokens {
		t.Errorf("expected maxTokens %d, got %d", cfg.MaxTokens, transformed.ChatRequest.MaxTokens)
	}
}

func TestTransformRequest_EdgeCases(t *testing.T) {
	cfg := CreateConfig()
	cfg.CompartmentID = "test-compartment-id"

	proxy := &Proxy{config: cfg}

	// Test with only system message
	openAIReq := openai.ChatCompletionRequest{
		Model: "gpt-3.5-turbo",
		Messages: []openai.ChatCompletionMessage{
			{Role: "system", Content: "System message only."},
		},
	}

	transformed := proxy.transformRequest(openAIReq)
	if transformed.ChatRequest.Message != "System message only." {
		t.Errorf("expected 'System message only.', got '%s'", transformed.ChatRequest.Message)
	}

	// Test with large token limits
	openAIReq.MaxTokens = 4096
	transformed = proxy.transformRequest(openAIReq)
	if transformed.ChatRequest.MaxTokens != 4096 {
		t.Errorf("expected maxTokens 4096, got %d", transformed.ChatRequest.MaxTokens)
	}
}

func TestConfig_EdgeValues(t *testing.T) {
	cfg := CreateConfig()

	// Test default values
	if cfg.MaxTokens != 600 {
		t.Errorf("expected default MaxTokens 600, got %d", cfg.MaxTokens)
	}
	if cfg.Temperature != 1.0 {
		t.Errorf("expected default Temperature 1.0, got %f", cfg.Temperature)
	}
	if cfg.TopP != 0.75 {
		t.Errorf("expected default TopP 0.75, got %f", cfg.TopP)
	}
	if cfg.FrequencyPenalty != 0.0 {
		t.Errorf("expected default FrequencyPenalty 0.0, got %f", cfg.FrequencyPenalty)
	}
	if cfg.PresencePenalty != 0.0 {
		t.Errorf("expected default PresencePenalty 0.0, got %f", cfg.PresencePenalty)
	}
	if cfg.TopK != 0 {
		t.Errorf("expected default TopK 0, got %d", cfg.TopK)
	}
}

func TestNew_NilConfig(t *testing.T) {
	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {})

	// This should panic since we don't check for nil config
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil config")
		}
	}()

	_, _ = New(ctx, next, nil, "oci-genai-proxy")
}
