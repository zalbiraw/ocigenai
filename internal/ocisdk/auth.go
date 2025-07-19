// Package ocisdk provides Oracle Cloud Infrastructure (OCI) Instance Principal authentication
// for the OCI GenAI proxy plugin. It implements custom OCI request signing without
// requiring the official OCI SDK, using only standard Go libraries.
package ocisdk

import (
	"fmt"
	"net/http"
)

// Authenticator handles OCI Instance Principal authentication and request signing.
type Authenticator struct {
	signer HTTPRequestSigner
}

// New creates a new authenticator with default settings.
func New() *Authenticator {
	auth := &Authenticator{}

	provider, err := InstancePrincipalConfigurationProvider()
	if err != nil {
		panic(fmt.Errorf("error getting provider: %v", err))
	}

	// Create the OCI request signer using the key provider
	auth.signer = DefaultRequestSigner(provider)

	return auth
}

// SignRequest adds OCI authentication headers to the given HTTP request.
// It uses cached credentials when available or fetches fresh ones if needed.
func (a *Authenticator) SignRequest(req *http.Request) error {
	// Use the OCI request signer to sign the request
	if err := a.signer.Sign(req); err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	return nil
}
