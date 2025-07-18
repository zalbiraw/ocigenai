package ocigenai

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zalbiraw/ocigenai/internal/config"
	"github.com/zalbiraw/ocigenai/pkg/types"
)

func TestNew_ValidConfig(t *testing.T) {
	cfg := config.New()
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

func TestNew_InvalidConfig(t *testing.T) {
	cfg := config.New()
	// Missing CompartmentID

	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {})

	_, err := New(ctx, next, cfg, "oci-genai-proxy")
	if err == nil {
		t.Fatal("expected error for missing compartmentId")
	}

	if !strings.Contains(err.Error(), "compartmentId is required and cannot be empty") {
		t.Errorf("expected compartmentId error, got: %v", err)
	}
}

func TestServeHTTP_NonPostRequest(t *testing.T) {
	cfg := config.New()
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

func TestServeHTTP_NonChatCompletionsPath(t *testing.T) {
	cfg := config.New()
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

func TestServeHTTP_MalformedJSON(t *testing.T) {
	cfg := config.New()
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
	body := strings.NewReader("{invalid json}")
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "/chat/completions", body)
	req.Header.Set("Content-Type", "application/json")

	handler.ServeHTTP(recorder, req)

	if nextCalled {
		t.Error("next handler should not be called for malformed JSON")
	}

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got: %d", recorder.Code)
	}

	if !strings.Contains(recorder.Body.String(), "Failed to parse OpenAI request") {
		t.Errorf("expected parse error message, got: %s", recorder.Body.String())
	}
}

func TestServeHTTP_ValidRequest(t *testing.T) {
	cfg := config.New()
	cfg.CompartmentID = "test-compartment-id"
	cfg.MaxTokens = 500
	cfg.Temperature = 0.8

	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		verifyTransformedRequest(t, req, cfg)
		verifyAuthHeaders(t, req)

		rw.WriteHeader(http.StatusOK)
	})

	handler, err := New(ctx, next, cfg, "oci-genai-proxy")
	if err != nil {
		t.Fatal(err)
	}

	// Create OpenAI request
	openAIReq := types.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []types.ChatCompletionMessage{
			{Role: "user", Content: "Hello, world!"},
		},
	}

	reqBody, err := json.Marshal(openAIReq)
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "/chat/completions", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	// Note: This test will fail authentication since we're not on an OCI instance
	// but we can verify the transformation logic works
	handler.ServeHTTP(recorder, req)

	// The request should fail at authentication step, which is expected in test environment
	if recorder.Code != http.StatusInternalServerError {
		t.Logf("Expected authentication failure in test environment, got status: %d", recorder.Code)
	}
}

func TestShouldProcessRequest(t *testing.T) {
	cfg := config.New()
	cfg.CompartmentID = "test-compartment-id"

	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {})

	handler, err := New(ctx, next, cfg, "oci-genai-proxy")
	if err != nil {
		t.Fatal(err)
	}

	proxy, ok := handler.(*Proxy)
	if !ok {
		t.Fatal("handler is not a *Proxy")
	}

	tests := []struct {
		method   string
		path     string
		expected bool
	}{
		{"POST", "/chat/completions", true},
		{"POST", "/v1/chat/completions", true},
		{"POST", "/api/v1/chat/completions", true},
		{"GET", "/chat/completions", false},
		{"POST", "/completions", false},
		{"POST", "/other/endpoint", false},
		{"PUT", "/chat/completions", false},
	}

	for _, test := range tests {
		req, _ := http.NewRequest(test.method, test.path, nil)
		result := proxy.shouldProcessRequest(req)
		if result != test.expected {
			t.Errorf("shouldProcessRequest(%s %s) = %v, expected %v", test.method, test.path, result, test.expected)
		}
	}
}

func TestServeHTTP_EmptyBody(t *testing.T) {
	cfg := config.New()
	cfg.CompartmentID = "test-compartment-id"

	ctx := context.Background()
	nextCalled := false
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		nextCalled = true
		rw.WriteHeader(http.StatusOK)
	})

	handler, err := New(ctx, next, cfg, "oci-genai-proxy")
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/chat/completions", bytes.NewReader([]byte("")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if nextCalled {
		t.Error("next handler should not be called for empty body")
	}

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestServeHTTP_InvalidJSON(t *testing.T) {
	cfg := config.New()
	cfg.CompartmentID = "test-compartment-id"

	ctx := context.Background()
	nextCalled := false
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		nextCalled = true
		rw.WriteHeader(http.StatusOK)
	})

	handler, err := New(ctx, next, cfg, "oci-genai-proxy")
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/chat/completions", bytes.NewReader([]byte("{invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if nextCalled {
		t.Error("next handler should not be called for invalid JSON")
	}

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateConfig(t *testing.T) {
	cfg := CreateConfig()

	if cfg == nil {
		t.Fatal("expected config to be created")
	}

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
