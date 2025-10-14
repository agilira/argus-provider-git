// provider_test.go
//
// Comprehensive test suite for the Git remote configuration provider
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package git

import (
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing/transport"
)

// TestGitProvider_Name verifies the provider returns the correct name
func TestGitProvider_Name(t *testing.T) {
	provider := GetProvider()
	expected := "Git Configuration Provider"
	actual := provider.Name()

	if actual != expected {
		t.Errorf("Expected name '%s', got '%s'", expected, actual)
	}
}

// TestGitProvider_Scheme verifies the provider returns the correct scheme
func TestGitProvider_Scheme(t *testing.T) {
	provider := GetProvider()
	expected := "git"
	actual := provider.Scheme()

	if actual != expected {
		t.Errorf("Expected scheme '%s', got '%s'", expected, actual)
	}
}

// TestGitProvider_Validate tests URL validation functionality
func TestGitProvider_Validate(t *testing.T) {
	provider := GetProvider()

	testCases := []struct {
		name        string
		url         string
		expectError bool
		description string
	}{
		{
			name:        "Valid HTTPS GitHub URL",
			url:         "https://github.com/user/repo.git#config.json?ref=main",
			expectError: false,
			description: "Standard GitHub HTTPS URL with file and branch",
		},
		{
			name:        "Valid SSH GitHub URL",
			url:         "ssh://git@github.com/user/repo.git#config.yaml?ref=develop",
			expectError: false,
			description: "GitHub SSH URL with YAML config",
		},
		{
			name:        "Valid GitLab URL with auth",
			url:         "https://gitlab.com/user/repo.git#configs/prod.json?ref=v1.0.0&auth=token:glpat_xxx",
			expectError: false,
			description: "GitLab with token authentication and tag",
		},
		{
			name:        "Valid self-hosted Git",
			url:         "git://git.company.com/team/configs.git#app.toml?ref=feature-branch",
			expectError: false,
			description: "Self-hosted Git server with TOML config",
		},
		{
			name:        "Valid with query file parameter",
			url:         "https://github.com/user/repo.git?file=config/app.json&ref=main",
			expectError: false,
			description: "Using file query parameter instead of fragment",
		},
		{
			name:        "Valid with SSH key auth",
			url:         "git+ssh://git@bitbucket.org/user/repo.git#config.yaml?auth=key:/path/to/key",
			expectError: false,
			description: "SSH key authentication",
		},
		{
			name:        "Valid with basic auth",
			url:         "https://gitlab.com/user/repo.git#config.json?auth=basic:testuser:testpass",
			expectError: false,
			description: "HTTP basic authentication",
		},
		{
			name:        "Valid with custom poll interval",
			url:         "https://github.com/user/repo.git#config.json?poll=60s",
			expectError: false,
			description: "Custom polling interval for watch",
		},
		// Error cases
		{
			name:        "Invalid scheme",
			url:         "http://github.com/user/repo.git#config.json",
			expectError: true,
			description: "HTTP scheme not allowed for security",
		},
		{
			name:        "Missing file path",
			url:         "https://github.com/user/repo.git",
			expectError: true,
			description: "Configuration file path not specified",
		},
		{
			name:        "Invalid file extension",
			url:         "https://github.com/user/repo.git#script.sh",
			expectError: true,
			description: "Non-configuration file extension",
		},
		{
			name:        "Empty URL",
			url:         "",
			expectError: true,
			description: "Empty URL should be rejected",
		},
		{
			name:        "Invalid URL format",
			url:         "not-a-valid-url",
			expectError: true,
			description: "Malformed URL should be rejected",
		},
		{
			name:        "Localhost security block",
			url:         "https://localhost/user/repo.git#config.json",
			expectError: true,
			description: "Localhost URLs blocked for security",
		},
		{
			name:        "Internal network security block",
			url:         "https://192.168.1.1/user/repo.git#config.json",
			expectError: true,
			description: "Internal network URLs blocked for security",
		},
		{
			name:        "Path traversal attempt",
			url:         "https://github.com/user/repo.git#../../../etc/passwd",
			expectError: true,
			description: "Path traversal should be blocked",
		},
		{
			name:        "Sensitive file access attempt",
			url:         "https://github.com/user/repo.git#.git/config",
			expectError: true,
			description: "Access to sensitive files should be blocked",
		},
		{
			name:        "Excessively long URL",
			url:         "https://github.com/user/repo.git#" + generateLongString(3000),
			expectError: true,
			description: "Excessively long URLs should be rejected",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := provider.Validate(tc.url)

			if tc.expectError && err == nil {
				t.Errorf("Expected error for %s, but got none. Description: %s", tc.url, tc.description)
			}

			if !tc.expectError && err != nil {
				t.Errorf("Expected no error for %s, but got: %v. Description: %s", tc.url, err, tc.description)
			}
		})
	}
}

// TestGitURL_Parsing tests the internal URL parsing functionality
func TestGitURL_Parsing(t *testing.T) {
	provider := &GitProvider{
		authCache: make(map[string]transport.AuthMethod),
		repoCache: make(map[string]*repoMetadata),
		tempDirs:  make([]string, 0),
	}

	testCases := []struct {
		name     string
		url      string
		expected GitURL
	}{
		{
			name: "GitHub HTTPS with token",
			url:  "https://github.com/user/repo.git#config.json?ref=main&auth=token:ghp_xxx",
			expected: GitURL{
				RepoURL:   "https://github.com/user/repo.git",
				FilePath:  "config.json",
				Reference: "main",
				AuthType:  "token",
				AuthData:  map[string]string{"token": "ghp_xxx"},
			},
		},
		{
			name: "GitLab SSH with key",
			url:  "ssh://git@gitlab.com/user/repo.git#configs/app.yaml?ref=develop&auth=key:/home/user/.ssh/id_rsa",
			expected: GitURL{
				RepoURL:   "ssh://git@gitlab.com/user/repo.git",
				FilePath:  "configs/app.yaml",
				Reference: "develop",
				AuthType:  "key",
				AuthData:  map[string]string{"keypath": "/home/user/.ssh/id_rsa"},
			},
		},
		{
			name: "Basic auth with custom poll",
			url:  "https://git.company.com/team/configs.git?file=production.toml&ref=v2.0.0&auth=basic:admin:secret&poll=120s",
			expected: GitURL{
				RepoURL:      "https://git.company.com/team/configs.git",
				FilePath:     "production.toml",
				Reference:    "v2.0.0",
				AuthType:     "basic",
				AuthData:     map[string]string{"username": "admin", "password": "secret"},
				PollInterval: 120 * time.Second,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := provider.parseGitURL(tc.url)
			if err != nil {
				t.Fatalf("Unexpected error parsing URL: %v", err)
			}

			// Verify core fields
			if result.RepoURL != tc.expected.RepoURL {
				t.Errorf("Expected RepoURL '%s', got '%s'", tc.expected.RepoURL, result.RepoURL)
			}

			if result.FilePath != tc.expected.FilePath {
				t.Errorf("Expected FilePath '%s', got '%s'", tc.expected.FilePath, result.FilePath)
			}

			if result.Reference != tc.expected.Reference {
				t.Errorf("Expected Reference '%s', got '%s'", tc.expected.Reference, result.Reference)
			}

			if result.AuthType != tc.expected.AuthType {
				t.Errorf("Expected AuthType '%s', got '%s'", tc.expected.AuthType, result.AuthType)
			}

			// Verify auth data
			for key, expectedValue := range tc.expected.AuthData {
				if actualValue, exists := result.AuthData[key]; !exists || actualValue != expectedValue {
					t.Errorf("Expected AuthData[%s] = '%s', got '%s' (exists: %v)", key, expectedValue, actualValue, exists)
				}
			}

			// Verify poll interval if specified
			if tc.expected.PollInterval != 0 {
				if result.PollInterval != tc.expected.PollInterval {
					t.Errorf("Expected PollInterval %v, got %v", tc.expected.PollInterval, result.PollInterval)
				}
			}
		})
	}
}

// TestGitProvider_ResourceManagement tests resource limits and cleanup
func TestGitProvider_ResourceManagement(t *testing.T) {
	provider := &GitProvider{
		authCache: make(map[string]transport.AuthMethod),
		repoCache: make(map[string]*repoMetadata),
		tempDirs:  make([]string, 0),
	}

	// Test operation count limits
	t.Run("Operation Count Limits", func(t *testing.T) {
		// Fill up to the limit
		for i := 0; i < maxConcurrentOperations; i++ {
			if !provider.incrementOperationCount() {
				t.Errorf("Failed to increment operation count at %d/%d", i, maxConcurrentOperations)
			}
		}

		// Should reject the next one
		if provider.incrementOperationCount() {
			t.Error("Should have rejected operation count increment beyond limit")
		}

		// Cleanup
		for i := 0; i < maxConcurrentOperations; i++ {
			provider.decrementOperationCount()
		}
	})

	// Test watch count limits
	t.Run("Watch Count Limits", func(t *testing.T) {
		// Fill up to the limit
		for i := 0; i < maxActiveWatches; i++ {
			if !provider.incrementWatchCount() {
				t.Errorf("Failed to increment watch count at %d/%d", i, maxActiveWatches)
			}
		}

		// Should reject the next one
		if provider.incrementWatchCount() {
			t.Error("Should have rejected watch count increment beyond limit")
		}

		// Cleanup
		for i := 0; i < maxActiveWatches; i++ {
			provider.decrementWatchCount()
		}
	})

	// Note: Close functionality not tested as it's not part of RemoteConfigProvider interface
}

// TestGitProvider_SecurityValidation tests security validation functions
func TestGitProvider_SecurityValidation(t *testing.T) {
	securityTests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "Path Traversal Protection",
			testFunc: func(t *testing.T) {
				maliciousPaths := []string{
					"../../../etc/passwd",
					"..\\..\\..\\windows\\system32\\config\\sam",
					"./../config.json",
					"config/../../../secret.key",
				}

				for _, path := range maliciousPaths {
					err := validateConfigFilePath(path)
					if err == nil {
						t.Errorf("Path traversal attempt should be blocked: %s", path)
					}
				}
			},
		},
		{
			name: "Sensitive File Protection",
			testFunc: func(t *testing.T) {
				sensitivePaths := []string{
					".git/config",
					".ssh/id_rsa",
					"private.key",
					"secret.env",
					"config.token",
				}

				for _, path := range sensitivePaths {
					err := validateConfigFilePath(path)
					if err == nil {
						t.Errorf("Access to sensitive file should be blocked: %s", path)
					}
				}
			},
		},
		{
			name: "Host Security Validation",
			testFunc: func(t *testing.T) {
				dangerousHosts := []string{
					"localhost",
					"127.0.0.1",
					"::1",
					"10.0.0.1",
					"192.168.1.1",
					"172.16.0.1",
				}

				for _, host := range dangerousHosts {
					err := validateGitHost(host)
					if err == nil {
						t.Errorf("Dangerous host should be blocked: %s", host)
					}
				}
			},
		},
	}

	for _, test := range securityTests {
		t.Run(test.name, test.testFunc)
	}
}

// TestGetProvider tests the factory function
func TestGetProvider(t *testing.T) {
	provider := GetProvider()

	if provider == nil {
		t.Fatal("GetProvider should not return nil")
	}

	// Provider implements RemoteConfigProvider interface by design

	// Test that it's properly initialized
	gitProvider, ok := provider.(*GitProvider)
	if !ok {
		t.Error("Provider should be of type *GitProvider")
	}

	if gitProvider.authCache == nil {
		t.Error("authCache should be initialized")
	}

	if gitProvider.repoCache == nil {
		t.Error("repoCache should be initialized")
	}

	if gitProvider.tempDirs == nil {
		t.Error("tempDirs should be initialized")
	}

	// Note: Cleanup testing not applicable as Close() not in RemoteConfigProvider interface
}

// Helper function to generate long strings for testing
func generateLongString(length int) string {
	result := make([]byte, length)
	for i := range result {
		result[i] = 'a'
	}
	return string(result)
}

// Benchmark tests for performance validation
func BenchmarkGitProvider_Validate(b *testing.B) {
	provider := GetProvider()
	testURL := "https://github.com/user/repo.git#config.json?ref=main"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.Validate(testURL)
	}
}

func BenchmarkGitURL_Parsing(b *testing.B) {
	provider := &GitProvider{
		authCache: make(map[string]transport.AuthMethod),
		repoCache: make(map[string]*repoMetadata),
		tempDirs:  make([]string, 0),
	}
	testURL := "https://github.com/user/repo.git#config.json?ref=main&auth=token:ghp_xxx"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = provider.parseGitURL(testURL)
	}
}
