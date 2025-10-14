// test_local_load.go
//
// Simple test to verify Load functionality works with local repository
//
// Run with: go run test_local_load.go

package main

import (
	"context"
	"fmt"
	"time"

	git "github.com/agilira/argus-provider-git"
)

func main() {
	fmt.Println("🧪 Testing Git Provider Load functionality...")

	provider := git.GetProvider()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test URL validation first - using VS Code repository package.json
	testURL := "https://github.com/microsoft/vscode.git#package.json?ref=main"
	fmt.Printf("Testing validation: %s\n", testURL)

	err := provider.Validate(testURL)
	if err != nil {
		fmt.Printf("❌ Validation failed: %v\n", err)
		return
	}

	fmt.Printf("✅ Validation passed\n")

	// Test Load functionality
	fmt.Printf("Testing Load functionality...\n")
	config, err := provider.Load(ctx, testURL)
	if err != nil {
		fmt.Printf("❌ Load failed: %v\n", err)
		return
	}

	fmt.Printf("✅ Load succeeded! Config keys: %d\n", len(config))
	if len(config) > 0 {
		fmt.Println("Configuration content:")
		for k, v := range config {
			fmt.Printf("  %s: %v\n", k, v)
		}
	}

	fmt.Println("\n✅ Local Load functionality verified!")
}
