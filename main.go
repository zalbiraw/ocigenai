// Package ocigenai is a Traefik plugin to proxy requests to OCI Generative AI using Instance Principals.
package ocigenai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// Proxy a plugin to proxy to OCI GenAI.
type Proxy struct {
	next   http.Handler
	config *Config
	name   string
}

// New creates a new Proxy plugin.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if config.CompartmentID == "" {
		return nil, fmt.Errorf("compartmentId cannot be empty")
	}

	return &Proxy{
		next:   next,
		config: config,
		name:   name,
	}, nil
}

func (p *Proxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Only process POST requests to /chat/completions
	if req.Method != http.MethodPost || !strings.HasSuffix(req.URL.Path, "/chat/completions") {
		p.next.ServeHTTP(rw, req)
		return
	}

	// Read the request body
	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(rw, "Failed to read request body", http.StatusBadRequest)
		return
	}
	if closeErr := req.Body.Close(); closeErr != nil {
		http.Error(rw, "Failed to close request body", http.StatusInternalServerError)
		return
	}

	// Parse OpenAI request
	var openAIReq openai.ChatCompletionRequest
	if unmarshalErr := json.Unmarshal(body, &openAIReq); unmarshalErr != nil {
		http.Error(rw, "Failed to parse OpenAI request", http.StatusBadRequest)
		return
	}

	// Transform to Oracle Cloud format
	oracleReq := p.transformRequest(openAIReq)

	// Marshal the Oracle Cloud request
	oracleBody, err := json.Marshal(oracleReq)
	if err != nil {
		http.Error(rw, "Failed to marshal Oracle Cloud request", http.StatusInternalServerError)
		return
	}

	// Create new request with Oracle Cloud format
	req.Body = io.NopCloser(bytes.NewReader(oracleBody))
	req.ContentLength = int64(len(oracleBody))
	req.Header.Set("Content-Type", "application/json")

	// Add OCI signature authentication headers
	if err := p.getAuthHeaders(req); err != nil {
		http.Error(rw, "Failed to authenticate request", http.StatusInternalServerError)
		return
	}

	// Forward to next handler
	p.next.ServeHTTP(rw, req)
}
