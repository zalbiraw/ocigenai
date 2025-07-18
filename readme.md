# OCI GenAI Proxy Plugin

[![Build Status](https://github.com/zalbiraw/ocigenai/workflows/Main/badge.svg?branch=master)](https://github.com/zalbiraw/ocigenai/actions)

A Traefik plugin that seamlessly proxies OpenAI API requests to Oracle Cloud Infrastructure (OCI) Generative AI service. This plugin transforms OpenAI ChatCompletion requests to OCI GenAI format and handles Instance Principal authentication automatically.

## Features

- **Seamless API Translation**: Converts OpenAI ChatCompletion requests to OCI GenAI format
- **Instance Principal Authentication**: Automatic OCI authentication using Instance Principal credentials
- **Certificate Caching**: Intelligent caching of OCI certificates with automatic refresh
- **Thread-Safe**: Concurrent request handling with thread-safe credential management
- **Configurable Parameters**: Customizable AI model parameters with sensible defaults
- **No External Dependencies**: Custom OCI authentication implementation using only standard Go libraries

## Architecture

The plugin follows idiomatic Go project structure:

```
ocigenai/
├── internal/              # Private application code
│   ├── auth/             # OCI Instance Principal authentication
│   ├── config/           # Configuration management
│   └── transform/        # OpenAI to OCI request transformation
├── pkg/                  # Public library code
│   └── types/           # Shared data structures
├── docs/                 # Documentation
└── plugin.go            # Main plugin implementation
```

## Installation

### Static Configuration

Add the plugin to your Traefik static configuration:

```yaml
# traefik.yml
experimental:
  plugins:
    ocigenai:
      moduleName: github.com/zalbiraw/ocigenai
      version: v1.0.0
```

### Dynamic Configuration

Configure the plugin in your dynamic configuration:

```yaml
# Dynamic configuration
http:
  middlewares:
    oci-genai-proxy:
      plugin:
        ocigenai:
          compartmentId: "ocid1.compartment.oc1..your-compartment-id"
          maxTokens: 1000
          temperature: 0.8
          topP: 0.9
          frequencyPenalty: 0.1
          presencePenalty: 0.1
          topK: 50

  routers:
    openai-to-oci:
      rule: "Host(`your-domain.com`) && PathPrefix(`/v1/chat/completions`)"
      service: oci-genai-service
      middlewares:
        - oci-genai-proxy

  services:
    oci-genai-service:
      loadBalancer:
        servers:
          - url: "https://generativeai.us-ashburn-1.oci.oraclecloud.com"
```

## Configuration Options

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `compartmentId` | string | ✅ | - | OCI compartment ID where GenAI service is located |
| `maxTokens` | int | ❌ | 600 | Maximum number of tokens to generate |
| `temperature` | float64 | ❌ | 1.0 | Controls randomness (0.0-2.0) |
| `topP` | float64 | ❌ | 0.75 | Nucleus sampling parameter (0.0-1.0) |
| `frequencyPenalty` | float64 | ❌ | 0.0 | Frequency penalty (-2.0 to 2.0) |
| `presencePenalty` | float64 | ❌ | 0.0 | Presence penalty (-2.0 to 2.0) |
| `topK` | int | ❌ | 0 | Top-K sampling (0 = disabled) |

## Usage

Once configured, send OpenAI-compatible requests to your Traefik endpoint:

```bash
curl -X POST https://your-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "model": "cohere.command-r-plus",
    "messages": [
      {
        "role": "user",
        "content": "Hello, how are you?"
      }
    ],
    "max_tokens": 150,
    "temperature": 0.7
  }'
```

The plugin will:
1. Intercept the OpenAI request
2. Transform it to OCI GenAI format
3. Add Instance Principal authentication headers
4. Forward to OCI GenAI service

## Prerequisites

- **OCI Instance Principal**: The plugin must run on an OCI compute instance with Instance Principal authentication configured
- **OCI GenAI Service**: Access to Oracle Cloud Generative AI service in your compartment
- **Traefik v2.5+**: Compatible with Traefik's plugin system

## Development

### Building

```bash
go build .
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/auth
go test ./internal/config
go test ./internal/transform
```

### Project Structure

- **`internal/auth`**: OCI Instance Principal authentication with certificate caching
- **`internal/config`**: Configuration management and validation
- **`internal/transform`**: OpenAI to OCI GenAI request transformation
- **`pkg/types`**: Shared data structures and types
- **`plugin.go`**: Main plugin implementation and HTTP handler

## Authentication

The plugin uses OCI Instance Principal authentication, which requires:

1. **Instance Principal Setup**: Configure your OCI compute instance for Instance Principal authentication
2. **IAM Policies**: Ensure the instance has appropriate permissions for the GenAI service
3. **Certificate Management**: The plugin automatically handles certificate fetching and caching

### Certificate Caching

The plugin implements intelligent certificate caching:
- Certificates are cached until 1 hour before expiration
- Thread-safe concurrent access
- Automatic refresh on expiration
- Fallback handling for soon-to-expire certificates

## Troubleshooting

### Common Issues

1. **"compartmentId cannot be empty"**
   - Ensure `compartmentId` is set in your configuration

2. **"failed to get instance metadata"**
   - Verify Instance Principal is configured on your OCI instance
   - Check network connectivity to metadata service (169.254.169.254)

3. **"failed to authenticate request"**
   - Verify IAM policies allow access to GenAI service
   - Check certificate validity and expiration

### Debug Mode

Enable Traefik debug logging to see detailed plugin operation:

```yaml
log:
  level: DEBUG
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run the test suite
6. Submit a pull request

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Support

For issues and questions:
- Create an issue on GitHub
- Check the [OCI documentation](https://docs.oracle.com/en-us/iaas/Content/generative-ai/home.htm) for GenAI service details
- Review [Traefik plugin documentation](https://doc.traefik.io/traefik/plugins/)

---

**Note**: This plugin requires OCI Instance Principal authentication and must run on an OCI compute instance with appropriate IAM permissions

http:
  routers:
    my-router:
      rule: host(`demo.localhost`)
      service: service-foo
      entryPoints:
        - web
      middlewares:
        - my-plugin

  services:
   service-foo:
      loadBalancer:
        servers:
          - url: http://127.0.0.1:5000
  
  middlewares:
    my-plugin:
      plugin:
        example:
          headers:
            Foo: Bar
```

### Local Mode

Traefik also offers a developer mode that can be used for temporary testing of plugins not hosted on GitHub.
To use a plugin in local mode, the Traefik static configuration must define the module name (as is usual for Go packages) and a path to a [Go workspace](https://golang.org/doc/gopath_code.html#Workspaces), which can be the local GOPATH or any directory.

The plugins must be placed in `./plugins-local` directory,
which should be in the working directory of the process running the Traefik binary.
The source code of the plugin should be organized as follows:

```
./plugins-local/
    └── src
        └── github.com
            └── traefik
                └── plugindemo
                    ├── demo.go
                    ├── demo_test.go
                    ├── go.mod
                    ├── LICENSE
                    ├── Makefile
                    └── readme.md
```

```yaml
# Static configuration

experimental:
  localPlugins:
    example:
      moduleName: github.com/traefik/plugindemo
```

(In the above example, the `plugindemo` plugin will be loaded from the path `./plugins-local/src/github.com/traefik/plugindemo`.)

```yaml
# Dynamic configuration

http:
  routers:
    my-router:
      rule: host(`demo.localhost`)
      service: service-foo
      entryPoints:
        - web
      middlewares:
        - my-plugin

  services:
   service-foo:
      loadBalancer:
        servers:
          - url: http://127.0.0.1:5000
  
  middlewares:
    my-plugin:
      plugin:
        example:
          headers:
            Foo: Bar
```

## Defining a Plugin

A plugin package must define the following exported Go objects:

- A type `type Config struct { ... }`. The struct fields are arbitrary.
- A function `func CreateConfig() *Config`.
- A function `func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error)`.

```go
// Package example a example plugin.
package example

import (
	"context"
	"net/http"
)

// Config the plugin configuration.
type Config struct {
	// ...
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		// ...
	}
}

// Example a plugin.
type Example struct {
	next     http.Handler
	name     string
	// ...
}

// New created a new plugin.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	// ...
	return &Example{
		// ...
	}, nil
}

func (e *Example) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// ...
	e.next.ServeHTTP(rw, req)
}
```

## Logs

Currently, the only way to send logs to Traefik is to use `os.Stdout.WriteString("...")` or `os.Stderr.WriteString("...")`.

In the future, we will try to provide something better and based on levels.

## Plugins Catalog

Traefik plugins are stored and hosted as public GitHub repositories.

Every 30 minutes, the Plugins Catalog online service polls Github to find plugins and add them to its catalog.

### Prerequisites

To be recognized by Plugins Catalog, your repository must meet the following criteria:

- The `traefik-plugin` topic must be set.
- The `.traefik.yml` manifest must exist, and be filled with valid contents.

If your repository fails to meet either of these prerequisites, Plugins Catalog will not see it.

### Manifest

A manifest is also mandatory, and it should be named `.traefik.yml` and stored at the root of your project.

This YAML file provides Plugins Catalog with information about your plugin, such as a description, a full name, and so on.

Here is an example of a typical `.traefik.yml`file:

```yaml
# The name of your plugin as displayed in the Plugins Catalog web UI.
displayName: Name of your plugin

# For now, `middleware` is the only type available.
type: middleware

# The import path of your plugin.
import: github.com/username/my-plugin

# A brief description of what your plugin is doing.
summary: Description of what my plugin is doing

# Medias associated to the plugin (optional)
iconPath: foo/icon.png
bannerPath: foo/banner.png

# Configuration data for your plugin.
# This is mandatory,
# and Plugins Catalog will try to execute the plugin with the data you provide as part of its startup validity tests.
testData:
  Headers:
    Foo: Bar
```

Properties include:

- `displayName` (required): The name of your plugin as displayed in the Plugins Catalog web UI.
- `type` (required): For now, `middleware` is the only type available.
- `import` (required): The import path of your plugin.
- `summary` (required): A brief description of what your plugin is doing.
- `testData` (required): Configuration data for your plugin. This is mandatory, and Plugins Catalog will try to execute the plugin with the data you provide as part of its startup validity tests.
- `iconPath` (optional): A local path in the repository to the icon of the project.
- `bannerPath` (optional): A local path in the repository to the image that will be used when you will share your plugin page in social medias.

There should also be a `go.mod` file at the root of your project. Plugins Catalog will use this file to validate the name of the project.

### Tags and Dependencies

Plugins Catalog gets your sources from a Go module proxy, so your plugins need to be versioned with a git tag.

Last but not least, if your plugin middleware has Go package dependencies, you need to vendor them and add them to your GitHub repository.

If something goes wrong with the integration of your plugin, Plugins Catalog will create an issue inside your Github repository and will stop trying to add your repo until you close the issue.

## Troubleshooting

If Plugins Catalog fails to recognize your plugin, you will need to make one or more changes to your GitHub repository.

In order for your plugin to be successfully imported by Plugins Catalog, consult this checklist:

- The `traefik-plugin` topic must be set on your repository.
- There must be a `.traefik.yml` file at the root of your project describing your plugin, and it must have a valid `testData` property for testing purposes.
- There must be a valid `go.mod` file at the root of your project.
- Your plugin must be versioned with a git tag.
- If you have package dependencies, they must be vendored and added to your GitHub repository.
