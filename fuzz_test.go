// fuzz_test.go - Professional Fuzz Testing Suite for Argus Git Provider
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package git

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

// FuzzValidateSecureGitURL tests URL validation for security issues
func FuzzValidateSecureGitURL(f *testing.F) {
	// Seed with attack vectors
	seeds := []string{
		"https://github.com/user/repo.git",
		"https://127.0.0.1/repo.git", // SSRF
		"https://169.254.169.254/",   // Metadata server
		"file:///etc/passwd",         // Protocol confusion
		"",                           // Empty
		"not-a-url",                  // Malformed
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, gitURL string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("validateSecureGitURL panicked with input %q: %v", truncate(gitURL), r)
			}
		}()

		// Performance check
		start := time.Now()
		parsedURL, err := validateSecureGitURL(gitURL)
		duration := time.Since(start)

		if duration > 100*time.Millisecond {
			t.Errorf("validateSecureGitURL too slow (%v) for: %q", duration, truncate(gitURL))
		}

		if err == nil && parsedURL != nil {
			// Accepted URL - verify security
			if isPrivate(parsedURL.Host) {
				t.Errorf("SECURITY: Private host accepted: %q -> %s", truncate(gitURL), parsedURL.Host)
			}
		}
	})
}

// FuzzValidateGitHost tests host validation
func FuzzValidateGitHost(f *testing.F) {
	seeds := []string{
		"github.com",      // Valid
		"localhost",       // Should be blocked
		"127.0.0.1",       // Should be blocked
		"169.254.169.254", // Metadata server
		"10.0.0.1",        // Private network
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, host string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("validateGitHost panicked with: %q", truncate(host))
			}
		}()

		err := validateGitHost(host)

		if err == nil && isPrivate(host) {
			t.Errorf("SECURITY: Private host accepted: %q", truncate(host))
		}
	})
}

// FuzzValidateRepositoryPath tests path traversal protection
func FuzzValidateRepositoryPath(f *testing.F) {
	seeds := []string{
		"/user/repo.git",            // Valid
		"/../../../etc/passwd",      // Path traversal
		"/%2e%2e/etc/passwd",        // URL encoded traversal
		"/user/../../../etc/shadow", // Mixed traversal
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, path string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("validateRepositoryPath panicked with: %q", truncate(path))
			}
		}()

		err := validateRepositoryPath(path)

		if err == nil && hasPathTraversal(path) {
			t.Errorf("SECURITY: Path traversal accepted: %q", truncate(path))
		}
	})
}

// FuzzValidateConfigFilePath tests config file path validation
func FuzzValidateConfigFilePath(f *testing.F) {
	seeds := []string{
		"config.json",              // Valid
		"../../../etc/passwd",      // Path traversal
		"config.exe",               // Invalid extension
		"config.json\x00malicious", // Null byte
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, filePath string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("validateConfigFilePath panicked with: %q", truncate(filePath))
			}
		}()

		err := validateConfigFilePath(filePath)

		if err == nil && hasPathTraversal(filePath) {
			t.Errorf("SECURITY: Path traversal in file path accepted: %q", truncate(filePath))
		}
	})
}

// FuzzParseGitURL tests complete URL parsing
func FuzzParseGitURL(f *testing.F) {
	seeds := []string{
		"git://github.com/user/repo.git#config.json",
		"git://127.0.0.1/repo.git#../../../etc/passwd",
		"git://github.com/repo.git#config.json?ref=../../etc/passwd",
		"not-git://github.com/repo.git#config.json",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, configURL string) {
		provider := &GitProvider{}

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("parseGitURL panicked with: %q", truncate(configURL))
			}
		}()

		gitURL, err := provider.parseGitURL(configURL)

		if err == nil && gitURL != nil {
			// Check security
			if hasPathTraversal(gitURL.FilePath) {
				t.Errorf("SECURITY: Path traversal in parsed file path: %q", truncate(gitURL.FilePath))
			}

			if parsedRepo, parseErr := url.Parse(gitURL.RepoURL); parseErr == nil {
				if isPrivate(parsedRepo.Host) {
					t.Errorf("SECURITY: Private host in repo URL: %q", parsedRepo.Host)
				}
			}
		}
	})
}

// Helper functions
func isPrivate(host string) bool {
	if host == "" {
		return false
	}

	// Remove port
	if idx := strings.LastIndex(host, ":"); idx > 0 {
		host = host[:idx]
	}

	// Remove IPv6 brackets
	host = strings.Trim(host, "[]")

	privateHosts := []string{"localhost", "127.0.0.1", "::1"}
	privatePrefixes := []string{"10.", "192.168.", "172.16.", "172.17.", "172.18.",
		"172.19.", "172.2", "172.30.", "172.31.", "169.254."}

	lowerHost := strings.ToLower(host)

	for _, p := range privateHosts {
		if lowerHost == p {
			return true
		}
	}

	for _, prefix := range privatePrefixes {
		if strings.HasPrefix(lowerHost, prefix) {
			return true
		}
	}

	return false
}

func hasPathTraversal(path string) bool {
	// Check both original and URL decoded
	paths := []string{path}
	if decoded, err := url.QueryUnescape(path); err == nil {
		paths = append(paths, decoded)
	}

	patterns := []string{"..", "../", "..\\", "%2e%2e", "%2f", "%5c"}

	for _, p := range paths {
		lowerP := strings.ToLower(p)
		for _, pattern := range patterns {
			if strings.Contains(lowerP, pattern) {
				return true
			}
		}
	}

	return false
}

func truncate(s string) string {
	if len(s) <= 100 {
		return s
	}
	return s[:100] + "..."
}
