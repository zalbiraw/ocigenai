// Package ocisdk provides Oracle Cloud Infrastructure (OCI) Instance Principal authentication
// for the OCI GenAI proxy plugin. It implements custom OCI request signing without
// requiring the official OCI SDK, using only standard Go libraries.
package ocisdk

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zalbiraw/ocigenai/pkg/types"
)

const (
	// OCI Instance Metadata Service endpoints
	// These are the standard endpoints provided by OCI for Instance Principal authentication.
	metadataBaseURL = "http://169.254.169.254/opc/v2/"
	identityURL     = metadataBaseURL + "identity/"
	certificateURL  = identityURL + "cert.pem"
	intermediateURL = identityURL + "intermediate.pem"
	privateKeyURL   = identityURL + "key.pem"

	// Cache settings.
	defaultCacheBuffer = 1 * time.Hour    // Refresh credentials 1 hour before expiration
	minCacheBuffer     = 30 * time.Minute // Minimum cache time for soon-to-expire certificates
)

// CachedCredentials holds cached instance principal credentials with thread-safe access.
// This prevents unnecessary calls to the metadata service and improves performance.
type CachedCredentials struct {
	metadata   *types.InstanceMetadata
	privateKey *rsa.PrivateKey
	keyID      string
	expiresAt  time.Time
	mu         sync.RWMutex
}

// Authenticator handles OCI Instance Principal authentication and request signing.
type Authenticator struct {
	cache  *CachedCredentials
	client *http.Client
	signer HTTPRequestSigner
}

// instancePrincipalKeyProvider implements the KeyProvider interface
// using cached instance principal credentials.
type instancePrincipalKeyProvider struct {
	auth *Authenticator
}

// PrivateRSAKey returns the cached private key.
func (kp *instancePrincipalKeyProvider) PrivateRSAKey() (*rsa.PrivateKey, error) {
	privateKey, _, err := kp.auth.getCredentials()
	return privateKey, err
}

// KeyID returns the cached key ID.
func (kp *instancePrincipalKeyProvider) KeyID() (string, error) {
	_, keyID, err := kp.auth.getCredentials()
	return keyID, err
}

// New creates a new authenticator with default settings.
func New() *Authenticator {
	auth := &Authenticator{
		cache: &CachedCredentials{},
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	// Create a key provider that uses this authenticator's credentials
	keyProvider := &instancePrincipalKeyProvider{auth: auth}

	// Create the OCI request signer using the key provider
	auth.signer = DefaultRequestSigner(keyProvider)

	return auth
}

// SignRequest adds OCI authentication headers to the given HTTP request.
// It uses cached credentials when available or fetches fresh ones if needed.
func (a *Authenticator) SignRequest(req *http.Request) error {
	log.Printf("[Auth] Starting request signing for %s %s", req.Method, req.URL.Path)

	// Use the OCI request signer to sign the request
	if err := a.signer.Sign(req); err != nil {
		log.Printf("[Auth] Failed to sign request: %v", err)
		return fmt.Errorf("failed to sign request: %w", err)
	}

	log.Printf("[Auth] Request signed successfully")
	return nil
}

// getCredentials returns cached credentials or fetches new ones if expired.
// This method is thread-safe and prevents multiple concurrent fetches.
func (a *Authenticator) getCredentials() (*rsa.PrivateKey, string, error) {
	// Check if we have valid cached credentials (read lock)
	a.cache.mu.RLock()
	if a.cache.metadata != nil && time.Now().Before(a.cache.expiresAt) {
		privateKey := a.cache.privateKey
		keyID := a.cache.keyID
		a.cache.mu.RUnlock()
		log.Printf("[Auth] Using cached credentials, expires at: %v", a.cache.expiresAt)
		return privateKey, keyID, nil
	}
	a.cache.mu.RUnlock()

	log.Printf("[Auth] Credentials cache miss or expired, fetching fresh credentials")

	// Need to refresh credentials (write lock)
	a.cache.mu.Lock()
	defer a.cache.mu.Unlock()

	// Double-check in case another goroutine already refreshed
	if a.cache.metadata != nil && time.Now().Before(a.cache.expiresAt) {
		log.Printf("[Auth] Another goroutine refreshed credentials, using cached")
		return a.cache.privateKey, a.cache.keyID, nil
	}

	// Fetch fresh metadata from OCI Instance Metadata Service
	log.Printf("[Auth] Fetching fresh instance metadata")
	metadata, err := a.fetchInstanceMetadata()
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch instance metadata: %w", err)
	}

	// Parse the private key from PEM format
	log.Printf("[Auth] Parsing private key")
	privateKey, err := parsePrivateKey(metadata.KeyPem)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse private key: %w", err)
	}

	// Extract key ID and certificate expiration time
	log.Printf("[Auth] Extracting key ID and expiration")
	keyID, expiresAt, err := extractKeyIDAndExpiration(metadata.CertPem)
	if err != nil {
		return nil, "", fmt.Errorf("failed to extract key ID and expiration: %w", err)
	}

	// Calculate cache expiration with safety margin
	cacheExpiresAt := expiresAt.Add(-defaultCacheBuffer)
	if cacheExpiresAt.Before(time.Now()) {
		// If certificate expires soon, cache for minimum time
		cacheExpiresAt = time.Now().Add(minCacheBuffer)
		log.Printf("[Auth] Certificate expires soon, using minimum cache buffer")
	}

	log.Printf("[Auth] Certificate expires at: %v, cache expires at: %v", expiresAt, cacheExpiresAt)

	// Update cache
	a.cache.metadata = metadata
	a.cache.privateKey = privateKey
	a.cache.keyID = keyID
	a.cache.expiresAt = cacheExpiresAt

	log.Printf("[Auth] Credentials cached successfully")
	return privateKey, keyID, nil
}

// fetchInstanceMetadata retrieves certificates and private key from OCI Instance Metadata Service.
func (a *Authenticator) fetchInstanceMetadata() (*types.InstanceMetadata, error) {
	log.Printf("[Auth] Fetching instance metadata from OCI metadata service")
	start := time.Now()

	// Fetch certificate
	log.Printf("[Auth] Fetching certificate from %s", certificateURL)
	certPem, err := a.fetchMetadataEndpoint(certificateURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch certificate: %w", err)
	}
	log.Printf("[Auth] Certificate fetched successfully, size: %d bytes", len(certPem))

	// Fetch intermediate certificate
	log.Printf("[Auth] Fetching intermediate certificate from %s", intermediateURL)
	intermediatePem, err := a.fetchMetadataEndpoint(intermediateURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch intermediate certificate: %w", err)
	}
	log.Printf("[Auth] Intermediate certificate fetched successfully, size: %d bytes", len(intermediatePem))

	// Fetch private key
	log.Printf("[Auth] Fetching private key from %s", privateKeyURL)
	keyPem, err := a.fetchMetadataEndpoint(privateKeyURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch private key: %w", err)
	}
	log.Printf("[Auth] Private key fetched successfully, size: %d bytes", len(keyPem))

	log.Printf("[Auth] All metadata fetched successfully in %v", time.Since(start))
	return &types.InstanceMetadata{
		CertPem:         string(certPem),
		IntermediatePem: string(intermediatePem),
		KeyPem:          string(keyPem),
	}, nil
}

// fetchMetadataEndpoint makes an authenticated request to an OCI metadata endpoint.
func (a *Authenticator) fetchMetadataEndpoint(url string) ([]byte, error) {
	log.Printf("[Auth] Making request to metadata endpoint: %s", url)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	// OCI metadata service requires this specific authorization header
	req.Header.Set("Authorization", "Bearer Oracle")

	start := time.Now()
	resp, err := a.client.Do(req)
	if err != nil {
		log.Printf("[Auth] Request to %s failed: %v", url, err)
		return nil, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("[Auth] Failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[Auth] Metadata service returned status %d for %s", resp.StatusCode, url)
		return nil, fmt.Errorf("metadata service returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[Auth] Failed to read response body from %s: %v", url, err)
		return nil, err
	}

	log.Printf("[Auth] Successfully fetched %d bytes from %s in %v", len(body), url, time.Since(start))
	return body, nil
}

// parsePrivateKey parses an RSA private key from PEM format.
// It supports both PKCS#1 and PKCS#8 formats.
func parsePrivateKey(keyPem string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(keyPem))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// Try PKCS#1 format first
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	// Try PKCS#8 format
	parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	rsaKey, ok := parsedKey.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not RSA")
	}

	return rsaKey, nil
}

// extractKeyIDAndExpiration extracts the key ID and expiration time from a certificate.
func extractKeyIDAndExpiration(certPem string) (string, time.Time, error) {
	block, _ := pem.Decode([]byte(certPem))
	if block == nil {
		return "", time.Time{}, fmt.Errorf("failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Extract key ID from certificate subject's uniqueIdentifier field
	var keyID string
	for _, name := range cert.Subject.Names {
		if name.Type.String() == "2.5.4.45" { // OID for uniqueIdentifier
			if keyIDValue, ok := name.Value.(string); ok {
				keyID = keyIDValue
			}
			break
		}
	}

	// Fallback: use certificate serial number as key ID
	if keyID == "" {
		keyID = cert.SerialNumber.String()
	}

	return keyID, cert.NotAfter, nil
}

// buildSigningString constructs the signing string according to OCI specification.
// The signing string includes: (request-target), host, and date headers.
func (a *Authenticator) buildSigningString(req *http.Request) string {
	// Build the signing string according to OCI HTTP Signature specification
	// Format: (request-target): post /path\nhost: hostname\ndate: date
	requestTarget := strings.ToLower(req.Method) + " " + req.URL.Path
	if req.URL.RawQuery != "" {
		requestTarget += "?" + req.URL.RawQuery
	}

	host := req.Host
	if host == "" {
		host = req.URL.Host
	}

	date := req.Header.Get("Date")
	if date == "" {
		date = time.Now().UTC().Format(http.TimeFormat)
		req.Header.Set("Date", date)
	}

	signingString := fmt.Sprintf("(request-target): %s\nhost: %s\ndate: %s",
		requestTarget, host, date)

	return signingString
}

// extractKeyID is a convenience function that extracts only the key ID from a certificate.
func extractKeyID(certPem string) (string, error) {
	keyID, _, err := extractKeyIDAndExpiration(certPem)
	return keyID, err
}
