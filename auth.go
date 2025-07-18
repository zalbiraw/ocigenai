// Package ocigenai is a Traefik plugin to proxy requests to OCI Generative AI using Instance Principals.
package ocigenai

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
	"time"
)

const (
	// OCI Instance Metadata Service endpoints
	metadataBaseURL = "http://169.254.169.254/opc/v2/"
	identityURL     = metadataBaseURL + "identity/"
	certificateURL  = identityURL + "cert.pem"
	intermediateURL = identityURL + "intermediate.pem"
	privateKeyURL   = identityURL + "key.pem"
)

type instanceMetadata struct {
	CertPem         string `json:"certPem"`
	IntermediatePem string `json:"intermediatePem"`
	KeyPem          string `json:"keyPem"`
}

func (p *Proxy) getAuthHeaders(req *http.Request) error {
	// Get instance metadata (certificate and private key)
	metadata, err := p.getInstanceMetadata()
	if err != nil {
		return fmt.Errorf("failed to get instance metadata: %w", err)
	}

	// Parse the private key
	privateKey, err := parsePrivateKey(metadata.KeyPem)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// Parse the certificate to get key ID
	keyID, err := extractKeyID(metadata.CertPem)
	if err != nil {
		return fmt.Errorf("failed to extract key ID: %w", err)
	}

	// Sign the request
	err = p.signRequest(req, privateKey, keyID)
	if err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	return nil
}

func (p *Proxy) getInstanceMetadata() (*instanceMetadata, error) {
	// Create HTTP client for metadata requests
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Get certificate
	certReq, err := http.NewRequest("GET", certificateURL, nil)
	if err != nil {
		return nil, err
	}
	certReq.Header.Set("Authorization", "Bearer Oracle")

	certResp, err := client.Do(certReq)
	if err != nil {
		return nil, err
	}
	defer certResp.Body.Close()

	certPem, err := io.ReadAll(certResp.Body)
	if err != nil {
		return nil, err
	}

	// Get intermediate certificate
	intReq, err := http.NewRequest("GET", intermediateURL, nil)
	if err != nil {
		return nil, err
	}
	intReq.Header.Set("Authorization", "Bearer Oracle")

	intResp, err := client.Do(intReq)
	if err != nil {
		return nil, err
	}
	defer intResp.Body.Close()

	intPem, err := io.ReadAll(intResp.Body)
	if err != nil {
		return nil, err
	}

	// Get private key
	keyReq, err := http.NewRequest("GET", privateKeyURL, nil)
	if err != nil {
		return nil, err
	}
	keyReq.Header.Set("Authorization", "Bearer Oracle")

	keyResp, err := client.Do(keyReq)
	if err != nil {
		return nil, err
	}
	defer keyResp.Body.Close()

	keyPem, err := io.ReadAll(keyResp.Body)
	if err != nil {
		return nil, err
	}

	return &instanceMetadata{
		CertPem:         string(certPem),
		IntermediatePem: string(intPem),
		KeyPem:          string(keyPem),
	}, nil
}

func parsePrivateKey(keyPem string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(keyPem))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 format
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

	return key, nil
}

func extractKeyID(certPem string) (string, error) {
	block, _ := pem.Decode([]byte(certPem))
	if block == nil {
		return "", fmt.Errorf("failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Extract key ID from certificate subject
	for _, name := range cert.Subject.Names {
		if name.Type.String() == "2.5.4.45" { // OID for uniqueIdentifier
			return name.Value.(string), nil
		}
	}

	// Fallback: use certificate serial number as key ID
	return cert.SerialNumber.String(), nil
}

func (p *Proxy) signRequest(req *http.Request, privateKey *rsa.PrivateKey, keyID string) error {
	// Build signing string
	signingString, err := p.buildSigningString(req)
	if err != nil {
		return err
	}

	// Sign the string
	hashed := sha256.Sum256([]byte(signingString))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashed[:])
	if err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	// Encode signature
	encodedSignature := base64.StdEncoding.EncodeToString(signature)

	// Set authorization header
	authorization := fmt.Sprintf(`Signature version="1",keyId="%s",algorithm="rsa-sha256",headers="(request-target) host date",signature="%s"`,
		keyID, encodedSignature)

	req.Header.Set("Authorization", authorization)
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))

	return nil
}

func (p *Proxy) buildSigningString(req *http.Request) (string, error) {
	// Build the signing string according to OCI specification
	var parts []string

	// (request-target)
	requestTarget := fmt.Sprintf("%s %s", strings.ToLower(req.Method), req.URL.RequestURI())
	parts = append(parts, fmt.Sprintf("(request-target): %s", requestTarget))

	// host
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	parts = append(parts, fmt.Sprintf("host: %s", host))

	// date
	date := req.Header.Get("Date")
	if date == "" {
		date = time.Now().UTC().Format(http.TimeFormat)
		req.Header.Set("Date", date)
	}
	parts = append(parts, fmt.Sprintf("date: %s", date))

	return strings.Join(parts, "\n"), nil
}
