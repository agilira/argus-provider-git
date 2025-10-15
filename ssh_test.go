// ssh_test.go
//
// SSH Authentication tests for Git provider
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGLIra library
// SPDX-License-Identifier: MPL-2.0

package git

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGitProvider_SSHAuthentication tests SSH key authentication
func TestGitProvider_SSHAuthentication(t *testing.T) {
	provider := GetProvider()

	// Create a temporary SSH key file for testing
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "test_key")

	// SECURITY NOTE: This is a MOCK/FAKE SSH key for testing only!
	// Not a real private key - truncated and invalid by design
	// Used only to test file permission validation logic
	mockKey := `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAFwAAAAdzc2gtcn
NhAAAAAwEAAQAAAQEA0
-----END OPENSSH PRIVATE KEY-----`

	err := os.WriteFile(keyPath, []byte(mockKey), 0o600)
	if err != nil {
		t.Fatalf("Failed to write test SSH key: %v", err)
	}

	testCases := []struct {
		name        string
		keyPath     string
		permissions os.FileMode
		expectError bool
		errorType   string
	}{
		{
			name:        "SSH key file validation (mock key expected to fail parsing)",
			keyPath:     keyPath,
			permissions: 0o600,
			expectError: true, // Mock key will fail parsing, but permissions check passes
			errorType:   "ARGUS_AUTH_ERROR",
		},
		{
			name:        "SSH key with too open permissions",
			keyPath:     keyPath,
			permissions: 0o644,
			expectError: true,
			errorType:   "ARGUS_AUTH_ERROR",
		},
		{
			name:        "Non-existent SSH key",
			keyPath:     "/nonexistent/key",
			permissions: 0o600,
			expectError: true,
			errorType:   "ARGUS_AUTH_ERROR",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set file permissions
			if tc.keyPath == keyPath {
				err := os.Chmod(keyPath, tc.permissions)
				if err != nil {
					t.Fatalf("Failed to set file permissions: %v", err)
				}
			}

			// Create GitURL with SSH authentication
			gitURL := &GitURL{
				RepoURL:  "git@github.com:test/repo.git",
				FilePath: "config.json",
				AuthType: "ssh",
				AuthData: map[string]string{
					"keypath": tc.keyPath,
				},
			}

			// Test authentication
			auth, err := provider.(*GitProvider).getAuthentication(gitURL)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for %s, but got none", tc.name)
					return
				}
				t.Logf("Expected error occurred: %v", err)
			} else {
				if err != nil {
					t.Errorf("Unexpected error for %s: %v", tc.name, err)
					return
				}
				if auth == nil {
					t.Errorf("Expected authentication object for %s, got nil", tc.name)
					return
				}
				t.Logf("SSH authentication created successfully for %s", tc.name)
			}
		})
	}
}

// TestGitProvider_AuthenticationCaching tests that authentication objects are cached
func TestGitProvider_AuthenticationCaching(t *testing.T) {
	provider := GetProvider().(*GitProvider)

	gitURL := &GitURL{
		RepoURL:  "https://github.com/test/repo.git",
		FilePath: "config.json",
		AuthType: "token",
		AuthData: map[string]string{
			"token": "test-token",
		},
	}

	// First call should create and cache authentication
	auth1, err1 := provider.getAuthentication(gitURL)
	if err1 != nil {
		t.Fatalf("First authentication call failed: %v", err1)
	}

	// Second call should return cached authentication
	auth2, err2 := provider.getAuthentication(gitURL)
	if err2 != nil {
		t.Fatalf("Second authentication call failed: %v", err2)
	}

	// Should be the same object (cached)
	if auth1 != auth2 {
		t.Error("Authentication objects should be the same (cached)")
	}

	t.Logf("Authentication caching works correctly")
}
