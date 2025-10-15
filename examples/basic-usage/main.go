// Basic Usage Example for Argus Git Provider
//
// This example demonstrates the fundamental usage of the Git provider
// for loading configuration from Git repositories.
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
	"log"
	"strings"
	"time"

	git "github.com/agilira/argus-provider-git"
)

func main() {
	fmt.Println("Argus Git Provider - Basic Usage Example")
	fmt.Println(strings.Repeat("=", 50))

	// Step 1: Initialize the provider
	fmt.Println("Initializing Git Provider...")
	provider := git.GetProvider()
	fmt.Printf("Provider: %s (scheme: %s)\n", provider.Name(), provider.Scheme())

	// Step 2: Define configuration URL
	// Using our own repository's test config as a real example
	configURL := "https://github.com/agilira/argus-provider-git.git#testdata/config.json?ref=main"
	fmt.Printf("Configuration URL: %s\n", configURL)

	// Step 3: Validate the URL (optional but recommended)
	fmt.Println("\nValidating URL format...")
	if err := provider.Validate(configURL); err != nil {
		log.Fatalf("URL validation failed: %v", err)
	}
	fmt.Println("URL validation passed")

	// Step 4: Load configuration
	fmt.Println("\nLoading configuration from Git repository...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	config, err := provider.Load(ctx, configURL)
	if err != nil {
		log.Fatalf(" Failed to load configuration: %v", err)
	}

	fmt.Printf("Configuration loaded successfully! (%d keys)\n", len(config))

	// Step 5: Display configuration
	fmt.Println("\nConfiguration content:")
	for key, value := range config {
		fmt.Printf("  - %s: %v\n", key, value)
	}

	// Step 6: Demonstrate URL scheme handling
	fmt.Println("\nProvider capabilities:")
	fmt.Printf("  - Handles scheme: %s://\n", provider.Scheme())
	fmt.Printf("  - Supports formats: JSON, YAML, TOML\n")
	fmt.Printf("  - Authentication: Token, SSH, Basic Auth\n")
	fmt.Printf("  - Features: Caching, Watching, Validation\n")

	fmt.Println("\nBasic usage example completed successfully!")
}
