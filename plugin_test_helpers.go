package ocigenai

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/zalbiraw/ocigenai/internal/config"
)

// Helper function to verify transformed request.
func verifyTransformedRequest(t *testing.T, req *http.Request, cfg *config.Config) {
	t.Helper()
	// Read and verify the transformed body
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("failed to read transformed body: %v", err)
	}

	// Parse the transformed request
	var oracleReq map[string]interface{}
	if err := json.Unmarshal(body, &oracleReq); err != nil {
		t.Fatalf("failed to parse transformed request: %v", err)
	}

	// Verify transformation
	if oracleReq["compartmentId"] != cfg.CompartmentID {
		t.Errorf("expected compartmentId %s, got %v", cfg.CompartmentID, oracleReq["compartmentId"])
	}

	servingMode, ok := oracleReq["servingMode"].(map[string]interface{})
	if !ok {
		t.Fatal("servingMode not found or invalid")
	}

	if servingMode["modelId"] != "gpt-4" {
		t.Errorf("expected model gpt-4, got %v", servingMode["modelId"])
	}

	chatRequest, ok := oracleReq["chatRequest"].(map[string]interface{})
	if !ok {
		t.Fatal("chatRequest not found or invalid")
	}

	if chatRequest["message"] != "Hello, world!" {
		t.Errorf("expected message 'Hello, world!', got %v", chatRequest["message"])
	}
}

// Helper function to verify authentication headers.
func verifyAuthHeaders(t *testing.T, req *http.Request) {
	t.Helper()
	// Verify authentication headers are present
	authHeader := req.Header.Get("Authorization")
	if authHeader == "" {
		t.Error("expected Authorization header to be set")
	}

	dateHeader := req.Header.Get("Date")
	if dateHeader == "" {
		t.Error("expected Date header to be set")
	}
}
