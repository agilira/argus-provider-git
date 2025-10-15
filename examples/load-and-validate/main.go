// Load and Validate Example for Argus Git Provider
//
// This example demonstrates advanced usage including validation,
// error handling, and configuration loading with timeouts.
//
// Run with: go run main.go
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGLIra library
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	git "github.com/agilira/argus-provider-git"
)

func main() {
	fmt.Println("Argus Git Provider - Load and Validate Example")
	fmt.Println(strings.Repeat("=", 50))

	provider := git.GetProvider()
	fmt.Printf(" Provider initialized: %s\n", provider.Name())

	// Test multiple URLs to demonstrate validation
	testURLs := []struct {
		url         string
		description string
		shouldPass  bool
	}{
		{
			url:         "https://github.com/agilira/argus-provider-git.git#testdata/config.json?ref=main",
			description: "Valid repository with JSON config",
			shouldPass:  true,
		},
		{
			url:         "https://github.com/microsoft/vscode.git#package.json?ref=main",
			description: "VS Code repository package.json",
			shouldPass:  true,
		},
		{
			url:         "invalid-url-format",
			description: "Invalid URL format",
			shouldPass:  false,
		},
		{
			url:         "https://localhost/repo.git#config.json",
			description: "Localhost (security blocked)",
			shouldPass:  false,
		},
	}

	fmt.Println("\nTesting URL validation...")
	for i, test := range testURLs {
		fmt.Printf("\n%d. %s\n", i+1, test.description)
		fmt.Printf("   URL: %s\n", test.url)

		err := provider.Validate(test.url)
		if test.shouldPass {
			if err != nil {
				fmt.Printf("   FAIL: Unexpected validation failure: %v\n", err)
			} else {
				fmt.Printf("   PASS: Validation passed (as expected)\n")
			}
		} else {
			if err != nil {
				fmt.Printf("   PASS: Validation correctly failed: %v\n", err)
			} else {
				fmt.Printf("   FAIL: Validation should have failed but didn't\n")
			}
		}
	}

	// Now test actual loading with the valid URL
	validURL := testURLs[0].url
	fmt.Printf("\nLoading configuration from: %s\n", validURL)

	// Test with different timeout scenarios
	timeouts := []time.Duration{
		1 * time.Second,  // Short timeout (might fail)
		10 * time.Second, // Medium timeout
		30 * time.Second, // Long timeout (should work)
	}

	for i, timeout := range timeouts {
		fmt.Printf("\n Attempt %d with %v timeout...\n", i+1, timeout)

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		start := time.Now()

		config, err := provider.Load(ctx, validURL)
		duration := time.Since(start)
		cancel()

		if err != nil {
			fmt.Printf("   FAIL: Load failed after %v: %v\n", duration, err)
			if i < len(timeouts)-1 {
				fmt.Printf("    Trying with longer timeout...\n")
			}
		} else {
			fmt.Printf("   PASS: Load succeeded in %v\n", duration)
			fmt.Printf("   Configuration loaded: %d keys\n", len(config))

			// Display some sample configuration
			fmt.Printf("   Sample configuration keys:\n")
			count := 0
			for key := range config {
				if count < 3 { // Show first 3 keys
					fmt.Printf("      • %s\n", key)
					count++
				}
			}
			if len(config) > 3 {
				fmt.Printf("      ... and %d more keys\n", len(config)-3)
			}
			break
		}
	}

	// Health check demonstration
	fmt.Printf("\nTesting provider health check...\n")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err := provider.HealthCheck(ctx, validURL)
	if err != nil {
		fmt.Printf("   FAIL: Health check failed: %v\n", err)
	} else {
		fmt.Printf("   PASS: Health check passed - repository is accessible\n")
	}

	fmt.Println("\n Load and validate example completed!")
}
