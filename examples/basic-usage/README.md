# Basic Usage Example - Argus Git Provider

This example demonstrates the complete workflow for loading configuration from Git repositories using the Argus Git Provider.

## Overview

The basic usage example shows how to:
- Initialize the Git provider
- Validate repository URLs
- Load configuration from remote Git repositories
- Handle timeouts and context management
- Display provider capabilities

## Prerequisites

- Go 1.19 or later
- Internet connection for accessing remote Git repositories
- Access to GitHub or other Git hosting services

## Dependencies

This example uses the following dependencies (managed automatically via `go.mod`):

```go
require github.com/agilira/argus-provider-git v0.0.0
```

## Running the Example

From the `examples/basic-usage` directory:

```bash
go run main.go
```

Or run with tests:

```bash
go test -v
```

## Code Walkthrough

### Step 1: Provider Initialization
```go
provider := git.GetProvider()
fmt.Printf("Provider: %s (scheme: %s)\n", provider.Name(), provider.Scheme())
```

### Step 2: URL Configuration
```go
configURL := "https://github.com/agilira/argus-provider-git.git#testdata/config.json?ref=main"
```

### Step 3: URL Validation
```go
if err := provider.Validate(configURL); err != nil {
    log.Fatalf("URL validation failed: %v", err)
}
```

### Step 4: Configuration Loading
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

config, err := provider.Load(ctx, configURL)
```

### Step 5: Configuration Usage
```go
for key, value := range config {
    fmt.Printf("  - %s: %v\n", key, value)
}
```

## Expected Output

```
Argus Git Provider - Basic Usage Example
==================================================
Initializing Git Provider...
Provider: Git Configuration Provider (scheme: git)
Configuration URL: https://github.com/agilira/argus-provider-git.git#testdata/config.json?ref=main

Validating URL format...
URL validation passed

Loading configuration from Git repository...
Configuration loaded successfully! (3 keys)

Configuration content:
  - database: map[host:localhost port:5432 name:myapp]
  - logging: map[level:info output:stdout]
  - features: map[cache:true metrics:true]

Provider capabilities:
  - Handles scheme: git://
  - Supports formats: JSON, YAML, TOML
  - Authentication: Token, SSH, Basic Auth
  - Features: Caching, Watching, Validation

Basic usage example completed successfully!
```

## Configuration File Requirements

The target configuration file must:

1. **Be accessible**: The repository must be publicly accessible or you must have proper authentication
2. **Exist at the specified path**: The file path in the URL must exist in the repository
3. **Have a supported format**: Currently supports JSON, YAML, and TOML formats
4. **Be valid**: The file must contain valid configuration data in the specified format

## URL Format Specification

```
https://[host]/[owner]/[repository].git#[file-path]?ref=[git-reference]
```

### Components:
- **host**: Git hosting service (e.g., `github.com`, `gitlab.com`)
- **owner**: Repository owner/organization name
- **repository**: Repository name
- **file-path**: Path to configuration file within the repository
- **git-reference**: Branch name, tag, or commit hash (optional, defaults to default branch)

### Examples:
```
https://github.com/user/app-config.git#config/production.json?ref=main
https://gitlab.com/company/configs.git#environments/staging.yaml?ref=v1.2.0
https://github.com/org/settings.git#app.toml?ref=feature-branch
```

## Authentication

The provider supports multiple authentication methods:

### 1. Public Repositories
No authentication required for public repositories.

### 2. Private Repositories
For private repositories, set environment variables:

```bash
# GitHub Personal Access Token
export GITHUB_TOKEN="your-token-here"

# SSH Key (for SSH URLs)
export SSH_PRIVATE_KEY_PATH="/path/to/private/key"

# Basic Authentication
export GIT_USERNAME="username"
export GIT_PASSWORD="password"
```

## Error Handling

The example demonstrates proper error handling for:

- **URL validation errors**: Invalid format or unsupported schemes
- **Network timeouts**: Connection or response timeouts
- **Authentication failures**: Invalid credentials or insufficient permissions
- **File not found**: Specified configuration file doesn't exist
- **Parse errors**: Invalid configuration file format

## Testing

Run the included tests to verify functionality:

```bash
# Run all tests
go test -v

# Run specific test
go test -v -run TestBasicUsage

# Run with coverage
go test -cover

# Skip integration tests (for faster testing)
go test -short
```

## Performance Considerations

- **Timeout Settings**: Adjust context timeout based on network conditions and repository size
- **Caching**: The provider includes built-in caching for repeated requests
- **Rate Limiting**: Be aware of Git hosting service rate limits

## Troubleshooting

### Common Issues

1. **"context deadline exceeded"**
   - Solution: Increase timeout duration or check network connectivity

2. **"validation failed"**
   - Solution: Verify URL format matches the expected pattern

3. **"repository not found"**
   - Solution: Check repository URL and access permissions

4. **"file not found"**
   - Solution: Verify the file path and Git reference exist

### Debug Mode

Enable debug logging for troubleshooting:

```bash
export ARGUS_DEBUG=true
go run main.go
```

## Next Steps

After mastering basic usage, explore:
- [Load and Validate Example](../load-and-validate/) - Advanced validation and error handling scenarios
- Main Argus documentation for integration patterns
- Provider-specific configuration options and advanced features

## Related Examples

- [URL Validation Example](../validation/) - URL format validation without network operations
- [Load and Validate Example](../load-and-validate/) - Advanced validation and error handling scenarios