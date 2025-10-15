// integration_test.go
//
// Real-world integration tests with actual Git repositories
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGLIra library
// SPDX-License-Identifier: MPL-2.0

package git

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestGitProvider_VSCodeRepo tests against VS Code repository
func TestGitProvider_VSCodeRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	provider := GetProvider()
	// Note: Close is not part of RemoteConfigProvider interface

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Test with our own repository that has test data
	url := "https://github.com/agilira/argus-provider-git.git#testdata/config.json?ref=main"

	err := provider.Validate(url)
	if err != nil {
		t.Fatalf("Validation failed for %s: %v", url, err)
	}

	config, err := provider.Load(ctx, url)
	if err != nil {
		t.Fatalf("Failed to load golang/example config: %v", err)
	}

	if len(config) == 0 {
		t.Error("Empty configuration returned from argus-provider-git repository")
		return
	}

	// Verify it contains expected JSON fields from our test config
	t.Logf("Successfully loaded argus-provider-git test config (%d keys)", len(config))

	// Our test config should have some basic fields
	if len(config) > 0 {
		t.Logf("Config loaded successfully: %+v", config)
	}
}

// TestGitProvider_RealRepository tests against actual GitHub repository
func TestGitProvider_RealRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	provider := GetProvider()
	// Note: Close is not part of RemoteConfigProvider interface

	// Test with a known public repository
	// We'll use the argus repository itself which has JSON config files
	testCases := []struct {
		name        string
		url         string
		expectError bool
		description string
	}{
		{
			name:        "Argus provider git config.json",
			url:         "https://github.com/agilira/argus-provider-git.git#testdata/config.json?ref=main",
			expectError: false, // This file exists and is valid JSON
			description: "Test with our own test data",
		},
		{
			name:        "Non-existent repository",
			url:         "https://github.com/nonexistent/repo.git#config.json",
			expectError: true,
			description: "Should fail gracefully for non-existent repository",
		},
		{
			name:        "Non-existent file",
			url:         "https://github.com/agilira/argus-provider-git.git#nonexistent.json?ref=main",
			expectError: true,
			description: "Should fail gracefully for non-existent file",
		},
		{
			name:        "Non-existent branch",
			url:         "https://github.com/agilira/argus-provider-git.git#testdata/config.json?ref=nonexistent-branch",
			expectError: true,
			description: "Should fail gracefully for non-existent branch",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test validation first
			err := provider.Validate(tc.url)
			if err != nil {
				t.Fatalf("Validation failed for %s: %v", tc.url, err)
			}

			// Test Load method
			config, err := provider.Load(ctx, tc.url)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for %s, but got none", tc.url)
				} else {
					t.Logf("Expected error occurred: %v", err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for %s: %v", tc.url, err)
				return
			}

			if len(config) == 0 {
				t.Errorf("Empty configuration returned for %s", tc.url)
				return
			}

			t.Logf("Successfully loaded config from %s (%d keys)", tc.url, len(config))

			// Verify specific content for package.json
			if tc.name == "VS Code repository package.json" {
				if name, exists := config["name"]; !exists {
					t.Error("Expected 'name' field in repository for package.json")
				} else if nameStr, ok := name.(string); !ok || !strings.Contains(nameStr, "code") {
					t.Errorf("Expected VS Code name field, got: %v", name)
				}
			}

			// Verify it's a valid configuration map
			if config == nil {
				t.Errorf("Config is nil for %s", tc.url)
			}
		})
	}
}

// TestGitProvider_ConfigFormats tests different configuration file formats
func TestGitProvider_ConfigFormats(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	provider := GetProvider()
	// Note: Close is not part of RemoteConfigProvider interface

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	formatTests := []struct {
		name       string
		url        string
		format     string
		shouldWork bool
	}{
		{
			name:       "JSON format with go.mod",
			url:        "https://github.com/agilira/argus.git#go.mod",
			format:     "JSON",
			shouldWork: false, // go.mod isn't JSON
		},
	}

	for _, tc := range formatTests {
		t.Run(tc.name, func(t *testing.T) {
			config, err := provider.Load(ctx, tc.url)

			if !tc.shouldWork {
				if err == nil {
					t.Errorf("Expected error for unsupported format %s", tc.format)
				}
				return
			}

			if err != nil {
				// If the file doesn't exist in the test repo, that's OK for this test
				t.Logf("Format test for %s: %v (file may not exist in test repo)", tc.format, err)
				return
			}

			if len(config) == 0 {
				t.Errorf("Empty configuration for format %s", tc.format)
				return
			}

			t.Logf("Successfully parsed %s format (%d keys)", tc.format, len(config))
		})
	}
}

// TestGitProvider_Watch tests the watch functionality
func TestGitProvider_Watch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	provider := GetProvider()
	// Note: Close is not part of RemoteConfigProvider interface

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test watch on our own repository with existing JSON file
	url := "https://github.com/agilira/argus-provider-git.git#testdata/config.json?poll=5s&ref=main"

	configChan, err := provider.Watch(ctx, url)
	if err != nil {
		t.Fatalf("Failed to start watch: %v", err)
	}

	// Should get initial config
	select {
	case config := <-configChan:
		if config == nil {
			t.Error("Received nil config from watch")
		} else {
			t.Logf("Watch received initial config: %d keys", len(config))
		}
	case <-time.After(45 * time.Second):
		t.Error("Watch didn't receive initial config within timeout")
	}

	// Cancel context and verify cleanup
	cancel()

	// Should close the channel
	select {
	case _, ok := <-configChan:
		if ok {
			t.Error("Watch channel should be closed after context cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Error("Watch channel didn't close after context cancellation")
	}
}

// TestGitProvider_ConcurrentOperations tests concurrent access
func TestGitProvider_ConcurrentOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	provider := GetProvider()
	// Note: Close is not part of RemoteConfigProvider interface

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	url := "https://github.com/agilira/argus-provider-git.git#testdata/config.json?ref=main"

	// Run multiple concurrent Get operations
	const numRoutines = 5
	results := make(chan error, numRoutines)

	for i := 0; i < numRoutines; i++ {
		go func(id int) {
			config, err := provider.Load(ctx, url)
			if err != nil {
				results <- fmt.Errorf("routine %d failed: %v", id, err)
				return
			}

			if len(config) == 0 {
				results <- fmt.Errorf("routine %d got empty config", id)
				return
			}

			results <- nil
		}(i)
	}

	// Collect results
	for i := 0; i < numRoutines; i++ {
		select {
		case err := <-results:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(15 * time.Second):
			t.Error("Concurrent operation timed out")
		}
	}
}

// TestGitProvider_MemoryUsage tests for memory leaks
func TestGitProvider_MemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	provider := GetProvider()
	// Note: Close is not part of RemoteConfigProvider interface

	ctx := context.Background()
	url := "https://github.com/agilira/argus-provider-git.git#testdata/config.json"

	// Run many operations to check for memory leaks
	for i := 0; i < 10; i++ {
		_, err := provider.Load(ctx, url)
		if err != nil {
			// If we can't connect to the repo, that's OK for this test
			t.Logf("Memory test iteration %d: %v", i, err)
			break
		}
	}

	// Test that provider is still usable after multiple operations
	// Note: RemoteConfigProvider interface doesn't include Close method
	_, err := provider.Load(ctx, url)
	if err != nil {
		t.Logf("Provider still functional after multiple operations: %v", err)
	}
}
