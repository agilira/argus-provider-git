// main_test.go
//
// Tests for the load-and-validate example
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGLIra library
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"testing"
	"time"

	git "github.com/agilira/argus-provider-git"
)

// TestLoadAndValidate tests the load and validate example functionality
func TestLoadAndValidate(t *testing.T) {
	t.Log("Testing load and validate functionality...")

	provider := git.GetProvider()
	if provider == nil {
		t.Fatal("Provider should not be nil")
	}

	t.Log("✅ Provider initialization test passed")
}

// TestMultipleURLValidation tests validation with multiple URLs
func TestMultipleURLValidation(t *testing.T) {
	t.Log("Testing multiple URL validation scenarios...")

	provider := git.GetProvider()

	testCases := []struct {
		url         string
		shouldPass  bool
		description string
	}{
		{
			url:         "https://github.com/agilira/argus-provider-git.git#testdata/config.json?ref=main",
			shouldPass:  true,
			description: "Valid repository with JSON config",
		},
		{
			url:         "invalid-url-format",
			shouldPass:  false,
			description: "Invalid URL format",
		},
		{
			url:         "https://localhost/repo.git#config.json",
			shouldPass:  false,
			description: "Localhost (security blocked)",
		},
		{
			url:         "https://192.168.1.1/repo.git#config.json",
			shouldPass:  false,
			description: "Private network (security blocked)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			err := provider.Validate(tc.url)
			if tc.shouldPass {
				if err != nil {
					t.Errorf("URL should be valid: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("URL should be invalid: %s", tc.url)
				}
			}
		})
	}

	t.Log("✅ Multiple URL validation test passed")
}

// TestTimeoutScenarios tests different timeout scenarios
func TestTimeoutScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}

	t.Log("Testing timeout scenarios...")

	provider := git.GetProvider()
	configURL := "https://github.com/agilira/argus-provider-git.git#testdata/config.json?ref=main"

	// Test with reasonable timeout (should succeed)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	config, err := provider.Load(ctx, configURL)
	if err != nil {
		t.Errorf("Load with 30s timeout should succeed: %v", err)
	}

	if config == nil {
		t.Error("Config should not be nil")
	}

	t.Log("✅ Timeout scenarios test passed")
}

// TestHealthCheckScenarios tests health check functionality
func TestHealthCheckScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping health check test in short mode")
	}

	t.Log("Testing health check scenarios...")

	provider := git.GetProvider()

	// Valid repository health check
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	validURL := "https://github.com/agilira/argus-provider-git.git#testdata/config.json?ref=main"
	err := provider.HealthCheck(ctx, validURL)
	if err != nil {
		t.Errorf("Health check for valid repo should pass: %v", err)
	}

	t.Log("✅ Health check scenarios test passed")
}

// TestProviderCapabilities tests various provider capabilities
func TestProviderCapabilities(t *testing.T) {
	t.Log("Testing provider capabilities...")

	provider := git.GetProvider()

	// Test name and scheme
	if provider.Name() == "" {
		t.Error("Provider name should not be empty")
	}

	if provider.Scheme() != "git" {
		t.Errorf("Expected scheme 'git', got '%s'", provider.Scheme())
	}

	t.Log("✅ Provider capabilities test passed")
}

// BenchmarkMultipleValidation benchmarks multiple URL validations
func BenchmarkMultipleValidation(b *testing.B) {
	provider := git.GetProvider()
	urls := []string{
		"https://github.com/user/repo1.git#config.json",
		"https://github.com/user/repo2.git#config.yaml",
		"https://gitlab.com/user/repo3.git#config.toml",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, url := range urls {
			_ = provider.Validate(url)
		}
	}
}
