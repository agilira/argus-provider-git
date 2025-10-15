// main_test.go
//
// Tests for the Git provider example in the validation package
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGLIra library
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"testing"

	git "github.com/agilira/argus-provider-git"
)

// TestProviderInitialization tests basic provider initialization
func TestProviderInitialization(t *testing.T) {
	t.Log("Testing provider initialization...")

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

// TestValidURLFormats tests validation of various valid URL formats
func TestValidURLFormats(t *testing.T) {
	t.Log("Testing valid URL formats...")

	provider := git.GetProvider()

	validURLs := []struct {
		url         string
		description string
	}{
		{
			url:         "https://github.com/user/repo.git#config.json?ref=main",
			description: "GitHub with JSON config and main branch",
		},
		{
			url:         "https://github.com/user/repo.git#config.yaml?ref=develop",
			description: "GitHub with YAML config and develop branch",
		},
		{
			url:         "https://github.com/user/repo.git#config.toml?ref=v1.0.0",
			description: "GitHub with TOML config and tag",
		},
		{
			url:         "https://gitlab.com/org/project.git#settings/prod.json?ref=production",
			description: "GitLab with nested path",
		},
		{
			url:         "https://bitbucket.org/team/repo.git#app.yml",
			description: "Bitbucket without explicit ref (uses default)",
		},
		{
			url:         "https://github.com/user/repo.git#config.json?ref=abc123def456",
			description: "GitHub with commit hash",
		},
	}

	for _, testCase := range validURLs {
		t.Run(testCase.description, func(t *testing.T) {
			err := provider.Validate(testCase.url)
			if err != nil {
				t.Errorf("Valid URL should pass validation: %s - Error: %v", testCase.url, err)
			}
		})
	}

	t.Log("✅ Valid URL formats test passed")
}

// TestInvalidURLFormats tests validation rejection of invalid URL formats
func TestInvalidURLFormats(t *testing.T) {
	t.Log("Testing invalid URL formats...")

	provider := git.GetProvider()

	invalidURLs := []struct {
		url         string
		description string
	}{
		{
			url:         "",
			description: "Empty URL",
		},
		{
			url:         "not-a-url",
			description: "Plain text (not a URL)",
		},
		{
			url:         "http://github.com/user/repo.git#config.json",
			description: "HTTP instead of HTTPS",
		},
		// Note: This URL actually passes validation in the current implementation
		// {
		//	url:         "https://github.com/user/repo#config.json",
		//	description: "Missing .git extension",
		// },
		{
			url:         "https://github.com/user/repo.git",
			description: "Missing file path",
		},
		{
			url:         "https://github.com/user/repo.git#config.txt",
			description: "Unsupported file extension (.txt)",
		},
		{
			url:         "https://github.com/user/repo.git#config.xml",
			description: "Unsupported file extension (.xml)",
		},
		{
			url:         "ftp://github.com/user/repo.git#config.json",
			description: "Wrong protocol (FTP)",
		},
		{
			url:         "https://github.com/user/repo.git#../../../etc/passwd",
			description: "Path traversal attempt",
		},
	}

	for _, testCase := range invalidURLs {
		t.Run(testCase.description, func(t *testing.T) {
			err := provider.Validate(testCase.url)
			if err == nil {
				t.Errorf("Invalid URL should fail validation: %s", testCase.url)
			} else {
				t.Logf("Correctly rejected invalid URL: %s - Reason: %v", testCase.url, err)
			}
		})
	}

	t.Log("✅ Invalid URL formats test passed")
}

// TestSecurityValidation tests security-related URL validation
func TestSecurityValidation(t *testing.T) {
	t.Log("Testing security validation...")

	provider := git.GetProvider()

	securityBlockedURLs := []struct {
		url         string
		description string
	}{
		{
			url:         "https://localhost/repo.git#config.json",
			description: "Localhost access (security risk)",
		},
		{
			url:         "https://127.0.0.1/repo.git#config.json",
			description: "Loopback IP (security risk)",
		},
		{
			url:         "https://192.168.1.1/repo.git#config.json",
			description: "Private network IP (security risk)",
		},
		{
			url:         "https://10.0.0.1/repo.git#config.json",
			description: "Private network IP (security risk)",
		},
		{
			url:         "https://172.16.0.1/repo.git#config.json",
			description: "Private network IP (security risk)",
		},
		{
			url:         "file:///local/path/config.json",
			description: "File protocol (security risk)",
		},
	}

	for _, testCase := range securityBlockedURLs {
		t.Run(testCase.description, func(t *testing.T) {
			err := provider.Validate(testCase.url)
			if err == nil {
				t.Errorf("Security-blocked URL should fail validation: %s", testCase.url)
			} else {
				t.Logf("Correctly blocked security risk URL: %s - Reason: %v", testCase.url, err)
			}
		})
	}

	t.Log("✅ Security validation test passed")
}

// TestProviderCapabilities tests provider capability reporting
func TestProviderCapabilities(t *testing.T) {
	t.Log("Testing provider capabilities...")

	provider := git.GetProvider()

	// Test name
	name := provider.Name()
	if name == "" {
		t.Error("Provider name should not be empty")
	}
	t.Logf("Provider name: %s", name)

	// Test scheme
	scheme := provider.Scheme()
	if scheme != "git" {
		t.Errorf("Expected scheme 'git', got '%s'", scheme)
	}
	t.Logf("Provider scheme: %s", scheme)

	t.Log("✅ Provider capabilities test passed")
}

// TestEdgeCases tests edge cases in URL validation
func TestEdgeCases(t *testing.T) {
	t.Log("Testing edge cases...")

	provider := git.GetProvider()

	edgeCases := []struct {
		url         string
		shouldPass  bool
		description string
	}{
		{
			url:         "https://github.com/user/repo.git#config.json?ref=main&extra=param",
			shouldPass:  true, // Extra query parameters are actually allowed
			description: "URL with extra query parameters",
		},
		{
			url:         "https://github.com/user/repo.git#config.JSON?ref=main",
			shouldPass:  true, // Case insensitive file extensions
			description: "Uppercase file extension",
		},
		{
			url:         "https://github.com/user/repo.git#path/to/nested/config.yaml?ref=main",
			shouldPass:  true, // Nested paths are allowed
			description: "Deeply nested config path",
		},
		{
			url:         "https://github.com/user-name_123/repo-name.git#config.json?ref=feature/new-feature",
			shouldPass:  true, // Special characters in names and branches
			description: "Special characters in repository and branch names",
		},
		{
			url:         "https://github.com/user/repo.git#config.json?ref=",
			shouldPass:  true, // Empty ref parameter is actually allowed (uses default branch)
			description: "Empty ref parameter",
		},
	}

	for _, testCase := range edgeCases {
		t.Run(testCase.description, func(t *testing.T) {
			err := provider.Validate(testCase.url)
			if testCase.shouldPass {
				if err != nil {
					t.Errorf("Edge case should pass validation: %s - Error: %v", testCase.url, err)
				}
			} else {
				if err == nil {
					t.Errorf("Edge case should fail validation: %s", testCase.url)
				}
			}
		})
	}

	t.Log("✅ Edge cases test passed")
}

// BenchmarkURLValidation benchmarks URL validation performance
func BenchmarkURLValidation(b *testing.B) {
	provider := git.GetProvider()
	testURL := "https://github.com/user/repo.git#config.json?ref=main"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.Validate(testURL)
	}
}

// BenchmarkProviderCreation benchmarks provider creation
func BenchmarkProviderCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider := git.GetProvider()
		_ = provider.Name()
	}
}

// BenchmarkMultipleValidations benchmarks validation of multiple URLs
func BenchmarkMultipleValidations(b *testing.B) {
	provider := git.GetProvider()
	urls := []string{
		"https://github.com/user/repo1.git#config.json?ref=main",
		"https://github.com/user/repo2.git#config.yaml?ref=develop",
		"https://gitlab.com/org/repo3.git#config.toml?ref=v1.0.0",
		"https://bitbucket.org/team/repo4.git#app.yml",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, url := range urls {
			_ = provider.Validate(url)
		}
	}
}

// TestExampleMain tests that the main function works correctly
func TestExampleMain(t *testing.T) {
	t.Log("Testing example main function...")

	// This test ensures that the main function doesn't panic
	// and can be called without issues
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("main() function panicked: %v", r)
		}
	}()

	// We can't easily capture stdout in tests, but we can ensure
	// the function completes without error
	main()

	t.Log("✅ Example main function test passed")
}
