// This example demonstrates how to use the Git provider for Argus
// to load configuration from Git repositories.
//
// Run with: go run examples/basic/main.go
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGLIra library
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"fmt"
	"strings"

	git "github.com/agilira/argus-provider-git"
)

// main function for standalone execution and testing
func main() {
	provider := git.GetProvider()
	fmt.Printf("Git Provider initialized: %s (scheme: %s)\n", provider.Name(), provider.Scheme())
	// Note: Close() not part of RemoteConfigProvider interface

	// Example validation test
	testURL := "https://github.com/user/repo.git#config.json?ref=main"
	err := provider.Validate(testURL)
	if err != nil {
		fmt.Printf("Validation failed: %v\n", err)
	} else {
		fmt.Printf("URL validation successful: %s\n", testURL)
	}

	fmt.Println("\n" + strings.Repeat("=", 50))
	// Note: Manual tests moved to test files
}
