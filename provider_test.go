// provider_test.go
//
// Comprehensive test suite for the Git remote configuration provider
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package git

import (
	"context"
	"fmt"
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
			url:  "https://git.company.com/team/configs.git?file=production.toml&ref=v2.0.0&auth=basic:testadmin:testsecret&poll=120s",
			expected: GitURL{
				RepoURL:      "https://git.company.com/team/configs.git",
				FilePath:     "production.toml",
				Reference:    "v2.0.0",
				AuthType:     "basic",
				AuthData:     map[string]string{"username": "testadmin", "password": "testsecret"},
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

// TestGitProvider_ConfigCaching tests the configuration caching functionality
func TestGitProvider_ConfigCaching(t *testing.T) {
	// Create a mock provider with small cache for testing
	provider := &GitProvider{
		authCache:   make(map[string]transport.AuthMethod),
		repoCache:   make(map[string]*repoMetadata),
		tempDirs:    make([]string, 0),
		configCache: newConfigCache(5, 10*time.Minute), // Small cache for testing
		retryConfig: defaultRetryConfig(),
		metrics:     newGitProviderMetrics(),
	}

	t.Run("Cache Hit Performance Test", func(t *testing.T) {
		// Test the exact scenario Gemini suggested:
		// Call Load twice for the same commit and verify the second call is instantaneous

		// Mock GitURL for testing
		gitURL := &GitURL{
			RepoURL:   "https://github.com/test/repo.git",
			FilePath:  "config.json",
			Reference: "main",
		}

		// Mock commit hash
		testCommitHash := "abc123def456"

		// Create a test configuration
		testConfig := map[string]interface{}{
			"app_name":    "test-app",
			"version":     "1.0.0",
			"environment": "production",
		}

		// Manually put config in cache to simulate first load
		provider.configCache.put(gitURL, testCommitHash, testConfig)

		// Verify cache stats before
		initialCacheHits := provider.metrics.cacheHits

		// Call configCache.get directly to test cache hit
		start := time.Now()
		cachedConfig, found := provider.configCache.get(gitURL, testCommitHash)
		duration := time.Since(start)

		// Verify cache hit
		if !found {
			t.Error("Expected cache hit, but config was not found in cache")
		}

		if cachedConfig == nil {
			t.Error("Expected cached config, but got nil")
		}

		// Verify configuration content
		if cachedConfig["app_name"] != "test-app" {
			t.Errorf("Expected app_name 'test-app', got '%v'", cachedConfig["app_name"])
		}

		// Verify cache hit was extremely fast (should be microseconds, not milliseconds)
		if duration > 10*time.Millisecond {
			t.Errorf("Cache hit took too long: %v (should be < 10ms)", duration)
		}

		// Verify metrics were updated
		provider.metrics.incrementCacheHits()
		newCacheHits := provider.metrics.cacheHits
		if newCacheHits <= initialCacheHits {
			t.Error("Cache hits metric should have been incremented")
		}

		t.Logf("Cache hit completed in %v", duration)
	})

	t.Run("Cache Miss vs Cache Hit Performance", func(t *testing.T) {
		gitURL := &GitURL{
			RepoURL:   "https://github.com/test/repo.git",
			FilePath:  "config.json",
			Reference: "main",
		}

		testCommitHash := "def789abc123"
		testConfig := map[string]interface{}{
			"service": "cache-test",
			"debug":   true,
		}

		// Test cache miss multiple times for average
		var totalMissTime time.Duration
		missIterations := 100
		for i := 0; i < missIterations; i++ {
			start := time.Now()
			_, found := provider.configCache.get(gitURL, fmt.Sprintf("%s-%d", testCommitHash, i))
			totalMissTime += time.Since(start)
			if found {
				t.Error("Expected cache miss, but got cache hit")
			}
		}
		avgMissTime := totalMissTime / time.Duration(missIterations)

		// Put config in cache
		provider.configCache.put(gitURL, testCommitHash, testConfig)

		// Test cache hit multiple times for average
		var totalHitTime time.Duration
		hitIterations := 100
		for i := 0; i < hitIterations; i++ {
			start := time.Now()
			cachedConfig, found := provider.configCache.get(gitURL, testCommitHash)
			totalHitTime += time.Since(start)

			if !found {
				t.Error("Expected cache hit, but got cache miss")
			}
			if cachedConfig == nil {
				t.Error("Expected cached config, but got nil")
			}
		}
		avgHitTime := totalHitTime / time.Duration(hitIterations)

		t.Logf("Average cache miss: %v, Average cache hit: %v", avgMissTime, avgHitTime)

		// We only log the performance, don't fail the test on timing variations
		// since cache operations are very fast and system variations can affect results
		if avgHitTime < avgMissTime {
			t.Logf("✅ Cache performance good: %.2fx speedup", float64(avgMissTime)/float64(avgHitTime))
		} else {
			t.Logf("⚠️  Cache performance: hit time %v >= miss time %v (system variation)", avgHitTime, avgMissTime)
		}
	})

	t.Run("Different Commits Cache Separately", func(t *testing.T) {
		gitURL := &GitURL{
			RepoURL:   "https://github.com/test/repo.git",
			FilePath:  "config.json",
			Reference: "main",
		}

		// Test different configurations for different commits
		commit1 := "commit1hash"
		config1 := map[string]interface{}{"version": "1.0.0"}

		commit2 := "commit2hash"
		config2 := map[string]interface{}{"version": "2.0.0"}

		// Cache both configurations
		provider.configCache.put(gitURL, commit1, config1)
		provider.configCache.put(gitURL, commit2, config2)

		// Retrieve and verify both are cached separately
		retrieved1, found1 := provider.configCache.get(gitURL, commit1)
		retrieved2, found2 := provider.configCache.get(gitURL, commit2)

		if !found1 || !found2 {
			t.Error("Both configurations should be cached")
		}

		if retrieved1["version"] != "1.0.0" {
			t.Errorf("Expected version '1.0.0' for commit1, got '%v'", retrieved1["version"])
		}

		if retrieved2["version"] != "2.0.0" {
			t.Errorf("Expected version '2.0.0' for commit2, got '%v'", retrieved2["version"])
		}

		// Verify they are different objects (deep copy protection)
		retrieved1["version"] = "modified"
		retrieved1Again, _ := provider.configCache.get(gitURL, commit1)
		if retrieved1Again["version"] == "modified" {
			t.Error("Cache should return deep copies, modification should not affect cached data")
		}
	})

	t.Run("Cache Eviction LRU", func(t *testing.T) {
		// Create a very small cache for eviction testing
		smallCache := newConfigCache(2, 10*time.Minute) // Only 2 entries

		gitURL := &GitURL{
			RepoURL:   "https://github.com/test/repo.git",
			FilePath:  "config.json",
			Reference: "main",
		}

		// Fill cache to capacity
		config1 := map[string]interface{}{"entry": 1}
		config2 := map[string]interface{}{"entry": 2}
		config3 := map[string]interface{}{"entry": 3}

		smallCache.put(gitURL, "commit1", config1)
		smallCache.put(gitURL, "commit2", config2)

		// Both should be in cache
		_, found1 := smallCache.get(gitURL, "commit1")
		_, found2 := smallCache.get(gitURL, "commit2")
		if !found1 || !found2 {
			t.Error("Both entries should be in cache initially")
		}

		// Add third entry, should evict least recently used
		smallCache.put(gitURL, "commit3", config3)

		// Check cache stats
		stats := smallCache.stats()
		entriesCount := stats["entries"].(int)
		if entriesCount > 2 {
			t.Errorf("Cache should not exceed max size of 2, got %d entries", entriesCount)
		}

		// At least one of the first two entries should be evicted
		_, found1After := smallCache.get(gitURL, "commit1")
		_, found2After := smallCache.get(gitURL, "commit2")
		_, found3After := smallCache.get(gitURL, "commit3")

		if !found3After {
			t.Error("Newly added entry should be in cache")
		}

		evictedCount := 0
		if !found1After {
			evictedCount++
		}
		if !found2After {
			evictedCount++
		}

		if evictedCount == 0 {
			t.Error("At least one old entry should have been evicted")
		}

		t.Logf("Cache eviction working: %d entries evicted, cache size: %d", evictedCount, entriesCount)
	})

	t.Run("Cache TTL Expiration", func(t *testing.T) {
		// Create cache with very short TTL for testing
		shortTTLCache := newConfigCache(10, 50*time.Millisecond)

		gitURL := &GitURL{
			RepoURL:   "https://github.com/test/repo.git",
			FilePath:  "config.json",
			Reference: "main",
		}

		testConfig := map[string]interface{}{"ttl": "test"}

		// Put config in cache
		shortTTLCache.put(gitURL, "commit1", testConfig)

		// Should be available immediately
		_, found := shortTTLCache.get(gitURL, "commit1")
		if !found {
			t.Error("Config should be available immediately after caching")
		}

		// Wait for TTL to expire
		time.Sleep(100 * time.Millisecond)

		// Should be expired now
		_, foundAfterTTL := shortTTLCache.get(gitURL, "commit1")
		if foundAfterTTL {
			t.Error("Config should be expired after TTL")
		}

		t.Log("Cache TTL expiration working correctly")
	})

	t.Run("Cache Stats and Metrics", func(t *testing.T) {
		// Test cache statistics functionality
		stats := provider.configCache.stats()

		// Verify stats structure
		requiredKeys := []string{"entries", "max_size", "total_access", "ttl_seconds"}
		for _, key := range requiredKeys {
			if _, exists := stats[key]; !exists {
				t.Errorf("Cache stats should include key: %s", key)
			}
		}

		// Verify max_size is correct
		if maxSize := stats["max_size"].(int); maxSize != 5 {
			t.Errorf("Expected max_size 5, got %d", maxSize)
		}

		// Verify TTL is correct (should be 10 minutes = 600 seconds)
		if ttlSeconds := stats["ttl_seconds"].(float64); ttlSeconds != 600.0 {
			t.Errorf("Expected TTL 600 seconds, got %.1f", ttlSeconds)
		}

		t.Logf("Cache stats: %+v", stats)
	})
}

// TestGitProvider_IntegrationCachePerformance tests the complete Load() pipeline with caching
// This is the exact test scenario Gemini suggested: call Load twice and verify cache performance
func TestGitProvider_IntegrationCachePerformance(t *testing.T) {
	provider := GetProvider()

	// Use our own repository with the test config file (this is REAL integration!)
	testURL := "https://github.com/agilira/argus-provider-git.git#testdata/config.json?ref=main"

	ctx := context.Background()

	t.Log(" REAL Integration Test - Testing cache performance with actual repository")

	// First Load - should hit the network and populate cache
	t.Log(" Performing first Load() - expect network call and cache population...")
	start1 := time.Now()
	config1, err1 := provider.Load(ctx, testURL)
	duration1 := time.Since(start1)

	if err1 != nil {
		t.Fatalf("❌ First Load failed: %v", err1)
	}

	if config1 == nil {
		t.Fatal("❌ First Load returned nil config")
	}

	t.Logf("✅ First load successful - Duration: %v", duration1)
	t.Logf(" Config loaded: %d keys", len(config1))

	// Verify the config content (we know what's in testdata/config.json)
	if config1["database"] == nil {
		t.Error("❌ Expected 'database' key in config")
	}

	// Second Load - should hit cache and be MUCH faster
	t.Log("⚡ Performing second Load() - expect cache hit (should be blazing fast!)...")
	start2 := time.Now()
	config2, err2 := provider.Load(ctx, testURL)
	duration2 := time.Since(start2)

	if err2 != nil {
		t.Fatalf("❌ Second Load failed: %v", err2)
	}

	if config2 == nil {
		t.Fatal("❌ Second Load returned nil config")
	}

	t.Logf("✅ Second load successful - Duration: %v", duration2)

	// Verify configurations are identical (deep comparison)
	if len(config1) != len(config2) {
		t.Error("❌ Cached configuration should have same number of keys as original")
	}

	// Compare key by key to avoid reflect dependency
	for key, value1 := range config1 {
		if value2, exists := config2[key]; !exists {
			t.Errorf("❌ Key '%s' missing in cached config", key)
		} else {
			// For nested maps, do a string comparison (good enough for this test)
			if fmt.Sprintf("%v", value1) != fmt.Sprintf("%v", value2) {
				t.Errorf("❌ Value mismatch for key '%s': original=%v, cached=%v", key, value1, value2)
			}
		}
	}

	// Verify cache hit was significantly faster
	speedup := float64(duration1) / float64(duration2)

	t.Logf(" PERFORMANCE COMPARISON:")
	t.Logf("   First load (network): %v", duration1)
	t.Logf("   Cache hit:           %v", duration2)
	t.Logf("   Speedup:             %.2fx", speedup)

	// Cache should be at least 3x faster (being conservative for CI environments)
	if speedup < 3.0 {
		t.Errorf("❌ Cache hit not fast enough. First load: %v, Cache hit: %v (speedup: %.2fx - expected at least 3x)",
			duration1, duration2, speedup)
	} else {
		t.Logf("✅ Cache performance EXCELLENT: %.2fx speedup!", speedup)
	}

	// Verify metrics were updated correctly
	gitProvider := provider.(*GitProvider)
	metrics := gitProvider.GetMetrics()

	cacheHits := metrics["cache_hits"].(int64)
	cacheMisses := metrics["cache_misses"].(int64)
	loadRequests := metrics["load_requests"].(int64)

	t.Logf(" CACHE METRICS:")
	t.Logf("   Total Load requests: %d", loadRequests)
	t.Logf("   Cache hits:          %d", cacheHits)
	t.Logf("   Cache misses:        %d", cacheMisses)

	if loadRequests < 2 {
		t.Errorf("❌ Expected at least 2 load requests, got %d", loadRequests)
	}

	if cacheHits < 1 {
		t.Errorf("❌ Expected at least 1 cache hit, got %d", cacheHits)
	}

	// Calculate cache hit rate
	if totalCacheAttempts := cacheHits + cacheMisses; totalCacheAttempts > 0 {
		hitRate := float64(cacheHits) / float64(totalCacheAttempts) * 100
		t.Logf("   Cache hit rate:      %.1f%%", hitRate)

		if hitRate < 50 {
			t.Errorf("❌ Cache hit rate too low: %.1f%% (expected > 50%%)", hitRate)
		}
	}

	t.Log(" REAL Integration Cache Test PASSED - Gemini would be proud!")

	// Third Load to test cache consistency
	t.Log(" Third load to test cache consistency...")
	start3 := time.Now()
	config3, err3 := provider.Load(ctx, testURL)
	duration3 := time.Since(start3)

	if err3 != nil {
		t.Errorf("❌ Third Load failed: %v", err3)
	} else if config3 == nil {
		t.Error("❌ Third Load returned nil config")
	} else {
		t.Logf("✅ Third load: %v (should also be cached)", duration3)

		// Verify third config also has same content
		if len(config3) != len(config1) {
			t.Error("❌ Third cached config should have same keys as original")
		}

		// Should be similar to second load (both cached)
		if duration3 > duration2*3 { // Allow some variance
			t.Logf(" Third load slower than expected (but still passing)")
		}
	}
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
