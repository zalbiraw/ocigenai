// Package auth provides Oracle Cloud Infrastructure (OCI) Instance Principal authentication
// for the OCI GenAI proxy plugin. It implements custom OCI request signing without
// requiring the official OCI SDK, using only standard Go libraries.
package auth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
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
}

// New creates a new authenticator with default settings.
func New() *Authenticator {
	return &Authenticator{
		cache: &CachedCredentials{},
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SignRequest adds OCI authentication headers to the given HTTP request.
// It uses cached credentials when available or fetches fresh ones if needed.
func (a *Authenticator) SignRequest(req *http.Request) error {
	// Get cached or fresh credentials
	privateKey, keyID, err := a.getCredentials()
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}

	// Sign the request with the credentials
	if err := a.signRequest(req, privateKey, keyID); err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

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
		return privateKey, keyID, nil
	}
	a.cache.mu.RUnlock()

	// Need to refresh credentials (write lock)
	a.cache.mu.Lock()
	defer a.cache.mu.Unlock()

	// Double-check in case another goroutine already refreshed
	if a.cache.metadata != nil && time.Now().Before(a.cache.expiresAt) {
		return a.cache.privateKey, a.cache.keyID, nil
	}

	// Fetch fresh metadata from OCI Instance Metadata Service
	metadata, err := a.fetchInstanceMetadata()
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch instance metadata: %w", err)
	}

	// Parse the private key from PEM format
	privateKey, err := parsePrivateKey(metadata.KeyPem)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse private key: %w", err)
	}

	// Extract key ID and certificate expiration time
	keyID, expiresAt, err := extractKeyIDAndExpiration(metadata.CertPem)
	if err != nil {
		return nil, "", fmt.Errorf("failed to extract key ID and expiration: %w", err)
	}

	// Calculate cache expiration with safety margin
	cacheExpiresAt := expiresAt.Add(-defaultCacheBuffer)
	if cacheExpiresAt.Before(time.Now()) {
		// If certificate expires soon, cache for minimum time
		cacheExpiresAt = time.Now().Add(minCacheBuffer)
	}

	// Update cache
	a.cache.metadata = metadata
	a.cache.privateKey = privateKey
	a.cache.keyID = keyID
	a.cache.expiresAt = cacheExpiresAt

	return privateKey, keyID, nil
}

// fetchInstanceMetadata retrieves certificates and private key from OCI Instance Metadata Service.
func (a *Authenticator) fetchInstanceMetadata() (*types.InstanceMetadata, error) {
	// Fetch certificate
	certPem, err := a.fetchMetadataEndpoint(certificateURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch certificate: %w", err)
	}

	// Fetch intermediate certificate
	intermediatePem, err := a.fetchMetadataEndpoint(intermediateURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch intermediate certificate: %w", err)
	}

	// Fetch private key
	keyPem, err := a.fetchMetadataEndpoint(privateKeyURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch private key: %w", err)
	}

	return &types.InstanceMetadata{
		CertPem:         string(certPem),
		IntermediatePem: string(intermediatePem),
		KeyPem:          string(keyPem),
	}, nil
}

// fetchMetadataEndpoint makes an authenticated request to an OCI metadata endpoint.
func (a *Authenticator) fetchMetadataEndpoint(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	// OCI metadata service requires this specific authorization header
	req.Header.Set("Authorization", "Bearer Oracle")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log error but don't fail the request - could add logging here if needed
			_ = closeErr // Explicitly ignore the error to satisfy linter
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metadata service returned status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
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

// signRequest signs an HTTP request according to OCI specification.
// It creates a signature using RSA-SHA256 and adds the appropriate headers.
func (a *Authenticator) signRequest(req *http.Request, privateKey *rsa.PrivateKey, keyID string) error {
	// Build the signing string according to OCI specification
	signingString := a.buildSigningString(req)

	// Create SHA-256 hash of the signing string
	hashed := sha256.Sum256([]byte(signingString))

	// Sign the hash using RSA-PKCS1v15
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashed[:])
	if err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	// Encode signature to base64
	encodedSignature := base64.StdEncoding.EncodeToString(signature)

	// Set OCI authorization header
	authorization := fmt.Sprintf(
		`Signature version="1",keyId="%s",algorithm="rsa-sha256",headers="(request-target) host date",signature="%s"`,
		keyID, encodedSignature,
	)

	req.Header.Set("Authorization", authorization)
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))

	return nil
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
