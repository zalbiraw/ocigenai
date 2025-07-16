// Package ocigenai is a Traefik plugin to proxy requests to OCI Generative AI using Instance Principals.
package ocigenai

import (
	"fmt"
	"net/http"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
)

func (p *Proxy) getAuthHeaders(req *http.Request) error {
	// Create instance principal configuration provider
	configProvider, err := auth.InstancePrincipalConfigurationProvider()
	if err != nil {
		return fmt.Errorf("failed to create instance principal config provider: %w", err)
	}

	// Create a signer using the instance principal
	signer := common.DefaultRequestSigner(configProvider)

	// Sign the request
	err = signer.Sign(req)
	if err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	return nil
}
