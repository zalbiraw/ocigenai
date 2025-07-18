package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Test helper to generate a test certificate and private key
func generateTestCertAndKey(t *testing.T, expiresAt time.Time) (string, string) {
	t.Helper()
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test-instance",
		},
		NotBefore:   time.Now().Add(-1 * time.Hour),
		NotAfter:    expiresAt,
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: nil,
	}

	// Add uniqueIdentifier to subject (OID 2.5.4.45)
	template.Subject.ExtraNames = []pkix.AttributeTypeAndValue{
		{
			Type:  []int{2, 5, 4, 45}, // OID for uniqueIdentifier
			Value: "test-key-id-12345",
		},
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	// Encode certificate to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	// Encode private key to PEM
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyDER,
	})

	return string(certPEM), string(keyPEM)
}

func TestNew(t *testing.T) {
	auth := New()
	if auth == nil {
		t.Fatal("expected authenticator to be created")
	}
	if auth.cache == nil {
		t.Error("expected cache to be initialized")
	}
	if auth.client == nil {
		t.Error("expected HTTP client to be initialized")
	}
}

func TestParsePrivateKey_PKCS8(t *testing.T) {
	// Generate a test private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	// Marshal to PKCS8 format
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyDER,
	})

	// Test parsing
	parsedKey, err := parsePrivateKey(string(keyPEM))
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}

	if parsedKey.N.Cmp(privateKey.N) != 0 {
		t.Error("parsed private key does not match original")
	}
}

func TestParsePrivateKey_PKCS1(t *testing.T) {
	// Generate a test private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	// Marshal to PKCS1 format
	privateKeyDER := x509.MarshalPKCS1PrivateKey(privateKey)

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyDER,
	})

	// Test parsing
	parsedKey, err := parsePrivateKey(string(keyPEM))
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}

	if parsedKey.N.Cmp(privateKey.N) != 0 {
		t.Error("parsed private key does not match original")
	}
}

func TestParsePrivateKey_InvalidPEM(t *testing.T) {
	_, err := parsePrivateKey("invalid pem data")
	if err == nil {
		t.Error("expected error for invalid PEM data")
	}
	if !strings.Contains(err.Error(), "failed to decode PEM block") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestExtractKeyIDAndExpiration(t *testing.T) {
	expiresAt := time.Now().Add(24 * time.Hour)
	certPEM, _ := generateTestCertAndKey(t, expiresAt)

	keyID, expiration, err := extractKeyIDAndExpiration(certPEM)
	if err != nil {
		t.Fatalf("failed to extract key ID and expiration: %v", err)
	}

	expectedKeyID := "test-key-id-12345"
	if keyID != expectedKeyID {
		t.Errorf("expected key ID %s, got %s", expectedKeyID, keyID)
	}

	// Check expiration time (allow 1 second tolerance)
	if expiration.Sub(expiresAt).Abs() > time.Second {
		t.Errorf("expected expiration %v, got %v", expiresAt, expiration)
	}
}

func TestExtractKeyID_FallbackToSerialNumber(t *testing.T) {
	// Create certificate without uniqueIdentifier
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(123456789),
		Subject: pkix.Name{
			CommonName: "test-instance",
		},
		NotBefore: time.Now().Add(-1 * time.Hour),
		NotAfter:  time.Now().Add(24 * time.Hour),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	keyID, err := extractKeyID(string(certPEM))
	if err != nil {
		t.Fatalf("failed to extract key ID: %v", err)
	}

	expectedKeyID := "123456789"
	if keyID != expectedKeyID {
		t.Errorf("expected key ID %s, got %s", expectedKeyID, keyID)
	}
}

func TestBuildSigningString(t *testing.T) {
	auth := New()

	req, err := http.NewRequest(http.MethodPost, "https://generativeai.us-ashburn-1.oci.oraclecloud.com/20240101/actions/generateText", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Host = "generativeai.us-ashburn-1.oci.oraclecloud.com"
	req.Header.Set("Date", "Thu, 05 Jan 2014 21:31:40 GMT")

	signingString := auth.buildSigningString(req)

	expectedLines := []string{
		"(request-target): post /20240101/actions/generateText",
		"host: generativeai.us-ashburn-1.oci.oraclecloud.com",
		"date: Thu, 05 Jan 2014 21:31:40 GMT",
	}
	expected := strings.Join(expectedLines, "\n")

	if signingString != expected {
		t.Errorf("signing string mismatch:\nexpected:\n%s\ngot:\n%s", expected, signingString)
	}
}

func TestSignRequest(t *testing.T) {
	expiresAt := time.Now().Add(24 * time.Hour)
	certPEM, keyPEM := generateTestCertAndKey(t, expiresAt)

	privateKey, err := parsePrivateKey(keyPEM)
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}

	keyID, _, err := extractKeyIDAndExpiration(certPEM)
	if err != nil {
		t.Fatalf("failed to extract key ID: %v", err)
	}

	auth := New()
	req, err := http.NewRequest(http.MethodPost, "https://generativeai.us-ashburn-1.oci.oraclecloud.com/20240101/actions/generateText", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	err = auth.signRequest(req, privateKey, keyID)
	if err != nil {
		t.Fatalf("failed to sign request: %v", err)
	}

	// Verify authorization header is set
	authHeader := req.Header.Get("Authorization")
	if authHeader == "" {
		t.Error("authorization header not set")
	}

	if !strings.Contains(authHeader, "Signature version=\"1\"") {
		t.Error("authorization header missing signature version")
	}

	if !strings.Contains(authHeader, fmt.Sprintf("keyId=\"%s\"", keyID)) {
		t.Error("authorization header missing key ID")
	}

	if !strings.Contains(authHeader, "algorithm=\"rsa-sha256\"") {
		t.Error("authorization header missing algorithm")
	}

	if !strings.Contains(authHeader, "headers=\"(request-target) host date\"") {
		t.Error("authorization header missing headers list")
	}

	// Verify date header is set
	dateHeader := req.Header.Get("Date")
	if dateHeader == "" {
		t.Error("date header not set")
	}
}
