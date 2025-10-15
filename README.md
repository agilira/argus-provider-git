# argus-provider-git: Argus remote provider for Git repositories
### an AGILira library

Official [Argus](https://github.com/agilira/argus) provider for remote configuration management through Git repositories.
Enables real-time configuration loading and watching from GitHub, GitLab, Bitbucket, and self-hosted Git servers with production-ready security and performance features.

[![CI](https://github.com/agilira/argus-provider-git/actions/workflows/ci.yml/badge.svg)](https://github.com/agilira/argus-provider-git/actions/workflows/ci.yml)
[![CodeQL](https://github.com/agilira/argus-provider-git/actions/workflows/codeql.yml/badge.svg)](https://github.com/agilira/argus-provider-git/actions/workflows/codeql.yml)
[![Security](https://img.shields.io/badge/Security-gosec-brightgreen)](https://github.com/agilira/argus-provider-git/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/agilira/argus-provider-git)](https://goreportcard.com/report/github.com/agilira/argus-provider-git)
[![Made For Argus](https://img.shields.io/badge/Made_for-Argus-AFEEEE)](https://github.com/agilira/argus)

**[Features](#features) • [Installation](#installation) • [Quick Start](#quick-start) • [Configuration](#configuration) • [Authentication](#authentication) • [Security](#security) • [Performance](#performance)**

## Features

**GitOps Integration**
- Multi-platform Git provider support (GitHub, GitLab, Bitbucket, self-hosted)
- Efficient polling using `git ls-remote` without full repository clones
- Multiple authentication methods: tokens, SSH keys, basic auth
- Branch, tag, and commit-specific configuration loading

**Secure by Design**
- [Red-team tested](security_test.go) against path traversal and SSRF attacks
- URL validation and sanitization
- SSH key permission validation
- Resource limits for DoS protection
- [Fuzz tested](fuzz_test.go) for path traversal protection

**High Performance**
- Intelligent multi-layer caching (authentication, repository metadata, configurations)
- Retry logic with exponential backoff
- Shallow clones for minimal bandwidth usage
- Concurrent operation limits with resource management

## Compatibility

Requires Go 1.25+ and [go-git](https://github.com/go-git/go-git) v5+.

> **Security Note:** Go 1.25+ is required to avoid supply chain vulnerabilities detected by govulncheck in earlier versions.

## Installation

```bash
go get github.com/agilira/argus-provider-git
```

## Quick Start

```go
import (
    "github.com/agilira/argus"
    git "github.com/agilira/argus-provider-git"
)

// Register the Git provider with Argus
provider := git.GetProvider()
err := argus.RegisterRemoteProvider(provider)
if err != nil {
    log.Fatal("Failed to register Git provider:", err)
}

// Load configuration from Git repository
config, err := argus.LoadRemoteConfig("https://github.com/myorg/configs.git#config.json?ref=main")
if err != nil {
    log.Fatal("Failed to load config:", err)
}
```

### Direct Provider Usage

For advanced use cases, you can use the provider directly:

```go
package main

import (
    "context"
    "log"

    git "github.com/agilira/argus-provider-git"
)

func main() {
    provider := git.GetProvider()
    ctx := context.Background()
    
    // Load configuration once
    configURL := "https://github.com/myorg/configs.git#config.json?ref=production"
    config, err := provider.Load(ctx, configURL)
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }
    
    log.Printf("Configuration: %+v", config)
    
    // Watch for configuration changes  
    configURL = "https://github.com/myorg/configs.git#config.json?ref=main&poll=30s"
    configChan, err := provider.Watch(ctx, configURL)
    if err != nil {
        log.Fatalf("Failed to start watch: %v", err)
    }
    
    for config := range configChan {
        log.Printf("Configuration updated: %+v", config)
        // Apply configuration to your application
    }
}
```

## Configuration

### URL Format

Configuration URLs follow the Git repository format with additional parameters:

```
<scheme>://<host>/<path>#<file>?<parameters>
```

**Supported Schemes:**
- `https://` - HTTPS (recommended)
- `ssh://` - SSH  
- `git://` - Git protocol

**Examples:**
```bash
https://github.com/user/repo.git#config.json?ref=main
https://github.com/user/private-repo.git#config.yaml?auth=token:ghp_xxxxx
ssh://git@gitlab.com/user/repo.git#configs/prod.toml?auth=key:/path/to/key
```

### URL Parameters

**File Selection:**
- `#<file>` - Path to configuration file in repository
- `ref=<branch|tag|commit>` - Git reference (default: "main")

**Polling Configuration:**
- `poll=<duration>` - Watch polling interval (e.g., "30s", "5m", "1h")

**Authentication:**
- `auth=token:<token>` - Access token (GitHub/GitLab)
- `auth=basic:<USERNAME>:<PASSWORD>` - HTTP Basic Authentication  
- `auth=key:<path>` - SSH private key path
- `auth=ssh:<path>:<passphrase>` - SSH key with passphrase

### Supported Configuration Formats

The provider supports multiple configuration file formats:

**JSON:**
```json
{
  "database": {
    "host": "localhost",
    "port": 5432
  },
  "features": {
    "caching": true
  }
}
```

**YAML:**
```yaml
database:
  host: localhost
  port: 5432
features:
  caching: true
```

**TOML:**
```toml
[database]
host = "localhost"
port = 5432

[features]
caching = true
```

**Additional formats:** HCL, INI, and Properties files are also supported.

## Authentication

**Token Authentication:** `?auth=token:YOUR_TOKEN` (GitHub, GitLab, Bitbucket)  
**SSH Keys:** `?auth=key:/path/to/key` (requires 0600 permissions)  
**Basic Auth:** `?auth=basic:username:password` (self-hosted Git)

```bash
# Examples
https://github.com/user/repo.git#config.json?auth=token:ghp_xxxxx
ssh://git@gitlab.com/user/repo.git#config.yaml?auth=key:/home/user/.ssh/id_rsa
```

### HTTP Basic Authentication

For Git servers supporting basic authentication:

```bash
https://git.company.com/repo.git#config.json?auth=basic:YOUR_USERNAME:YOUR_PASSWORD
```

## Security

### Security Features

**[Path Traversal Protection](security_test.go#L450)** - Prevents access outside repository boundaries with 50+ attack vector tests  
**[SSRF Protection](security_test.go#L270)** - Blocks localhost, private networks, and cloud metadata access  
**[SSH Security](ssh_test.go)** - Validates key permissions and secure credential caching  
**[Automated Security](.github/workflows/codeql.yml)** - CodeQL analysis, gosec, and govulncheck

### Resource Limits

The provider enforces the following limits for security and performance:

```go
const (
    maxConfigFileSize       = 5 * 1024 * 1024  // 5MB maximum file size
    maxConcurrentOperations = 10               // Maximum parallel operations  
    maxActiveWatches       = 5                 // Maximum active watch operations
    defaultGitTimeout      = 60 * time.Second  // Git operation timeout
    minPollInterval        = 5 * time.Second   // Minimum polling interval
    maxPollInterval        = 10 * time.Minute  // Maximum polling interval
)
```

## Performance

**Multi-layer Caching** - Authentication, metadata, and configuration caching with intelligent eviction  
**Efficient Operations** - `git ls-remote` for change detection, shallow clones, exponential backoff  
**Resource Management** - Connection pooling, concurrent limits, memory-efficient file handling

## Advanced Configuration

**Environment Variables:** Use `GITHUB_TOKEN`, `GITLAB_TOKEN`, `SSH_KEY_PATH` for secure credential management  
**Multi-Environment:** Support for dev/staging/prod configurations with different repositories and branches
```

## Troubleshooting

### Common Issues

**Authentication Failures**
- Verify token has correct permissions (repo scope for private repositories)
- Ensure SSH keys are registered in your Git platform account  
- Test authentication: `git ls-remote <repo-url>`

**File Not Found**
- Verify file exists in specified repository and branch
- Check file path case sensitivity
- Ensure branch/tag exists

**Configuration Parse Errors**  
- Validate configuration file format
- Check file encoding (must be UTF-8)
- Use online validators for JSON/YAML/TOML

**Network Timeouts**
- Verify network connectivity to Git server
- Consider using tokens for better rate limits
- Increase polling intervals for watch operations

## License

Mozilla Public License 2.0 - see the [LICENSE](LICENSE.md) file for details.

---

argus-provider-redis • an AGILira library