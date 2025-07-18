// Package ocigenai is a Traefik plugin that proxies OpenAI API requests to Oracle Cloud Infrastructure (OCI) Generative AI service.
//
// The plugin intercepts POST requests to /chat/completions, transforms them from OpenAI format
// to OCI GenAI format, adds OCI Instance Principal authentication, and forwards them to the
// configured OCI GenAI endpoint.
//
// Key features:
// - Seamless OpenAI to OCI GenAI API translation
// - Instance Principal authentication with certificate caching
// - Configurable AI model parameters with sensible defaults
// - Thread-safe credential management
// - Comprehensive error handling and logging
package ocigenai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/zalbiraw/ocigenai/internal/auth"
	"github.com/zalbiraw/ocigenai/internal/config"
	"github.com/zalbiraw/ocigenai/internal/transform"
	"github.com/zalbiraw/ocigenai/pkg/types"
)

// Proxy represents the main plugin instance that handles request proxying.
// It contains all the necessary components for transforming and authenticating requests.
type Proxy struct {
	next          http.Handler           // Next handler in the middleware chain
	config        *config.Config         // Plugin configuration
	name          string                 // Plugin instance name
	transformer   *transform.Transformer // Request transformer
	authenticator *auth.Authenticator    // OCI authenticator
}

// New creates a new Proxy plugin instance.
// It validates the configuration and initializes all necessary components.
//
// Parameters:
//   - ctx: Context for the plugin initialization
//   - next: Next HTTP handler in the middleware chain
//   - cfg: Plugin configuration
//   - name: Name of the plugin instance
//
// Returns the configured plugin handler or an error if configuration is invalid.
func New(ctx context.Context, next http.Handler, cfg *config.Config, name string) (http.Handler, error) {
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Initialize components
	transformer := transform.New(cfg)
	authenticator := auth.New()

	return &Proxy{
		next:          next,
		config:        cfg,
		name:          name,
		transformer:   transformer,
		authenticator: authenticator,
	}, nil
}

// ServeHTTP implements the http.Handler interface and processes incoming requests.
//
// The plugin only processes POST requests to paths ending with "/chat/completions".
// All other requests are passed through to the next handler unchanged.
//
// For matching requests, the plugin:
// 1. Parses the OpenAI ChatCompletion request
// 2. Transforms it to OCI GenAI format
// 3. Adds OCI Instance Principal authentication headers
// 4. Forwards the request to the next handler.
func (p *Proxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Only process POST requests to /chat/completions
	if !p.shouldProcessRequest(req) {
		p.next.ServeHTTP(rw, req)
		return
	}

	// Process the OpenAI request
	if err := p.processOpenAIRequest(rw, req); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	// Forward to next handler
	p.next.ServeHTTP(rw, req)
}

// shouldProcessRequest determines if a request should be processed by this plugin.
func (p *Proxy) shouldProcessRequest(req *http.Request) bool {
	return req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/chat/completions")
}

// processOpenAIRequest handles the transformation and authentication of OpenAI requests.
func (p *Proxy) processOpenAIRequest(rw http.ResponseWriter, req *http.Request) error {
	// Read the request body
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	// Close the original body
	if closeErr := req.Body.Close(); closeErr != nil {
		return fmt.Errorf("failed to close request body: %w", closeErr)
	}

	// Parse OpenAI request
	var openAIReq types.ChatCompletionRequest
	if unmarshalErr := json.Unmarshal(body, &openAIReq); unmarshalErr != nil {
		http.Error(rw, "Failed to parse OpenAI request", http.StatusBadRequest)
		return unmarshalErr // Return the actual error for proper error handling
	}

	// Transform to Oracle Cloud format
	oracleReq := p.transformer.ToOracleCloudRequest(openAIReq)

	// Marshal the Oracle Cloud request
	oracleBody, err := json.Marshal(oracleReq)
	if err != nil {
		return fmt.Errorf("failed to marshal Oracle Cloud request: %w", err)
	}

	// Replace request body with transformed content
	req.Body = io.NopCloser(bytes.NewReader(oracleBody))
	req.ContentLength = int64(len(oracleBody))
	req.Header.Set("Content-Type", "application/json")

	// Add OCI authentication headers
	if err := p.authenticator.SignRequest(req); err != nil {
		return fmt.Errorf("failed to authenticate request: %w", err)
	}

	return nil
}

// CreateConfig creates the default plugin configuration.
// This function is required by Traefik's plugin system.
func CreateConfig() *config.Config {
	return config.New()
}
