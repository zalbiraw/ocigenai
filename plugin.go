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
	"log"
	"net/http"
	"strings"
	"time"

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
	log.Printf("[%s] Initializing OCI GenAI proxy plugin", name)

	// Validate configuration
	log.Printf("[%s] Validating configuration", name)
	if err := cfg.Validate(); err != nil {
		log.Printf("[%s] Configuration validation failed: %v", name, err)
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Initialize components
	log.Printf("[%s] Initializing transformer", name)
	transformer := transform.New(cfg)
	log.Printf("[%s] Initializing authenticator", name)
	authenticator := auth.New()

	log.Printf("[%s] Plugin initialization completed successfully", name)
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
	log.Printf("[%s] Request received: %s %s", p.name, req.Method, req.URL.Path)

	// Only process POST requests to /chat/completions
	if !p.shouldProcessRequest(req) {
		log.Printf("[%s] Request filtered out - not processing", p.name)
		p.next.ServeHTTP(rw, req)
		return
	}

	log.Printf("[%s] Processing OpenAI request", p.name)
	start := time.Now()

	// Process the OpenAI request
	if err := p.processOpenAIRequest(rw, req); err != nil {
		log.Printf("[%s] Request processing failed: %v", p.name, err)
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("[%s] Request processing completed in %v", p.name, time.Since(start))

	// Forward to next handler
	p.next.ServeHTTP(rw, req)
}

// shouldProcessRequest determines if a request should be processed by this plugin.
func (p *Proxy) shouldProcessRequest(req *http.Request) bool {
	shouldProcess := req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/chat/completions")
	log.Printf("[%s] Request filtering: method=%s, path=%s, shouldProcess=%v", p.name, req.Method, req.URL.Path, shouldProcess)
	return shouldProcess
}

// processOpenAIRequest handles the transformation and authentication of OpenAI requests.
func (p *Proxy) processOpenAIRequest(rw http.ResponseWriter, req *http.Request) error {
	// Read the request body
	log.Printf("[%s] Reading request body", p.name)
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}
	log.Printf("[%s] Request body size: %d bytes", p.name, len(body))

	// Close the original body
	if closeErr := req.Body.Close(); closeErr != nil {
		return fmt.Errorf("failed to close request body: %w", closeErr)
	}

	// Parse OpenAI request
	log.Printf("[%s] Parsing OpenAI request", p.name)
	var openAIReq types.ChatCompletionRequest
	if unmarshalErr := json.Unmarshal(body, &openAIReq); unmarshalErr != nil {
		log.Printf("[%s] Failed to parse OpenAI request: %v", p.name, unmarshalErr)
		http.Error(rw, "Failed to parse OpenAI request", http.StatusBadRequest)
		return unmarshalErr // Return the actual error for proper error handling
	}
	log.Printf("[%s] OpenAI request parsed successfully: model=%s, messages=%d", p.name, openAIReq.Model, len(openAIReq.Messages))

	// Transform to Oracle Cloud format
	log.Printf("[%s] Transforming to Oracle Cloud format", p.name)
	oracleReq := p.transformer.ToOracleCloudRequest(openAIReq)

	// Marshal the Oracle Cloud request
	log.Printf("[%s] Marshaling Oracle Cloud request", p.name)
	oracleBody, err := json.Marshal(oracleReq)
	if err != nil {
		return fmt.Errorf("failed to marshal Oracle Cloud request: %w", err)
	}
	log.Printf("[%s] Oracle Cloud request size: %d bytes", p.name, len(oracleBody))

	// Replace request body with transformed content
	req.Body = io.NopCloser(bytes.NewReader(oracleBody))
	req.ContentLength = int64(len(oracleBody))
	req.Header.Set("Content-Type", "application/json")
	log.Printf("[%s] Request body replaced with transformed content", p.name)

	// Add OCI authentication headers
	log.Printf("[%s] Adding OCI authentication headers", p.name)
	if err := p.authenticator.SignRequest(req); err != nil {
		log.Printf("[%s] Authentication failed: %v", p.name, err)
		return fmt.Errorf("failed to authenticate request: %w", err)
	}
	log.Printf("[%s] Authentication successful", p.name)

	return nil
}

// CreateConfig creates the default plugin configuration.
// This function is required by Traefik's plugin system.
func CreateConfig() *config.Config {
	return config.New()
}
