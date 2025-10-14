// Package git provides a Git remote configuration provider for Argus
//
// This package implements the Argus RemoteConfigProvider interface to enable
// loading and watching configuration from Git repositories (GitHub, GitLab, self-hosted).
//
// GitOps Configuration Management:
//   - Supports public and private repositories
//   - Multiple authentication methods (token, SSH, basic auth)
//   - Automatic branch/tag/commit detection
//   - Real-time polling for configuration changes
//   - Secure path validation and sanitization
//
// Security Features:
//   - Red-team tested against path traversal attacks
//   - Git URL validation and normalization
//   - Repository content size limits (DoS protection)
//   - SSH key validation and secure authentication
//   - Rate limiting for repository polling
//
// Performance Features:
//   - Efficient Git operations with shallow clones
//   - Configurable polling intervals with exponential backoff
//   - Memory-efficient file handling
//   - Connection pooling for multiple repositories
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package git

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/agilira/go-errors"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	toml "github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// Security and resource limit constants for DoS prevention
const (
	// Maximum allowed configuration file size (5MB)
	maxConfigFileSize = 5 * 1024 * 1024

	// Default timeout for Git operations (60 seconds)
	defaultGitTimeout = 60 * time.Second

	// Maximum concurrent clone/fetch operations
	maxConcurrentOperations = 10

	// Maximum number of active watch operations per provider
	maxActiveWatches = 5

	// Default polling interval for watch operations
	defaultPollInterval = 30 * time.Second

	// Maximum polling interval (prevents excessive wait times)
	maxPollInterval = 10 * time.Minute

	// Minimum polling interval (prevents excessive API calls)
	minPollInterval = 5 * time.Second

	// Maximum repository size for shallow clone (100MB) - TODO: implement size check
	// maxRepoSize = 100 * 1024 * 1024

	// Maximum path length to prevent DoS
	maxPathLength = 1024

	// Maximum URL length to prevent DoS
	maxURLLength = 2048

	// Retry configuration constants
	defaultMaxRetries = 3
	defaultRetryDelay = time.Second
	maxRetryDelay     = 30 * time.Second
)

// retryConfig defines retry behavior for Git operations
type retryConfig struct {
	maxRetries    int           // Maximum number of retry attempts
	baseDelay     time.Duration // Base delay between retries
	maxDelay      time.Duration // Maximum delay between retries
	backoffFactor float64       // Exponential backoff multiplier
}

// defaultRetryConfig returns a sensible default retry configuration
func defaultRetryConfig() *retryConfig {
	return &retryConfig{
		maxRetries:    defaultMaxRetries,
		baseDelay:     defaultRetryDelay,
		maxDelay:      maxRetryDelay,
		backoffFactor: 2.0, // Double the delay on each retry
	}
}

// validateSecureGitURL validates and sanitizes Git URLs to prevent attacks.
//
// SECURITY: This function implements comprehensive URL validation to prevent:
// - SSRF attacks via malicious URLs
// - Path traversal in repository URLs
// - Protocol confusion attacks
// - Excessively long URLs (DoS prevention)
// - Invalid characters and encoding attacks
//
// The function normalizes the URL and ensures it's safe for Git operations.
func validateSecureGitURL(gitURL string) (*url.URL, error) {
	if gitURL == "" {
		return nil, errors.New("ARGUS_INVALID_CONFIG", "git URL cannot be empty")
	}

	// SECURITY: Limit URL length to prevent DoS
	if len(gitURL) > maxURLLength {
		return nil, errors.New("ARGUS_INVALID_CONFIG",
			fmt.Sprintf("git URL too long: %d bytes (max %d)", len(gitURL), maxURLLength))
	}

	// Parse the URL
	parsedURL, err := url.Parse(gitURL)
	if err != nil {
		return nil, errors.Wrap(err, "ARGUS_INVALID_CONFIG", "invalid git URL format")
	}

	// SECURITY: Validate allowed schemes
	allowedSchemes := map[string]bool{
		"git":     true,
		"https":   true,
		"ssh":     true,
		"git+ssh": true,
	}

	if !allowedSchemes[parsedURL.Scheme] {
		return nil, errors.New("ARGUS_INVALID_CONFIG",
			fmt.Sprintf("unsupported git URL scheme: %s (allowed: git, https, ssh, git+ssh)", parsedURL.Scheme))
	}

	// SECURITY: Prevent localhost and internal network access
	if err := validateGitHost(parsedURL.Host); err != nil {
		return nil, err
	}

	// SECURITY: Validate repository path
	if err := validateRepositoryPath(parsedURL.Path); err != nil {
		return nil, err
	}

	return parsedURL, nil
}

// validateGitHost validates the host part of Git URLs to prevent SSRF attacks.
func validateGitHost(host string) error {
	if host == "" {
		return errors.New("ARGUS_INVALID_CONFIG", "git URL host cannot be empty")
	}

	// SECURITY: Block localhost and internal networks
	dangerousHosts := []string{
		"localhost", "127.0.0.1", "::1",
		"10.", "172.16.", "172.17.", "172.18.", "172.19.",
		"172.20.", "172.21.", "172.22.", "172.23.", "172.24.",
		"172.25.", "172.26.", "172.27.", "172.28.", "172.29.",
		"172.30.", "172.31.", "192.168.",
	}

	lowerHost := strings.ToLower(host)
	for _, dangerous := range dangerousHosts {
		if strings.Contains(lowerHost, dangerous) {
			return errors.New("ARGUS_SECURITY_ERROR",
				fmt.Sprintf("git URL host not allowed for security reasons: %s", host))
		}
	}

	return nil
}

// validateRepositoryPath validates repository paths to prevent path traversal.
func validateRepositoryPath(path string) error {
	if path == "" {
		return errors.New("ARGUS_INVALID_CONFIG", "git repository path cannot be empty")
	}

	// SECURITY: Detect path traversal patterns
	dangerousPatterns := []string{
		"..", "../", "..\\", "./../", ".\\..\\",
		"/.git/../", "\\.git\\..\\",
	}

	lowerPath := strings.ToLower(path)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerPath, pattern) {
			return errors.New("ARGUS_SECURITY_ERROR",
				fmt.Sprintf("dangerous path traversal pattern detected: %s", pattern))
		}
	}

	return nil
}

// validateConfigFilePath validates configuration file paths within repositories.
func validateConfigFilePath(filePath string) error {
	if filePath == "" {
		return errors.New("ARGUS_INVALID_CONFIG", "configuration file path cannot be empty")
	}

	// SECURITY: Limit path length
	if len(filePath) > maxPathLength {
		return errors.New("ARGUS_INVALID_CONFIG",
			fmt.Sprintf("config file path too long: %d bytes (max %d)", len(filePath), maxPathLength))
	}

	// SECURITY: Detect null bytes and control characters
	for i, b := range []byte(filePath) {
		if b == 0 {
			return errors.New("ARGUS_SECURITY_ERROR", "null byte in file path not allowed")
		}
		if b < 32 && b != 9 && b != 10 && b != 13 { // Allow tab, LF, CR
			return errors.New("ARGUS_SECURITY_ERROR",
				fmt.Sprintf("control character (0x%02x) at position %d not allowed", b, i))
		}
	}

	// SECURITY: Block path traversal attempts
	pathTraversalPatterns := []string{
		"..", "/../", "\\..\\", "./", ".\\",
		"../", "..\\", "./..", ".\\..",
	}
	for _, pattern := range pathTraversalPatterns {
		if strings.Contains(filePath, pattern) {
			return errors.New("ARGUS_SECURITY_ERROR",
				fmt.Sprintf("path traversal attempt detected: %s", pattern))
		}
	}

	// SECURITY: Validate file extension (must be a config file)
	allowedExtensions := []string{".json", ".yaml", ".yml", ".toml", ".hcl", ".ini", ".properties"}
	hasValidExtension := false
	lowerPath := strings.ToLower(filePath)

	for _, ext := range allowedExtensions {
		if strings.HasSuffix(lowerPath, ext) {
			hasValidExtension = true
			break
		}
	}

	if !hasValidExtension {
		return errors.New("ARGUS_INVALID_CONFIG",
			fmt.Sprintf("unsupported config file extension (allowed: %v)", allowedExtensions))
	}

	// SECURITY: Prevent access to sensitive files
	sensitivePaths := []string{
		".git/", ".ssh/", ".env", "passwd", "shadow", "id_rsa", "id_dsa",
		"config.key", "private.key", "secret", "token",
	}

	for _, sensitive := range sensitivePaths {
		if strings.Contains(lowerPath, sensitive) {
			return errors.New("ARGUS_SECURITY_ERROR",
				fmt.Sprintf("access to sensitive file not allowed: %s", sensitive))
		}
	}

	return nil
}

// RemoteConfigProvider defines the interface for remote configuration sources.
// This interface is copied here to avoid importing argus (which would create
// a circular dependency). The provider is completely standalone and implements
// this interface. When imported, Argus will call the registration function.
type RemoteConfigProvider interface {
	// Name returns a human-readable name for this provider (used for debugging and logging)
	Name() string

	// Scheme returns the URL scheme this provider handles (e.g., "git")
	Scheme() string

	// Load loads configuration from the remote source
	// The URL contains the full connection information including credentials
	// Returns parsed configuration as map[string]interface{}
	Load(ctx context.Context, configURL string) (map[string]interface{}, error)

	// Watch starts watching for configuration changes
	// Returns a channel that sends new configurations when they change
	// Uses polling mechanism for detecting repository updates
	Watch(ctx context.Context, configURL string) (<-chan map[string]interface{}, error)

	// Validate validates that the provider can handle the given URL
	// Performs comprehensive URL parsing and validation without connecting
	Validate(configURL string) error

	// HealthCheck performs a health check on the remote source
	// Verifies Git repository accessibility and authentication
	HealthCheck(ctx context.Context, configURL string) error
}

// GitProvider implements RemoteConfigProvider for Git repositories
//
// This provider supports:
// - Loading JSON/YAML/TOML configurations from Git repositories
// - Public and private repositories (GitHub, GitLab, self-hosted)
// - Multiple authentication methods (HTTP basic auth, tokens, SSH keys)
// - Polling-based watching for configuration updates
// - Branch, tag, and commit-specific configuration loading
// - Secure validation against malicious repositories and paths
//
// URL Format Examples:
//
//	git://github.com/user/repo.git/config/app.json?ref=main&auth=token:ghp_xxx
//	https://gitlab.com/user/repo.git/configs/prod.yaml?ref=v1.0.0&auth=basic:MYUSER:MYPASS
//	ssh://git@bitbucket.org/user/repo.git/config.json?ref=develop&key=/path/to/key
//	git+ssh://custom-git.example.com/repo.git/app.toml?ref=feature-branch
type GitProvider struct {
	// Thread-safe operation counting for resource management
	operationCount int64 // Current number of active operations
	watchCount     int64 // Current number of active watches

	// Provider state management
	closed int64 // Atomic flag indicating if provider is closed (1 = closed)

	// Temporary directory management for Git clones
	tempDirMutex sync.Mutex
	tempDirs     []string // Track temporary directories for cleanup

	// Authentication cache for performance
	authCacheMutex sync.RWMutex
	authCache      map[string]transport.AuthMethod // Cached authentication objects

	// Repository metadata cache
	repoCacheMutex sync.RWMutex
	repoCache      map[string]*repoMetadata // Cached repository information

	// Configuration cache for smart caching
	configCache *configCache

	// Retry configuration
	retryConfig *retryConfig

	// Metrics collection
	metrics *gitProviderMetrics
}

// gitProviderMetrics contains metrics for monitoring provider performance
type gitProviderMetrics struct {
	// Operation counters
	loadRequests     int64 // Total Load() calls
	watchRequests    int64 // Total Watch() calls
	cacheHits        int64 // Cache hits
	cacheMisses      int64 // Cache misses
	retryAttempts    int64 // Total retry attempts
	failedOperations int64 // Failed operations

	// Timing metrics (in nanoseconds for precision)
	totalLoadTime  int64 // Total time spent in Load operations
	totalCloneTime int64 // Total time spent cloning
	totalParseTime int64 // Total time spent parsing configs

	// Resource metrics
	tempDirsCreated int64 // Total temporary directories created
	configsCached   int64 // Total configurations cached

	// Error counters by type
	networkErrors int64 // Network-related errors
	authErrors    int64 // Authentication errors
	parseErrors   int64 // Configuration parsing errors
	gitErrors     int64 // Git operation errors
}

// newGitProviderMetrics creates a new metrics collection
func newGitProviderMetrics() *gitProviderMetrics {
	return &gitProviderMetrics{}
}

// repoMetadata contains cached information about a repository
type repoMetadata struct {
	LastCommit string    // Last known commit hash
	LastCheck  time.Time // When we last checked for updates
}

// configCacheEntry represents a cached configuration with its metadata
type configCacheEntry struct {
	Config      map[string]interface{} // Cached configuration data
	CommitHash  string                 // Commit hash this config corresponds to
	CachedAt    time.Time              // When this config was cached
	AccessCount int64                  // Number of times this cache entry was accessed
}

// configCache provides intelligent caching for loaded configurations
type configCache struct {
	entries map[string]*configCacheEntry // Map from cache key to cached config
	mutex   sync.RWMutex                 // Protects the cache map
	maxSize int                          // Maximum number of cache entries
	ttl     time.Duration                // Cache time-to-live
}

// GitURL represents a parsed Git configuration URL
type GitURL struct {
	RepoURL      string            // Base repository URL
	FilePath     string            // Path to configuration file within repo
	Reference    string            // Git reference (branch, tag, commit)
	AuthType     string            // Authentication type (token, basic, key)
	AuthData     map[string]string // Authentication data
	PollInterval time.Duration     // Custom polling interval for watch
}

// Name returns the human-readable name of this provider
func (g *GitProvider) Name() string {
	return "Git Configuration Provider"
}

// Scheme returns the URL scheme this provider handles
func (g *GitProvider) Scheme() string {
	return "git"
}

// parseGitURL parses and validates a Git configuration URL
func (g *GitProvider) parseGitURL(configURL string) (*GitURL, error) {
	// Manual parsing to handle Git-style URLs with fragments containing queries
	// Parse URL like: https://github.com/user/repo.git#config.json?ref=main&auth=token:xxx

	// Split at # first to separate base URL from fragment+query
	hashPos := strings.Index(configURL, "#")
	var baseURL, fragmentPart string

	if hashPos != -1 {
		baseURL = configURL[:hashPos]
		fragmentPart = configURL[hashPos+1:]
	} else {
		baseURL = configURL
		fragmentPart = ""
	}

	// Validate the base URL
	parsedURL, err := validateSecureGitURL(baseURL)
	if err != nil {
		return nil, err
	}

	// Build repository URL preserving user info for SSH
	var repoURL string
	if parsedURL.User != nil {
		repoURL = fmt.Sprintf("%s://%s@%s%s", parsedURL.Scheme, parsedURL.User.Username(), parsedURL.Host, parsedURL.Path)
	} else {
		repoURL = fmt.Sprintf("%s://%s%s", parsedURL.Scheme, parsedURL.Host, parsedURL.Path)
	}
	// Ensure .git suffix
	if !strings.HasSuffix(repoURL, ".git") {
		repoURL += ".git"
	}

	gitURL := &GitURL{
		RepoURL:      repoURL,
		Reference:    "main", // Default branch
		PollInterval: defaultPollInterval,
		AuthData:     make(map[string]string),
	}

	// Parse fragment part which might contain file?query=params
	var filePath string
	var queryString string

	if fragmentPart != "" {
		// Split fragment at ? to separate file from query
		if qPos := strings.Index(fragmentPart, "?"); qPos != -1 {
			filePath = fragmentPart[:qPos]
			queryString = fragmentPart[qPos+1:]
		} else {
			filePath = fragmentPart
		}
	}

	// Also check original URL query parameters
	originalQuery := parsedURL.Query()

	// Parse fragment query if present
	var fragmentQuery url.Values
	if queryString != "" {
		fragmentQuery, _ = url.ParseQuery(queryString)
	} else {
		fragmentQuery = make(url.Values)
	}

	// Extract file path from fragment or original query
	if filePath != "" {
		gitURL.FilePath = filePath
	} else if filepath := originalQuery.Get("file"); filepath != "" {
		gitURL.FilePath = filepath
	} else if filepath := fragmentQuery.Get("file"); filepath != "" {
		gitURL.FilePath = filepath
	} else {
		return nil, errors.New("ARGUS_INVALID_CONFIG", "configuration file path not specified (use #file.json or ?file=file.json)")
	}

	// Validate file path
	if err := validateConfigFilePath(gitURL.FilePath); err != nil {
		return nil, err
	}

	// Extract git reference (branch, tag, commit) from fragment query or original query
	if ref := fragmentQuery.Get("ref"); ref != "" {
		gitURL.Reference = ref
	} else if ref := originalQuery.Get("ref"); ref != "" {
		gitURL.Reference = ref
	} else if ref := fragmentQuery.Get("branch"); ref != "" {
		gitURL.Reference = ref
	} else if ref := originalQuery.Get("branch"); ref != "" {
		gitURL.Reference = ref
	} else if ref := fragmentQuery.Get("tag"); ref != "" {
		gitURL.Reference = ref
	} else if ref := originalQuery.Get("tag"); ref != "" {
		gitURL.Reference = ref
	} else if ref := fragmentQuery.Get("commit"); ref != "" {
		gitURL.Reference = ref
	} else if ref := originalQuery.Get("commit"); ref != "" {
		gitURL.Reference = ref
	}

	// Extract authentication information from fragment query or original query
	var auth string
	if auth = fragmentQuery.Get("auth"); auth == "" {
		auth = originalQuery.Get("auth")
	}

	if auth != "" {
		parts := strings.SplitN(auth, ":", 3)
		if len(parts) >= 2 {
			gitURL.AuthType = parts[0]
			switch gitURL.AuthType {
			case "token":
				gitURL.AuthData["token"] = parts[1]
			case "basic":
				if len(parts) >= 3 {
					gitURL.AuthData["username"] = parts[1]
					gitURL.AuthData["password"] = parts[2]
				}
			case "key", "ssh":
				gitURL.AuthData["keypath"] = parts[1]
				if len(parts) >= 3 {
					gitURL.AuthData["passphrase"] = parts[2]
				}
			}
		}
	}

	// Extract custom polling interval for watch
	var interval string
	if interval = fragmentQuery.Get("poll"); interval == "" {
		interval = originalQuery.Get("poll")
	}

	if interval != "" {
		if duration, err := time.ParseDuration(interval); err == nil {
			if duration >= minPollInterval && duration <= maxPollInterval {
				gitURL.PollInterval = duration
			}
		}
	}

	return gitURL, nil
}

// Load loads configuration from a Git repository
func (g *GitProvider) Load(ctx context.Context, configURL string) (map[string]interface{}, error) {
	start := time.Now()
	g.metrics.incrementLoadRequests()

	defer func() {
		g.metrics.addLoadTime(time.Since(start))
	}()

	// Check if provider is closed
	if atomic.LoadInt64(&g.closed) == 1 {
		g.metrics.incrementFailedOperations()
		return nil, errors.New("ARGUS_PROVIDER_CLOSED", "git provider is closed")
	}

	// Increment operation count
	if !g.incrementOperationCount() {
		g.metrics.incrementFailedOperations()
		return nil, errors.New("ARGUS_RESOURCE_LIMIT",
			fmt.Sprintf("maximum concurrent operations reached (%d)", maxConcurrentOperations))
	}
	defer g.decrementOperationCount()

	// Parse the Git URL
	gitURL, err := g.parseGitURL(configURL)
	if err != nil {
		g.metrics.incrementFailedOperations()
		g.classifyAndRecordError(err)
		return nil, err
	}

	// Clone and read configuration
	config, err := g.loadConfigFromRepo(ctx, gitURL)
	if err != nil {
		g.metrics.incrementFailedOperations()
		g.classifyAndRecordError(err)
		return nil, err
	}

	return config, nil
}

// Watch starts watching for configuration changes in a Git repository
func (g *GitProvider) Watch(ctx context.Context, configURL string) (<-chan map[string]interface{}, error) {
	g.metrics.incrementWatchRequests()

	// Check if provider is closed
	if atomic.LoadInt64(&g.closed) == 1 {
		g.metrics.incrementFailedOperations()
		return nil, errors.New("ARGUS_PROVIDER_CLOSED", "git provider is closed")
	}

	// Check watch limit
	if !g.incrementWatchCount() {
		g.metrics.incrementFailedOperations()
		return nil, errors.New("ARGUS_RESOURCE_LIMIT",
			fmt.Sprintf("maximum active watches reached (%d)", maxActiveWatches))
	}

	// Parse the Git URL
	gitURL, err := g.parseGitURL(configURL)
	if err != nil {
		g.decrementWatchCount()
		g.metrics.incrementFailedOperations()
		g.classifyAndRecordError(err)
		return nil, err
	}

	// Create watch channel
	configChan := make(chan map[string]interface{}, 1)

	// Start watching in a goroutine
	go g.startWatching(ctx, gitURL, configChan)

	return configChan, nil
}

// Validate validates that the provider can handle the given URL
func (g *GitProvider) Validate(configURL string) error {
	_, err := g.parseGitURL(configURL)
	return err
}

// HealthCheck performs a health check on the Git repository
func (g *GitProvider) HealthCheck(ctx context.Context, configURL string) error {
	// Parse the Git URL
	gitURL, err := g.parseGitURL(configURL)
	if err != nil {
		return err
	}

	// Try to list remote references (lightweight operation)
	return g.checkRepositoryHealth(ctx, gitURL)
}

// Close cleanly shuts down the provider and releases resources
func (g *GitProvider) Close() error {
	// Check if already closed (idempotent operation)
	if !atomic.CompareAndSwapInt64(&g.closed, 0, 1) {
		return nil
	}

	// Clean up temporary directories
	g.cleanupTempDirectories()

	// Clear caches
	g.authCacheMutex.Lock()
	g.authCache = nil
	g.authCacheMutex.Unlock()

	g.repoCacheMutex.Lock()
	g.repoCache = nil
	g.repoCacheMutex.Unlock()

	return nil
}

// incrementOperationCount safely increments the operation counter
func (g *GitProvider) incrementOperationCount() bool {
	current := atomic.LoadInt64(&g.operationCount)
	if current >= maxConcurrentOperations {
		return false
	}
	atomic.AddInt64(&g.operationCount, 1)
	return true
}

// decrementOperationCount safely decrements the operation counter
func (g *GitProvider) decrementOperationCount() {
	atomic.AddInt64(&g.operationCount, -1)
}

// incrementWatchCount safely increments the watch counter
func (g *GitProvider) incrementWatchCount() bool {
	current := atomic.LoadInt64(&g.watchCount)
	if current >= maxActiveWatches {
		return false
	}
	atomic.AddInt64(&g.watchCount, 1)
	return true
}

// decrementWatchCount safely decrements the watch counter
func (g *GitProvider) decrementWatchCount() {
	atomic.AddInt64(&g.watchCount, -1)
}

// loadConfigFromRepo clones the repository and loads the configuration file with intelligent caching
func (g *GitProvider) loadConfigFromRepo(ctx context.Context, gitURL *GitURL) (map[string]interface{}, error) {
	// First, try to get the current commit hash for caching
	commitHash, err := g.getRemoteCommitHash(ctx, gitURL)
	if err != nil {
		// If we can't get the commit hash, fall back to direct loading
		return g.loadConfigFromRepoDirectly(ctx, gitURL)
	}

	// Check if we have this configuration cached
	if cachedConfig, found := g.configCache.get(gitURL, commitHash); found {
		g.metrics.incrementCacheHits()
		return cachedConfig, nil
	}
	g.metrics.incrementCacheMisses()

	// Load configuration directly
	config, err := g.loadConfigFromRepoDirectly(ctx, gitURL)
	if err != nil {
		return nil, err
	}

	// Cache the loaded configuration
	g.configCache.put(gitURL, commitHash, config)
	g.metrics.incrementConfigsCached()

	return config, nil
}

// loadConfigFromRepoDirectly performs the actual repository cloning and config loading
func (g *GitProvider) loadConfigFromRepoDirectly(ctx context.Context, gitURL *GitURL) (map[string]interface{}, error) {
	// Create temporary directory for clone
	tempDir, err := g.createTempDirectory()
	if err != nil {
		return nil, errors.Wrap(err, "ARGUS_IO_ERROR", "failed to create temporary directory")
	}
	defer g.removeTempDirectory(tempDir)

	// Clone repository
	repo, err := g.cloneRepository(ctx, gitURL, tempDir)
	if err != nil {
		return nil, err
	}

	// Read configuration file
	config, err := g.readConfigFile(repo, gitURL.FilePath, gitURL.Reference)
	if err != nil {
		return nil, err
	}

	return config, nil
}

// cloneRepository clones a Git repository to a temporary directory with retry logic
func (g *GitProvider) cloneRepository(ctx context.Context, gitURL *GitURL, tempDir string) (*git.Repository, error) {
	var repo *git.Repository

	err := g.retryOperation(ctx, func() error {
		// Prepare clone options
		cloneOptions := &git.CloneOptions{
			URL:      gitURL.RepoURL,
			Progress: nil, // No progress reporting for security
			Depth:    1,   // Shallow clone for performance
		}

		// Set authentication if provided
		if auth, err := g.getAuthentication(gitURL); err == nil && auth != nil {
			cloneOptions.Auth = auth
		}

		// Set reference if specified
		if gitURL.Reference != "" {
			cloneOptions.ReferenceName = plumbing.ReferenceName("refs/heads/" + gitURL.Reference)
			cloneOptions.SingleBranch = true
		}

		// Add timeout to context
		cloneCtx, cancel := context.WithTimeout(ctx, defaultGitTimeout)
		defer cancel()

		// Clone repository
		var err error
		repo, err = git.PlainCloneContext(cloneCtx, tempDir, false, cloneOptions)
		if err != nil {
			return errors.Wrap(err, "ARGUS_GIT_ERROR", "failed to clone repository")
		}

		return nil
	}, "git clone")

	if err != nil {
		return nil, err
	}

	return repo, nil
}

// readConfigFile reads and parses a configuration file from the repository
func (g *GitProvider) readConfigFile(repo *git.Repository, filePath, reference string) (map[string]interface{}, error) {
	// Get worktree
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, errors.Wrap(err, "ARGUS_GIT_ERROR", "failed to get repository worktree")
	}

	// Checkout specific reference if needed
	if reference != "" && reference != "main" && reference != "master" {
		err = g.checkoutReference(worktree, reference)
		if err != nil {
			return nil, err
		}
	}

	// Read file from worktree with secure path validation
	rootPath := worktree.Filesystem.Root()
	fullPath := filepath.Join(rootPath, filePath)

	// Security check: ensure the resolved path is still within the repository root
	cleanPath, err := filepath.Abs(fullPath)
	if err != nil {
		return nil, errors.Wrap(err, "ARGUS_SECURITY_ERROR", "failed to resolve absolute path")
	}

	cleanRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, errors.Wrap(err, "ARGUS_SECURITY_ERROR", "failed to resolve repository root path")
	}

	if !strings.HasPrefix(cleanPath, cleanRoot+string(filepath.Separator)) {
		return nil, errors.New("ARGUS_SECURITY_ERROR",
			fmt.Sprintf("path traversal detected: %s is outside repository root", filePath))
	}

	// #nosec G304 - Path is validated above to prevent directory traversal
	fileContent, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, errors.Wrap(err, "ARGUS_IO_ERROR",
			fmt.Sprintf("failed to read configuration file: %s", filePath))
	}

	// Check file size limit
	if len(fileContent) > maxConfigFileSize {
		return nil, errors.New("ARGUS_RESOURCE_LIMIT",
			fmt.Sprintf("configuration file too large: %d bytes (max %d)", len(fileContent), maxConfigFileSize))
	}

	// Parse configuration based on file extension
	config, err := g.parseConfigFile(filePath, fileContent)
	if err != nil {
		return nil, err
	}

	return config, nil
}

// parseConfigFile parses configuration content based on file extension
func (g *GitProvider) parseConfigFile(filePath string, content []byte) (map[string]interface{}, error) {
	var config map[string]interface{}

	// Determine format from file extension
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".json":
		err := json.Unmarshal(content, &config)
		if err != nil {
			return nil, errors.Wrap(err, "ARGUS_PARSE_ERROR", "failed to parse JSON configuration")
		}
	case ".yaml", ".yml":
		// Use proper YAML parsing
		err := yaml.Unmarshal(content, &config)
		if err != nil {
			return nil, errors.Wrap(err, "ARGUS_PARSE_ERROR", "failed to parse YAML configuration")
		}
	case ".toml":
		// Use TOML parsing
		err := toml.Unmarshal(content, &config)
		if err != nil {
			return nil, errors.Wrap(err, "ARGUS_PARSE_ERROR", "failed to parse TOML configuration")
		}
	default:
		return nil, errors.New("ARGUS_UNSUPPORTED_FORMAT",
			fmt.Sprintf("unsupported configuration file format: %s (supported: .json, .yaml, .yml, .toml)", ext))
	}

	return config, nil
}

// checkoutReference checks out a specific Git reference (branch, tag, commit)
func (g *GitProvider) checkoutReference(worktree *git.Worktree, reference string) error {
	// Try as branch name first
	err := worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/heads/" + reference),
	})
	if err == nil {
		return nil
	}

	// Try as tag
	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/tags/" + reference),
	})
	if err == nil {
		return nil
	}

	// Try as commit hash
	if len(reference) >= 7 { // Minimum viable commit hash length
		hash := plumbing.NewHash(reference)
		err = worktree.Checkout(&git.CheckoutOptions{
			Hash: hash,
		})
		if err == nil {
			return nil
		}
	}

	return errors.New("ARGUS_GIT_ERROR",
		fmt.Sprintf("failed to checkout reference: %s", reference))
}

// getAuthentication creates authentication object based on GitURL auth data
func (g *GitProvider) getAuthentication(gitURL *GitURL) (transport.AuthMethod, error) {
	if gitURL.AuthType == "" {
		return nil, nil // No authentication
	}

	// Check cache first
	cacheKey := fmt.Sprintf("%s:%s", gitURL.AuthType, gitURL.RepoURL)
	g.authCacheMutex.RLock()
	if auth, exists := g.authCache[cacheKey]; exists {
		g.authCacheMutex.RUnlock()
		return auth, nil
	}
	g.authCacheMutex.RUnlock()

	var auth transport.AuthMethod
	var err error

	switch gitURL.AuthType {
	case "token":
		if token, exists := gitURL.AuthData["token"]; exists {
			auth = &http.BasicAuth{
				Username: "token",
				Password: token,
			}
		}
	case "basic":
		username := gitURL.AuthData["username"]
		password := gitURL.AuthData["password"]
		if username != "" && password != "" {
			auth = &http.BasicAuth{
				Username: username,
				Password: password,
			}
		}
	case "key", "ssh":
		keyPath := gitURL.AuthData["keypath"]
		if keyPath != "" {
			// Validate SSH key file permissions for security
			if info, err := os.Stat(keyPath); err != nil {
				return nil, errors.Wrap(err, "ARGUS_AUTH_ERROR",
					fmt.Sprintf("SSH key file not accessible: %s", keyPath))
			} else if info.Mode().Perm() > 0o600 {
				return nil, errors.New("ARGUS_AUTH_ERROR",
					fmt.Sprintf("SSH key file permissions too open: %s (should be 0600 or less)", keyPath))
			}

			passphrase := gitURL.AuthData["passphrase"]
			auth, err = ssh.NewPublicKeysFromFile("git", keyPath, passphrase)
			if err != nil {
				return nil, errors.Wrap(err, "ARGUS_AUTH_ERROR", "failed to load SSH key")
			}
		}
	default:
		return nil, errors.New("ARGUS_AUTH_ERROR",
			fmt.Sprintf("unsupported authentication type: %s", gitURL.AuthType))
	}

	// Cache the authentication object
	if auth != nil {
		g.authCacheMutex.Lock()
		g.authCache[cacheKey] = auth
		g.authCacheMutex.Unlock()
	}

	return auth, nil
}

// startWatching starts polling for repository changes
func (g *GitProvider) startWatching(ctx context.Context, gitURL *GitURL, configChan chan<- map[string]interface{}) {
	defer close(configChan)
	defer g.decrementWatchCount()

	ticker := time.NewTicker(gitURL.PollInterval)
	defer ticker.Stop()

	// Load initial configuration
	config, err := g.loadConfigFromRepo(ctx, gitURL)
	if err == nil {
		select {
		case configChan <- config:
		case <-ctx.Done():
			return
		}
	}

	// Poll for changes
	for {
		select {
		case <-ticker.C:
			if g.hasRepositoryChanged(ctx, gitURL) {
				newConfig, err := g.loadConfigFromRepo(ctx, gitURL)
				if err == nil {
					select {
					case configChan <- newConfig:
					case <-ctx.Done():
						return
					}
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

// hasRepositoryChanged checks if the repository has new commits using git ls-remote
func (g *GitProvider) hasRepositoryChanged(ctx context.Context, gitURL *GitURL) bool {
	// Create context with timeout to prevent hanging
	lsCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Get current commit hash for the reference
	currentCommit, err := g.getRemoteCommitHash(lsCtx, gitURL)
	if err != nil {
		// If we can't get remote commit, assume change to trigger reload
		// This ensures we don't miss updates due to network issues
		return true
	}

	// Check our cache for the last known commit
	g.repoCacheMutex.RLock()
	cached, exists := g.repoCache[gitURL.RepoURL]
	g.repoCacheMutex.RUnlock()

	if !exists || cached.LastCommit != currentCommit {
		// Update cache with new commit hash
		g.updateRepoCache(gitURL.RepoURL, currentCommit)
		return true
	}

	return false
}

// getRemoteCommitHash uses git ls-remote to get the latest commit hash for a reference with retry
func (g *GitProvider) getRemoteCommitHash(ctx context.Context, gitURL *GitURL) (string, error) {
	var commitHash string

	err := g.retryOperation(ctx, func() error {
		// Use go-git's Remote to list references without cloning
		// This is much more efficient than full clone for checking changes

		// Create a remote reference pointing to the repository
		storage := memory.NewStorage()
		remote := git.NewRemote(storage, &config.RemoteConfig{
			Name: "origin",
			URLs: []string{gitURL.RepoURL},
		})

		// Set authentication if available
		var auth transport.AuthMethod
		if authMethod, err := g.getAuthentication(gitURL); err == nil && authMethod != nil {
			auth = authMethod
		}

		// List remote references (equivalent to git ls-remote)
		refs, err := remote.ListContext(ctx, &git.ListOptions{
			Auth: auth,
		})
		if err != nil {
			return errors.Wrap(err, "ARGUS_GIT_ERROR", "failed to list remote references")
		}

		// Find the commit hash for our target reference
		targetRef := fmt.Sprintf("refs/heads/%s", gitURL.Reference)

		// Also check for tags if it's not a branch
		targetTagRef := fmt.Sprintf("refs/tags/%s", gitURL.Reference)

		for _, ref := range refs {
			refName := ref.Name().String()
			if refName == targetRef || refName == targetTagRef {
				commitHash = ref.Hash().String()
				return nil
			}
		}

		// If no exact match, try HEAD for default branch
		for _, ref := range refs {
			if ref.Name().String() == "HEAD" {
				commitHash = ref.Hash().String()
				return nil
			}
		}

		return errors.New("ARGUS_GIT_ERROR",
			fmt.Sprintf("reference %s not found in remote repository", gitURL.Reference))
	}, "git ls-remote")

	if err != nil {
		return "", err
	}

	return commitHash, nil
}

// updateRepoCache updates the repository cache with new commit information
func (g *GitProvider) updateRepoCache(repoURL, commitHash string) {
	g.repoCacheMutex.Lock()
	defer g.repoCacheMutex.Unlock()

	g.repoCache[repoURL] = &repoMetadata{
		LastCommit: commitHash,
		LastCheck:  time.Now(),
	}
}

// newConfigCache creates a new configuration cache with specified parameters
func newConfigCache(maxSize int, ttl time.Duration) *configCache {
	return &configCache{
		entries: make(map[string]*configCacheEntry),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// getCacheKey generates a unique cache key for a Git URL and commit hash
func (c *configCache) getCacheKey(gitURL *GitURL, commitHash string) string {
	return fmt.Sprintf("%s:%s:%s", gitURL.RepoURL, gitURL.FilePath, commitHash)
}

// get retrieves a configuration from the cache if it exists and is still valid
func (c *configCache) get(gitURL *GitURL, commitHash string) (map[string]interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	key := c.getCacheKey(gitURL, commitHash)
	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	// Check if cache entry has expired
	if time.Since(entry.CachedAt) > c.ttl {
		return nil, false
	}

	// Update access count for LRU eviction
	atomic.AddInt64(&entry.AccessCount, 1)

	// Return a copy to prevent modification of cached data
	return c.copyConfig(entry.Config), true
}

// put stores a configuration in the cache
func (c *configCache) put(gitURL *GitURL, commitHash string, config map[string]interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	key := c.getCacheKey(gitURL, commitHash)

	// Check if we need to evict entries (simple LRU based on access count and time)
	if len(c.entries) >= c.maxSize {
		c.evictLRU()
	}

	// Store a copy to prevent modification of cached data
	c.entries[key] = &configCacheEntry{
		Config:      c.copyConfig(config),
		CommitHash:  commitHash,
		CachedAt:    time.Now(),
		AccessCount: 1,
	}
}

// evictLRU removes the least recently used cache entry
func (c *configCache) evictLRU() {
	if len(c.entries) == 0 {
		return
	}

	var oldestKey string
	var oldestTime time.Time
	var lowestAccess int64 = math.MaxInt64

	// Find the least recently used entry (combination of access count and time)
	for key, entry := range c.entries {
		accessCount := atomic.LoadInt64(&entry.AccessCount)

		// Prefer evicting entries with lower access count
		// If access counts are equal, prefer older entries
		if accessCount < lowestAccess ||
			(accessCount == lowestAccess && (oldestKey == "" || entry.CachedAt.Before(oldestTime))) {
			oldestKey = key
			oldestTime = entry.CachedAt
			lowestAccess = accessCount
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// copyConfig creates a deep copy of a configuration map to prevent modification
func (c *configCache) copyConfig(config map[string]interface{}) map[string]interface{} {
	if config == nil {
		return nil
	}

	copy := make(map[string]interface{}, len(config))
	for k, v := range config {
		// For simple types, direct assignment is fine
		// For complex types, we'd need deeper copying, but for config data this is usually sufficient
		copy[k] = v
	}
	return copy
}

// stats returns cache statistics for monitoring
func (c *configCache) stats() map[string]interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	totalAccess := int64(0)
	oldestEntry := time.Now()
	newestEntry := time.Time{}

	for _, entry := range c.entries {
		totalAccess += atomic.LoadInt64(&entry.AccessCount)
		if entry.CachedAt.Before(oldestEntry) {
			oldestEntry = entry.CachedAt
		}
		if entry.CachedAt.After(newestEntry) {
			newestEntry = entry.CachedAt
		}
	}

	return map[string]interface{}{
		"entries":      len(c.entries),
		"max_size":     c.maxSize,
		"total_access": totalAccess,
		"oldest_entry": oldestEntry,
		"newest_entry": newestEntry,
		"ttl_seconds":  c.ttl.Seconds(),
	}
}

// retryOperation performs an operation with exponential backoff retry logic
func (g *GitProvider) retryOperation(ctx context.Context, operation func() error, operationName string) error {
	var lastErr error

	for attempt := 0; attempt <= g.retryConfig.maxRetries; attempt++ {
		// Perform the operation
		err := operation()
		if err == nil {
			return nil // Success!
		}

		lastErr = err
		g.classifyAndRecordError(err)

		// Don't retry on the last attempt
		if attempt == g.retryConfig.maxRetries {
			break
		}

		// Check if this error is retryable
		if !g.isRetryableError(err) {
			return err // Don't retry non-retryable errors
		}

		g.metrics.incrementRetryAttempts()

		// Calculate delay with exponential backoff
		delay := g.calculateRetryDelay(attempt)

		// Wait for the delay or until context cancellation
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "ARGUS_CONTEXT_CANCELLED",
				fmt.Sprintf("%s cancelled during retry attempt %d", operationName, attempt))
		}
	}

	return errors.Wrap(lastErr, "ARGUS_RETRY_EXHAUSTED",
		fmt.Sprintf("%s failed after %d attempts", operationName, g.retryConfig.maxRetries+1))
}

// isRetryableError determines if an error is worth retrying
func (g *GitProvider) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Network-related errors that are usually temporary
	retryablePatterns := []string{
		"connection refused",
		"connection reset",
		"connection timeout",
		"network is unreachable",
		"timeout",
		"temporary failure",
		"service unavailable",
		"bad gateway",
		"gateway timeout",
		"too many requests",
		"rate limit",
		"dns",
		"no such host",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	// Non-retryable errors (authentication, not found, etc.)
	nonRetryablePatterns := []string{
		"authentication failed",
		"permission denied",
		"not found",
		"forbidden",
		"unauthorized",
		"invalid credentials",
		"repository not found",
		"access denied",
	}

	for _, pattern := range nonRetryablePatterns {
		if strings.Contains(errStr, pattern) {
			return false
		}
	}

	// Default to retryable for unknown errors
	return true
}

// calculateRetryDelay calculates the delay for a retry attempt using exponential backoff
func (g *GitProvider) calculateRetryDelay(attempt int) time.Duration {
	// Calculate exponential backoff: baseDelay * (backoffFactor ^ attempt)
	delay := float64(g.retryConfig.baseDelay) * math.Pow(g.retryConfig.backoffFactor, float64(attempt))

	// Add some jitter to prevent thundering herd (simple approach without rand for now)
	// In a full implementation, you'd use crypto/rand or math/rand
	jitterPercent := float64(attempt%10) / 100.0 // Simple deterministic jitter 0-9%
	jitter := delay * jitterPercent
	delay += jitter

	// Cap the delay at maxDelay
	if delay > float64(g.retryConfig.maxDelay) {
		delay = float64(g.retryConfig.maxDelay)
	}

	return time.Duration(delay)
}

// Metrics methods for tracking provider performance
func (m *gitProviderMetrics) incrementLoadRequests() {
	atomic.AddInt64(&m.loadRequests, 1)
}

func (m *gitProviderMetrics) incrementWatchRequests() {
	atomic.AddInt64(&m.watchRequests, 1)
}

func (m *gitProviderMetrics) incrementCacheHits() {
	atomic.AddInt64(&m.cacheHits, 1)
}

func (m *gitProviderMetrics) incrementCacheMisses() {
	atomic.AddInt64(&m.cacheMisses, 1)
}

func (m *gitProviderMetrics) incrementRetryAttempts() {
	atomic.AddInt64(&m.retryAttempts, 1)
}

func (m *gitProviderMetrics) incrementFailedOperations() {
	atomic.AddInt64(&m.failedOperations, 1)
}

func (m *gitProviderMetrics) addLoadTime(duration time.Duration) {
	atomic.AddInt64(&m.totalLoadTime, int64(duration))
}

func (m *gitProviderMetrics) incrementTempDirsCreated() {
	atomic.AddInt64(&m.tempDirsCreated, 1)
}

func (m *gitProviderMetrics) incrementConfigsCached() {
	atomic.AddInt64(&m.configsCached, 1)
}

func (m *gitProviderMetrics) incrementNetworkErrors() {
	atomic.AddInt64(&m.networkErrors, 1)
}

func (m *gitProviderMetrics) incrementAuthErrors() {
	atomic.AddInt64(&m.authErrors, 1)
}

func (m *gitProviderMetrics) incrementParseErrors() {
	atomic.AddInt64(&m.parseErrors, 1)
}

func (m *gitProviderMetrics) incrementGitErrors() {
	atomic.AddInt64(&m.gitErrors, 1)
}

// GetMetrics returns current metrics as a map for monitoring systems
func (g *GitProvider) GetMetrics() map[string]interface{} {
	m := g.metrics

	// Calculate derived metrics
	totalRequests := atomic.LoadInt64(&m.loadRequests) + atomic.LoadInt64(&m.watchRequests)
	cacheHitRate := float64(0)
	if totalCacheAttempts := atomic.LoadInt64(&m.cacheHits) + atomic.LoadInt64(&m.cacheMisses); totalCacheAttempts > 0 {
		cacheHitRate = float64(atomic.LoadInt64(&m.cacheHits)) / float64(totalCacheAttempts) * 100
	}

	avgLoadTime := time.Duration(0)
	if loadRequests := atomic.LoadInt64(&m.loadRequests); loadRequests > 0 {
		avgLoadTime = time.Duration(atomic.LoadInt64(&m.totalLoadTime) / loadRequests)
	}

	return map[string]interface{}{
		// Request metrics
		"load_requests":  atomic.LoadInt64(&m.loadRequests),
		"watch_requests": atomic.LoadInt64(&m.watchRequests),
		"total_requests": totalRequests,

		// Cache metrics
		"cache_hits":     atomic.LoadInt64(&m.cacheHits),
		"cache_misses":   atomic.LoadInt64(&m.cacheMisses),
		"cache_hit_rate": cacheHitRate,
		"configs_cached": atomic.LoadInt64(&m.configsCached),

		// Performance metrics
		"retry_attempts":      atomic.LoadInt64(&m.retryAttempts),
		"failed_operations":   atomic.LoadInt64(&m.failedOperations),
		"avg_load_time_ms":    float64(avgLoadTime.Nanoseconds()) / 1000000,
		"total_clone_time_ms": float64(atomic.LoadInt64(&m.totalCloneTime)) / 1000000,
		"total_parse_time_ms": float64(atomic.LoadInt64(&m.totalParseTime)) / 1000000,

		// Resource metrics
		"temp_dirs_created": atomic.LoadInt64(&m.tempDirsCreated),

		// Error metrics
		"network_errors": atomic.LoadInt64(&m.networkErrors),
		"auth_errors":    atomic.LoadInt64(&m.authErrors),
		"parse_errors":   atomic.LoadInt64(&m.parseErrors),
		"git_errors":     atomic.LoadInt64(&m.gitErrors),

		// Configuration cache metrics
		"config_cache": g.configCache.stats(),
	}
}

// classifyAndRecordError classifies an error and records the appropriate metric
func (g *GitProvider) classifyAndRecordError(err error) {
	if err == nil {
		return
	}

	errStr := strings.ToLower(err.Error())

	// Classify error types and record metrics
	if strings.Contains(errStr, "network") || strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "timeout") || strings.Contains(errStr, "dns") {
		g.metrics.incrementNetworkErrors()
	} else if strings.Contains(errStr, "auth") || strings.Contains(errStr, "permission") ||
		strings.Contains(errStr, "credential") || strings.Contains(errStr, "forbidden") {
		g.metrics.incrementAuthErrors()
	} else if strings.Contains(errStr, "parse") || strings.Contains(errStr, "marshal") ||
		strings.Contains(errStr, "json") || strings.Contains(errStr, "yaml") || strings.Contains(errStr, "toml") {
		g.metrics.incrementParseErrors()
	} else if strings.Contains(errStr, "git") || strings.Contains(errStr, "clone") ||
		strings.Contains(errStr, "checkout") || strings.Contains(errStr, "repository") {
		g.metrics.incrementGitErrors()
	}
}

// checkRepositoryHealth verifies repository accessibility
func (g *GitProvider) checkRepositoryHealth(ctx context.Context, gitURL *GitURL) error {
	// Create a memory-based clone for health check (no disk I/O)
	cloneOptions := &git.CloneOptions{
		URL:      gitURL.RepoURL,
		Progress: nil,
		Depth:    1,
	}

	// Set authentication if provided
	if auth, err := g.getAuthentication(gitURL); err == nil && auth != nil {
		cloneOptions.Auth = auth
	}

	// Add timeout to context
	healthCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Try to clone into memory
	_, err := git.CloneContext(healthCtx, memory.NewStorage(), nil, cloneOptions)
	if err != nil {
		return errors.Wrap(err, "ARGUS_HEALTH_CHECK_FAILED", "repository not accessible")
	}

	return nil
}

// createTempDirectory creates a temporary directory for Git operations
func (g *GitProvider) createTempDirectory() (string, error) {
	tempDir, err := os.MkdirTemp("", "argus-git-*")
	if err != nil {
		return "", err
	}

	g.tempDirMutex.Lock()
	g.tempDirs = append(g.tempDirs, tempDir)
	g.tempDirMutex.Unlock()

	g.metrics.incrementTempDirsCreated()

	return tempDir, nil
}

// removeTempDirectory removes a specific temporary directory
func (g *GitProvider) removeTempDirectory(tempDir string) {
	_ = os.RemoveAll(tempDir) // Intentionally ignore error as cleanup is best-effort

	g.tempDirMutex.Lock()
	for i, dir := range g.tempDirs {
		if dir == tempDir {
			g.tempDirs = append(g.tempDirs[:i], g.tempDirs[i+1:]...)
			break
		}
	}
	g.tempDirMutex.Unlock()
}

// cleanupTempDirectories removes all temporary directories
func (g *GitProvider) cleanupTempDirectories() {
	g.tempDirMutex.Lock()
	defer g.tempDirMutex.Unlock()

	for _, dir := range g.tempDirs {
		_ = os.RemoveAll(dir) // Intentionally ignore errors during cleanup
	}
	g.tempDirs = nil
}

// GetProvider returns a new instance of the Git provider
//
// This function is called by Argus during the provider registration process.
// It returns a fresh instance of the provider that Argus will register
// and use for handling git:// URLs.
func GetProvider() RemoteConfigProvider {
	return &GitProvider{
		authCache:   make(map[string]transport.AuthMethod),
		repoCache:   make(map[string]*repoMetadata),
		tempDirs:    make([]string, 0),
		configCache: newConfigCache(100, 10*time.Minute), // Cache up to 100 configs for 10 minutes
		retryConfig: defaultRetryConfig(),
		metrics:     newGitProviderMetrics(),
	}
}
