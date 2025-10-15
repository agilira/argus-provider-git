// main_test.go
//
// Tests for the basic usage example
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

// TestBasicUsage tests the basic usage example functionality
func TestBasicUsage(t *testing.T) {
	t.Log("Testing basic usage functionality...")

	// Test provider initialization
	provider := git.GetProvider()
	if provider == nil {
		t.Fatal("Provider should not be nil")
	}

	expectedName := "Git Configuration Provider"
	if provider.Name() != expectedName {
		t.Errorf("Expected provider name '%s', got '%s'", expectedName, provider.Name())
	}

	expectedScheme := "git"
	if provider.Scheme() != expectedScheme {
		t.Errorf("Expected scheme '%s', got '%s'", expectedScheme, provider.Scheme())
	}

	t.Log("✅ Provider initialization test passed")
}

// TestURLValidation tests the URL validation functionality
func TestURLValidation(t *testing.T) {
	t.Log("Testing URL validation...")

	provider := git.GetProvider()

	validURL := "https://github.com/agilira/argus-provider-git.git#testdata/config.json?ref=main"
	err := provider.Validate(validURL)
	if err != nil {
		t.Errorf("Valid URL should pass validation: %v", err)
	}

	invalidURLs := []string{
		"",
		"invalid-url",
		"https://localhost/repo.git#config.json",
		"https://github.com/user/repo.git", // Missing file
		"https://github.com/user/repo.git#config.txt",          // Invalid extension
		"https://github.com/user/repo.git#../../../etc/passwd", // Path traversal
	}

	for _, url := range invalidURLs {
		err := provider.Validate(url)
		if err == nil {
			t.Errorf("Invalid URL should fail validation: %s", url)
		}
	}

	t.Log("✅ URL validation test passed")
}

// TestConfigurationLoad tests loading configuration (integration test)
func TestConfigurationLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Log("Testing configuration load (integration test)...")

	provider := git.GetProvider()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	configURL := "https://github.com/agilira/argus-provider-git.git#testdata/config.json?ref=main"

	config, err := provider.Load(ctx, configURL)
	if err != nil {
		t.Fatalf("Load should succeed: %v", err)
	}

	if config == nil {
		t.Fatal("Config should not be nil")
	}

	if len(config) == 0 {
		t.Error("Config should have at least one key")
	}

	// Test that expected keys exist (based on testdata/config.json)
	expectedKeys := []string{"database", "logging", "features"}
	for _, key := range expectedKeys {
		if _, exists := config[key]; !exists {
			t.Errorf("Expected key '%s' not found in config", key)
		}
	}

	t.Logf("✅ Configuration load test passed - loaded %d keys", len(config))
}

// TestHealthCheck tests the health check functionality
func TestHealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Log("Testing health check...")

	provider := git.GetProvider()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	configURL := "https://github.com/agilira/argus-provider-git.git#testdata/config.json?ref=main"

	err := provider.HealthCheck(ctx, configURL)
	if err != nil {
		t.Errorf("Health check should pass: %v", err)
	}

	t.Log("✅ Health check test passed")
}

// BenchmarkProviderInitialization benchmarks provider creation
func BenchmarkProviderInitialization(b *testing.B) {
	for i := 0; i < b.N; i++ {
		provider := git.GetProvider()
		_ = provider.Name()
	}
}

// BenchmarkURLValidation benchmarks URL validation
func BenchmarkURLValidation(b *testing.B) {
	provider := git.GetProvider()
	testURL := "https://github.com/user/repo.git#config.json?ref=main"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.Validate(testURL)
	}
}
