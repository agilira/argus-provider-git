// Package git provides a high-performance Git remote configuration provider for Argus.
//
// # Overview
//
// This package implements the Argus RemoteConfigProvider interface to enable real-time
// configuration loading and monitoring from Git repositories (GitHub, GitLab, Bitbucket,
// self-hosted). The provider leverages Git's native capabilities for efficient configuration
// management with comprehensive security validation and GitOps best practices.
//
// The implementation follows high-performance design principles with connection pooling,
// shallow clones, efficient polling mechanisms, and minimal memory allocations during runtime
// operations. It's designed for production environments where security and reliability matter.
//
// # Key Features
//
//   - GitOps Configuration Management: Load configs from Git repositories with full versioning
//   - Multi-Platform Support: GitHub, GitLab, Bitbucket, and self-hosted Git servers
//   - Multiple Authentication Methods: Personal tokens, SSH keys, basic auth
//   - Real-time Polling: Efficient repository monitoring for configuration changes
//   - Branch/Tag/Commit Support: Flexible reference targeting (main, v1.2.3, commit SHA)
//   - Format Detection: JSON, YAML, TOML configuration file support
//   - Security Hardened: Red-team tested against SSRF, path traversal, and injection attacks
//   - Performance Optimized: Shallow clones, connection reuse, and caching mechanisms
//   - Thread-Safe Operations: Concurrent access with proper synchronization
//   - Graceful Shutdown: Clean resource management and connection cleanup
//
// # URL Format and Configuration
//
// The provider accepts Git URLs in the following format:
//
//	git://host.com/user/repo.git#config/file.json[?query_params]
//
// Where query_params can include:
//   - ref=main: Specify Git reference (branch, tag, or commit SHA)
//   - token=ghp_xxxx: GitHub/GitLab personal access token
//   - ssh_key=/path/to/key: Path to SSH private key for authentication
//   - poll=30s: Custom polling interval for watch operations
//
// The URL fragment (#) specifies the configuration file path within the repository.
//
// # Examples
//
// Basic configuration loading from GitHub:
//
//	import (
//	    "context"
//	    "log"
//
//	    "github.com/agilira/argus"
//	    git "github.com/agilira/argus-provider-git"
//	)
//
//	func main() {
//	    // Register the Git provider with Argus
//	    provider, err := git.GetProvider()
//	    if err != nil {
//	        log.Fatal("Failed to create Git provider:", err)
//	    }
//
//	    if err := argus.RegisterRemoteProvider("git", provider); err != nil {
//	        log.Fatal("Failed to register provider:", err)
//	    }
//
//	    // Load configuration from GitHub repository
//	    configURL := "git://github.com/company/configs.git#production/app.json?ref=main"
//	    config, err := argus.LoadRemoteConfig(configURL)
//	    if err != nil {
//	        log.Fatal("Configuration loading failed:", err)
//	    }
//
//	    log.Printf("Configuration loaded: %+v", config)
//	}
//
// Real-time configuration monitoring with Git polling:
//
//	func watchConfiguration() {
//	    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
//	    defer cancel()
//
//	    // Start watching for Git repository changes
//	    configURL := "git://github.com/company/configs.git#production/app.json?" +
//	        "ref=main&poll=30s&token=ghp_your_token_here"
//
//	    configChan, err := argus.WatchRemoteConfigWithContext(ctx, configURL)
//	    if err != nil {
//	        log.Fatal("Watch startup failed:", err)
//	    }
//
//	    // Handle real-time configuration updates
//	    go func() {
//	        for newConfig := range configChan {
//	            log.Printf("Git configuration updated: %+v", newConfig)
//	            // Apply new configuration to your application
//	        }
//	    }()
//
//	    <-ctx.Done()
//	}
//
// SSH authentication with private keys:
//
//	sshURL := "git://git.company.com/devops/configs.git#staging/database.yml?" +
//	    "ref=v2.1.0&ssh_key=/home/user/.ssh/deploy_key"
//
//	config, err := argus.LoadRemoteConfig(sshURL)
//	if err != nil {
//	    log.Fatal("SSH authentication failed:", err)
//	}
//
// GitLab with personal access token:
//
//	gitlabURL := "git://gitlab.example.com/infrastructure/configs.git#k8s/app.toml?" +
//	    "ref=production&token=glpat_your_gitlab_token"
//
//	config, err := argus.LoadRemoteConfig(gitlabURL)
//	if err != nil {
//	    log.Fatal("GitLab configuration loading failed:", err)
//	}
//
// # Configuration File Formats
//
// The provider supports multiple configuration formats with automatic detection:
//
// JSON configuration:
//
//	{
//	  "service_name": "my-service",
//	  "port": 8080,
//	  "database": {
//	    "host": "db.example.com",
//	    "port": 5432,
//	    "ssl": true
//	  },
//	  "features": {
//	    "enable_metrics": true,
//	    "enable_tracing": false
//	  }
//	}
//
// YAML configuration:
//
//	service_name: my-service
//	port: 8080
//	database:
//	  host: db.example.com
//	  port: 5432
//	  ssl: true
//	features:
//	  enable_metrics: true
//	  enable_tracing: false
//
// TOML configuration:
//
//	service_name = "my-service"
//	port = 8080
//
//	[database]
//	host = "db.example.com"
//	port = 5432
//	ssl = true
//
//	[features]
//	enable_metrics = true
//	enable_tracing = false
//
// # Security Features
//
// The provider implements comprehensive security validation to protect against:
//
//   - SSRF (Server-Side Request Forgery): Blocks localhost, private networks, metadata servers
//   - Path Traversal Attacks: Validates file paths and prevents directory traversal
//   - Git URL Injection: Strict URL parsing and validation
//   - SSH Key Security: Validates SSH key file permissions (0600 or stricter)
//   - Repository Size Limits: Prevents DoS through large repository cloning
//   - Authentication Token Security: Secure handling of credentials and tokens
//
// Security best practices implemented:
//   - Input validation with URL decoding to prevent encoding bypasses
//   - Whitelist-based host validation (blocks 127.0.0.1, 10.x.x.x, 192.168.x.x, etc.)
//   - File path sanitization to prevent access to sensitive files
//   - Timeout controls to prevent resource exhaustion
//   - Memory usage limits for large files
//
// # Performance Characteristics
//
// The provider is optimized for high-performance production environments:
//
//   - Shallow Clones: Only fetches necessary commits for minimal network transfer
//   - Connection Reuse: HTTP/SSH connection pooling for reduced overhead
//   - Efficient Polling: Smart polling with exponential backoff for watch operations
//   - Memory Management: Careful resource cleanup prevents memory leaks
//   - Concurrent Safety: Thread-safe operations using proper synchronization
//   - Caching Strategy: Authentication object caching and repository metadata caching
//
// Benchmarks show minimal overhead for configuration loading and efficient polling,
// making it suitable for latency-sensitive and high-throughput applications.
//
// # Authentication Methods
//
// The provider supports multiple authentication methods for different Git platforms:
//
// GitHub Personal Access Token:
//
//	git://github.com/user/repo.git#config.json?token=ghp_xxxxxxxxxxxx
//
// GitLab Personal Access Token:
//
//	git://gitlab.com/user/repo.git#config.json?token=glpat_xxxxxxxxxxxx
//
// SSH Key Authentication:
//
//	git://github.com/user/repo.git#config.json?ssh_key=/path/to/private/key
//
// HTTP Basic Authentication:
//
//	git://username:password@git.example.com/user/repo.git#config.json
//
// # GitOps Integration
//
// The provider enables full GitOps workflows by treating Git repositories as the
// single source of truth for configuration:
//
//   - Version Control: All configuration changes are tracked in Git history
//   - Branch-based Environments: Use branches for dev/staging/production configs
//   - Tag-based Releases: Pin configurations to specific Git tags
//   - Pull Request Workflow: Review configuration changes through Git workflows
//   - Rollback Support: Easy rollback to previous configurations using Git references
//   - Audit Trail: Complete audit trail through Git commit history
//
// # Error Handling and Resilience
//
// The provider implements comprehensive error handling with structured error types:
//
//   - ARGUS_INVALID_CONFIG: Malformed Git URLs or invalid parameters
//   - ARGUS_CONFIG_NOT_FOUND: Configuration file not found in repository
//   - ARGUS_IO_ERROR: File reading or Git operation errors
//   - ARGUS_AUTH_ERROR: Authentication failures (SSH keys, tokens, credentials)
//   - ARGUS_SECURITY_ERROR: Security validation failures (SSRF, path traversal)
//   - ARGUS_RETRY_EXHAUSTED: All retry attempts failed for Git operations
//
// Watch operations include intelligent retry mechanisms with exponential backoff
// and jitter, ensuring resilient behavior in unstable network conditions.
//
// # Testing Support
//
// The provider includes comprehensive testing capabilities:
//   - Unit tests with 100% coverage of security validations
//   - Integration tests using real Git repositories
//   - Fuzz testing for security vulnerability discovery
//   - SSH authentication testing with permission validation
//   - Performance benchmarks and memory usage tests
//   - Concurrent operation testing for race condition detection
//
// Example testing setup:
//
//	func TestGitConfiguration(t *testing.T) {
//	    provider := &GitProvider{}
//
//	    // Test with real repository
//	    configURL := "git://github.com/agilira/test-configs.git#test.json?ref=main"
//	    config, err := provider.Load(context.Background(), configURL)
//
//	    assert.NoError(t, err)
//	    assert.NotNil(t, config)
//	}
//
// # Fuzz Testing and Security Validation
//
// The provider includes professional fuzz testing capabilities to discover security
// vulnerabilities and edge cases:
//
//   - URL Validation Fuzzing: Tests Git URL parsing against malicious inputs
//   - Host Validation Fuzzing: Tests SSRF protection against bypass attempts
//   - Path Traversal Fuzzing: Tests file path validation against directory traversal
//   - Authentication Fuzzing: Tests credential handling and SSH key validation
//   - Parser Fuzzing: Tests configuration file parsing against malformed content
//
// Fuzz tests can be executed with:
//
//	make fuzz  # Runs comprehensive fuzz test suite
//
// # Architecture and Design Patterns
//
// The implementation follows Go best practices and design patterns:
//
//   - Interface Implementation: Clean implementation of Argus RemoteConfigProvider
//   - Dependency Injection: No circular dependencies with Argus core
//   - Factory Pattern: GetProvider() function for clean instantiation
//   - Resource Management: Proper cleanup and lifecycle management
//   - Error Wrapping: Structured error handling with context preservation
//   - Thread Safety: Proper synchronization for concurrent operations
//   - Configuration Caching: Multi-layer caching for performance optimization
//
// The provider is designed as a standalone library that integrates seamlessly
// with Argus while maintaining independence for testing and development.
//
// # Compatibility and Support
//
// System Requirements:
//   - Go 1.25+ (utilizes latest performance and security features)
//   - Git 2.25+ (requires modern Git features for efficient operations)
//   - Linux/macOS/Windows (full cross-platform compatibility)
//   - Network access to Git repositories (GitHub, GitLab, self-hosted)
//
// Git Platform Support:
//   - GitHub.com and GitHub Enterprise
//   - GitLab.com and self-hosted GitLab
//   - Bitbucket Cloud and Server
//   - Self-hosted Git servers (Gitea, Forgejo, etc.)
//   - Any Git-compatible repository hosting
//
// # Production Deployment Considerations
//
// For production deployments, consider:
//
//   - Authentication Security: Use personal access tokens or SSH keys, never passwords
//   - Network Security: Ensure Git repositories are accessible from application environment
//   - Rate Limiting: Be aware of Git provider rate limits for API calls
//   - Caching Strategy: Configure appropriate polling intervals to balance freshness and performance
//   - Error Monitoring: Implement monitoring for Git authentication and network errors
//   - Repository Access: Use read-only deploy keys or tokens with minimal required permissions
//   - Backup Strategy: Ensure Git repositories have proper backup and disaster recovery
//   - Security Hardening: Regularly rotate access tokens and SSH keys
//
// # License and Contribution
//
// This package is licensed under the Mozilla Public License 2.0 (MPL-2.0).
// For contribution guidelines, bug reports, and feature requests, visit:
// https://github.com/agilira/argus-provider-git
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: AGILira System Libraries
// SPDX-License-Identifier: MPL-2.0
package git
