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
- Red-team tested against path traversal and SSRF attacks
- URL validation and sanitization
- SSH key permission validation
- Resource limits for DoS protection
- Comprehensive audit logging

**High Performance**
- Intelligent multi-layer caching (authentication, repository metadata, configurations)
- Retry logic with exponential backoff
- Shallow clones for minimal bandwidth usage
- Concurrent operation limits with resource management

## Compatibility and Support

argus-provider-git works with Git servers supporting the Git protocol and follows Long-Term Support guidelines to ensure consistent performance across production deployments.

## Installation

```bash
go get github.com/agilira/argus-provider-git
```

## Quick Start

To use the provider, import it and register it with Argus:

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/agilira/argus"
    git "github.com/agilira/argus-provider-git"
)

func main() {
    // Create Argus configuration watcher
    watcher := argus.New(argus.Config{
        Remote: argus.RemoteConfig{
            Enabled:     true,
            PrimaryURL:  "https://github.com/myorg/configs.git#config.json?ref=main",
            SyncInterval: 30 * time.Second,
        },
    })
    
    // Start watching for configuration changes
    watcher.Start()
    defer watcher.Stop()
    
    // Access loaded configuration
    config := watcher.Get()
    log.Printf("Configuration loaded: %+v", config)
    
    // Your application logic here
    select {}
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
# GitHub public repository
https://github.com/user/repo.git#config.json?ref=main

# GitHub private repository with token
https://github.com/user/private-repo.git#config.yaml?ref=v1.0&auth=token:ghp_xxxxx

# GitLab with SSH key
ssh://git@gitlab.com/user/repo.git#configs/prod.toml?auth=key:/path/to/key

# Self-hosted Git server
https://git.company.com/configs.git#app.json?ref=production&auth=basic:MYUSER:MYPASS
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

### GitHub Token Authentication

Create a Personal Access Token in GitHub Settings:

```bash
# Required scopes: repo (for private repositories)
https://github.com/user/private-repo.git#config.json?auth=token:ghp_xxxxxxxxx
```

### GitLab Token Authentication

Create a Project Access Token in GitLab:

```bash
# Required scopes: read_repository
https://gitlab.com/user/repo.git#config.json?auth=token:glpat-xxxxxxx
```

### SSH Key Authentication

Ensure your SSH key is registered in your Git platform account:

```bash
# SSH key without passphrase
ssh://git@github.com/user/repo.git#config.json?auth=key:/home/user/.ssh/id_rsa

# SSH key with passphrase
ssh://git@gitlab.com/user/repo.git#config.yaml?auth=ssh:/path/to/key:mypassphrase
```

**Security Requirements:**
- SSH key file must have permissions 0600 or more restrictive
- Key file must be accessible by the application user

### HTTP Basic Authentication

For Git servers supporting basic authentication:

```bash
https://git.company.com/repo.git#config.json?auth=basic:YOUR_USERNAME:YOUR_PASSWORD
```

## Security

### Security Features

**Path Traversal Protection**
- Validates and sanitizes all file paths
- Prevents access outside repository boundaries
- Blocks access to sensitive system files

**Network Security**  
- URL validation and normalization
- SSRF attack prevention
- Resource exhaustion protection

**Authentication Security**
- SSH key permission validation
- Credential caching with secure storage
- Authentication failure logging

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

### Caching System

The provider implements multi-layer caching for optimal performance:

**Authentication Cache**
- Caches authentication objects to avoid re-creation
- Automatic cache invalidation and renewal

**Repository Metadata Cache**  
- Stores commit hashes for efficient change detection
- Reduces network calls during polling operations

**Configuration Cache**
- Caches loaded configurations by commit hash
- Intelligent cache eviction based on TTL and capacity

### Performance Optimizations

**Efficient Git Operations**
- Uses `git ls-remote` for change detection without cloning
- Shallow clones minimize bandwidth and storage requirements
- Retry logic with exponential backoff for reliability

**Resource Management**
- Connection pooling for multiple repositories
- Concurrent operation limits prevent resource exhaustion
- Memory-efficient file handling for large configurations

## Advanced Configuration

### Environment Variables

For secure credential management:

```bash
export GITHUB_TOKEN="ghp_xxxxxxxxxxxxxxxx"
export GITLAB_TOKEN="glpat-xxxxxxxxxxxxxxx"
export SSH_KEY_PATH="/secure/path/to/key"
```

```go
token := os.Getenv("GITHUB_TOKEN")
configURL := fmt.Sprintf("https://github.com/org/repo.git#config.json?auth=token:%s", token)
```

### Multi-Environment Setup

```go
type ConfigManager struct {
    provider git.RemoteConfigProvider
    envConfigs map[string]string
}

func NewConfigManager() *ConfigManager {
    return &ConfigManager{
        provider: git.GetProvider(),
        envConfigs: map[string]string{
            "dev":  "https://github.com/myorg/configs.git#dev.json?ref=develop",
            "prod": "https://github.com/myorg/configs.git#prod.json?ref=main&auth=token:xxx",
        },
    }
}

func (cm *ConfigManager) LoadConfig(env string) (map[string]interface{}, error) {
    configURL, exists := cm.envConfigs[env]
    if !exists {
        return nil, fmt.Errorf("unknown environment: %s", env)
    }
    
    return cm.provider.Load(context.Background(), configURL)
}
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

## Requirements

- Go 1.21 or later
- Git client (for SSH authentication)
- Network access to Git repositories

## License

This project is licensed under the Mozilla Public License 2.0 - see the [LICENSE](LICENSE.md) file for details.

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Support

- **Documentation**: [GitHub Repository](https://github.com/agilira/argus-provider-git)
- **Issues**: [Issue Tracker](https://github.com/agilira/argus-provider-git/issues)
- **Security**: Please report security issues privately to security@agilira.com

---

Part of the [Argus](https://github.com/agilira/argus) ecosystem by [AGILira](https://github.com/agilira).