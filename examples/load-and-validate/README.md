# Load and Validate Example - Argus Git Provider

This example demonstrates advanced validation scenarios, error handling, and comprehensive testing of the Argus Git Provider functionality.

## Overview

The load and validate example shows how to:
- Test multiple URL validation scenarios
- Handle different timeout configurations
- Perform comprehensive error handling
- Test provider health checks
- Demonstrate robust configuration loading patterns

## Prerequisites

- Go 1.19 or later
- Internet connection for accessing remote Git repositories
- Access to GitHub or other Git hosting services for integration tests

## Dependencies

This example uses the following dependencies (managed automatically via `go.mod`):

```go
require github.com/agilira/argus-provider-git v0.0.0
```

## Running the Example

From the `examples/load-and-validate` directory:

```bash
go run main.go
```

Or run with comprehensive tests:

```bash
go test -v
```

For faster testing (skip integration tests):

```bash
go test -short
```

## Key Features Demonstrated

### 1. Multiple URL Validation Scenarios
The example tests various URL formats to demonstrate validation capabilities:

- **Valid URLs**: Properly formatted repository URLs with valid file paths
- **Invalid formats**: Malformed URLs that should fail validation
- **Security restrictions**: URLs pointing to localhost or private networks (blocked for security)
- **Missing components**: URLs lacking required components like file paths

### 2. Timeout Management
Tests different timeout scenarios:

```go
timeouts := []time.Duration{
    1 * time.Second,  // Short timeout (might fail)
    10 * time.Second, // Medium timeout
    30 * time.Second, // Long timeout (should work)
}
```

### 3. Health Check Functionality
Demonstrates provider health checking:

```go
err := provider.HealthCheck(ctx, validURL)
```

## Code Structure

### Validation Test Cases
```go
testURLs := []struct {
    url         string
    description string
    shouldPass  bool
}{
    {
        url:         "https://github.com/agilira/argus-provider-git.git#testdata/config.json?ref=main",
        description: "Valid repository with JSON config",
        shouldPass:  true,
    },
    {
        url:         "invalid-url-format",
        description: "Invalid URL format",
        shouldPass:  false,
    },
    // ... more test cases
}
```

### Progressive Timeout Testing
```go
for i, timeout := range timeouts {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    start := time.Now()
    
    config, err := provider.Load(ctx, validURL)
    duration := time.Since(start)
    cancel()
    
    if err != nil && i < len(timeouts)-1 {
        // Try with longer timeout
        continue
    }
    // Handle success or final failure
}
```

## Expected Output

```
Argus Git Provider - Load and Validate Example
==================================================
Provider initialized: Git Configuration Provider

Testing URL validation...

1. Valid repository with JSON config
   URL: https://github.com/agilira/argus-provider-git.git#testdata/config.json?ref=main
   PASS: Validation passed (as expected)

2. VS Code repository package.json
   URL: https://github.com/microsoft/vscode.git#package.json?ref=main
   PASS: Validation passed (as expected)

3. Invalid URL format
   URL: invalid-url-format
   PASS: Validation correctly failed: invalid URL format

4. Localhost (security blocked)
   URL: https://localhost/repo.git#config.json
   PASS: Validation correctly failed: localhost not allowed for security

Loading configuration from: https://github.com/agilira/argus-provider-git.git#testdata/config.json?ref=main

Attempt 1 with 1s timeout...
   FAIL: Load failed after 1.2s: context deadline exceeded
    Trying with longer timeout...

Attempt 2 with 10s timeout...
   PASS: Load succeeded in 3.4s
   Configuration loaded: 3 keys
   Sample configuration keys:
      - database
      - logging
      - features

Testing provider health check...
   PASS: Health check passed - repository is accessible

Load and validate example completed!
```

## Validation Scenarios

### Security Validations
The provider includes security validations that block:

- **Localhost URLs**: `https://localhost/repo.git#config.json`
- **Private network IPs**: `https://192.168.1.1/repo.git#config.json`
- **File protocol**: `file:///local/path/config.json`
- **Path traversal**: `https://github.com/user/repo.git#../../../etc/passwd`

### Format Validations
The provider validates:

- **URL structure**: Must follow Git URL conventions
- **File extensions**: Only `.json`, `.yaml`, `.yml`, `.toml` supported
- **Required components**: Repository URL and file path must be present
- **Git references**: Branch, tag, or commit hash format validation

## Error Handling Patterns

### Network Errors
```go
if err != nil {
    if ctx.Err() == context.DeadlineExceeded {
        // Handle timeout
    } else if strings.Contains(err.Error(), "no such host") {
        // Handle DNS resolution failure
    } else {
        // Handle other network errors
    }
}
```

### Repository Errors
- **Repository not found**: Invalid repository URL or access denied
- **File not found**: Specified file doesn't exist in repository
- **Invalid reference**: Branch, tag, or commit doesn't exist
- **Authentication required**: Private repository without proper credentials

### Configuration Errors
- **Parse error**: Invalid JSON/YAML/TOML format
- **Empty file**: Configuration file is empty
- **Invalid encoding**: File encoding not supported

## Testing Capabilities

### Unit Tests
```bash
# Test provider initialization
go test -v -run TestLoadAndValidate

# Test validation scenarios
go test -v -run TestMultipleURLValidation

# Test timeout handling
go test -v -run TestTimeoutScenarios
```

### Integration Tests
```bash
# Test with real repositories (requires network)
go test -v

# Test health checks
go test -v -run TestHealthCheckScenarios
```

### Benchmarks
```bash
# Benchmark validation performance
go test -bench=BenchmarkMultipleValidation

# Benchmark with memory profiling
go test -bench=. -memprofile=mem.prof
```

## Performance Metrics

The example measures and reports:

- **Validation time**: Time to validate URL format
- **Network latency**: Time to establish connection
- **Download time**: Time to fetch configuration file
- **Parse time**: Time to parse configuration format
- **Total operation time**: End-to-end operation timing

## Advanced Configuration

### Environment Variables

```bash
# Enable debug logging
export ARGUS_DEBUG=true

# Set custom timeout
export ARGUS_TIMEOUT=60s

# Configure retry behavior
export ARGUS_MAX_RETRIES=3
export ARGUS_RETRY_DELAY=1s

# Authentication for private repositories
export GITHUB_TOKEN="your-token"
export GITLAB_TOKEN="your-token"
```

### Provider Options

```go
// Custom timeout configuration
ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)

// Custom validation options (if supported by provider)
opts := &git.ValidationOptions{
    AllowLocalhost: false,
    AllowPrivateNetworks: false,
    MaxFileSize: 1024 * 1024, // 1MB limit
}
```

## Production Considerations

### Error Handling Strategy
- Implement exponential backoff for retries
- Add circuit breaker pattern for repeated failures
- Log all validation and loading attempts for monitoring
- Set appropriate timeouts based on network conditions

### Security Best Practices
- Never disable security validations in production
- Use environment variables for authentication tokens
- Regularly rotate access tokens and SSH keys
- Monitor for unauthorized access attempts

### Performance Optimization
- Implement caching for frequently accessed configurations
- Use connection pooling for multiple requests
- Consider using webhooks for real-time configuration updates
- Monitor repository access patterns and rate limits

## Troubleshooting

### Debug Mode
Enable comprehensive debug logging:

```bash
export ARGUS_DEBUG=true
export ARGUS_LOG_LEVEL=debug
go run main.go
```

### Common Issues and Solutions

1. **Consistent timeout errors**
   - Check network connectivity and DNS resolution
   - Verify repository is accessible and not rate-limited
   - Consider using a local Git cache or mirror

2. **Validation failures for valid URLs**
   - Check URL format matches expected pattern exactly
   - Verify file extension is supported (.json, .yaml, .toml)
   - Ensure no URL encoding issues

3. **Authentication errors**
   - Verify tokens have appropriate repository access permissions
   - Check token expiration dates
   - Ensure SSH keys are properly configured

4. **Health check failures**
   - Verify repository exists and is accessible
   - Check for repository maintenance or outages
   - Confirm network policies allow Git protocol access

## Next Steps

After mastering load and validate patterns:
- Integrate with Argus configuration management system
- Implement custom validation rules for your use case
- Set up monitoring and alerting for configuration loading
- Explore advanced features like configuration watching and caching

## Related Examples

- [URL Validation Example](../validation/) - URL format validation without network operations
- [Basic Usage Example](../basic-usage/) - Standard configuration loading workflow