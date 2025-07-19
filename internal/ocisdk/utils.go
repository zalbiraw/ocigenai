// Copyright (c) 2016, 2018, 2025, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.

package ocisdk

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"fmt"
	"net/http"
	"strings"
)

// httpGet makes a simple HTTP GET request to the given URL, expecting only "200 OK" status code.
// This is basically for the Instance Metadata Service.
func httpGet(dispatcher HTTPRequestDispatcher, url string) (body bytes.Buffer, statusCode int, err error) {
	var response *http.Response
	request, err := http.NewRequest(http.MethodGet, url, nil)

	request.Header.Add("Authorization", "Bearer Oracle")

	if response, err = dispatcher.Do(request); err != nil {
		return
	}

	statusCode = response.StatusCode

	defer func() { _ = response.Body.Close() }() // nolint:errcheck
	if _, err = body.ReadFrom(response.Body); err != nil {
		return
	}

	if statusCode != http.StatusOK {
		err = fmt.Errorf("HTTP Get failed: URL: %s, Status: %s, Message: %s",
			url, response.Status, body.String())
		return
	}

	return
}

func extractTenancyIDFromCertificate(cert *x509.Certificate) string {
	for _, nameAttr := range cert.Subject.Names {
		value := nameAttr.Value.(string)
		if strings.HasPrefix(value, "opc-tenant:") {
			return value[len("opc-tenant:"):]
		}
	}
	return ""
}

func fingerprint(certificate *x509.Certificate) string {
	fingerprint := sha256.Sum256(certificate.Raw)
	return colonSeparatedString(fingerprint)
}

func colonSeparatedString(fingerprint [sha256.Size]byte) string {
	spaceSeparated := fmt.Sprintf("% x", fingerprint)
	return strings.Replace(spaceSeparated, " ", ":", -1)
}

func sanitizeCertificateString(certString string) string {
	certString = strings.Replace(certString, "-----BEGIN CERTIFICATE-----", "", -1)
	certString = strings.Replace(certString, "-----END CERTIFICATE-----", "", -1)
	certString = strings.Replace(certString, "-----BEGIN PUBLIC KEY-----", "", -1)
	certString = strings.Replace(certString, "-----END PUBLIC KEY-----", "", -1)
	certString = strings.Replace(certString, "\n", "", -1)
	return certString
}

// GetGenericConfigurationProvider checks auth config paras in config file and return the final configuration provider
func GetGenericConfigurationProvider(configProvider ConfigurationProvider) (ConfigurationProvider, error) {
	if authConfig, err := configProvider.AuthType(); err == nil && authConfig.IsFromConfigFile {
		switch authConfig.AuthType {
		case InstancePrincipal:
			return InstancePrincipalConfigurationProvider()
		}
	}
	return configProvider, nil
}
