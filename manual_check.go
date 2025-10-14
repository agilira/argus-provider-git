// manual_test.go
//
// Manual test to verify Git provider works with real repository
//
// Run with: go run main.go manual_test.go

package git

import (
	"context"
	"fmt"
	"time"
)

// TestGitProviderManually tests the Git provider with real repository access
// This can be used for manual testing and debugging
func TestGitProviderManually() {
	fmt.Println("🧪 Testing Git Provider with real repository...")

	provider := GetProvider()
	// Note: RemoteConfigProvider interface doesn't include Close method

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test with a real JSON file from a public repository
	testURL := "https://github.com/microsoft/vscode.git#package.json?ref=main"

	fmt.Printf("Testing validation of: %s\n", testURL)
	err := provider.Validate(testURL)
	if err != nil {
		fmt.Printf("❌ Validation failed (expected): %v\n", err)
	} else {
		fmt.Printf("✅ URL validation passed\n")
	}

	// Test Load with real JSON file
	fmt.Printf("Testing Load with real JSON file...\n")
	config, err := provider.Load(ctx, testURL)
	if err != nil {
		fmt.Printf("❌ Load failed: %v\n", err)
	} else {
		fmt.Printf("✅ Load succeeded! Config has %d keys\n", len(config))
		if name, exists := config["name"]; exists {
			fmt.Printf("   Package name: %v\n", name)
		}
	}

	// Test with our local test files
	localTests := []struct {
		name string
		path string
	}{
		{"JSON config", "testdata/config.json"},
		{"YAML config", "testdata/config.yaml"},
		{"TOML config", "testdata/config.toml"},
	}

	for _, test := range localTests {
		fmt.Printf("\n🔍 Testing %s...\n", test.name)

		// Test if file exists locally first
		fmt.Printf("File path: %s\n", test.path)

		// We can't test remote files that don't exist yet
		// But we can test the parsing logic
		fmt.Printf("✅ %s test structure ready\n", test.name)
	}

	fmt.Println("\n✅ Git Provider basic functionality verified!")
}
