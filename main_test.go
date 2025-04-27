package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcessAssets(t *testing.T) {
	// Create a temporary directory for testing
	sourceDir, err := os.MkdirTemp("", "assetid-source")
	if err != nil {
		t.Fatalf("Failed to create temp source dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(sourceDir)
	})

	outputDir, err := os.MkdirTemp("", "assetid-output")
	if err != nil {
		t.Fatalf("Failed to create temp output dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(outputDir)
	})

	// Create some test files with both JS and CSS
	testFiles := map[string]string{
		"test.js":         "function test() { return 'test'; }",
		"app.js":          "const app = {}; console.log('app loaded');",
		"subdir/util.js":  "function util() { return 'utility'; }",
		"styles.css":      "body { color: #333; font-size: 16px; padding: 20px; margin: 0; }",
		"theme.css":       "header { background-color: #f0f0f0; } footer { color: gray; }",
		"subdir/page.css": ".page { width: 960px; margin: 0 auto; }",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(sourceDir, path)
		dirPath := filepath.Dir(fullPath)

		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dirPath, err)
		}

		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file %s: %v", fullPath, err)
		}
	}

	// Process the assets
	err = processAssets(sourceDir, outputDir)
	if err != nil {
		t.Fatalf("processAssets failed: %v", err)
	}

	// Check that the manifest file was created
	manifestPath := filepath.Join(outputDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("Failed to read manifest file: %v", err)
	}

	var manifest AssetManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("Failed to unmarshal manifest: %v", err)
	}

	// Check that all test files are in the manifest
	if len(manifest.Assets) != len(testFiles) {
		t.Errorf("Expected %d assets in manifest, got %d", len(testFiles), len(manifest.Assets))
	}

	for origPath := range testFiles {
		if _, ok := manifest.Assets[origPath]; !ok {
			t.Errorf("Asset %s not found in manifest", origPath)
		}
	}

	// Check that all fingerprinted files exist in the output directory
	for origPath, fingerprintedName := range manifest.Assets {
		fingerprintedPath := filepath.Join(outputDir, fingerprintedName)
		if _, err := os.Stat(fingerprintedPath); os.IsNotExist(err) {
			t.Errorf("Fingerprinted file %s does not exist", fingerprintedPath)
		}

		// Check that the fingerprinted name contains a hash
		parts := strings.Split(fingerprintedName, "-")
		if len(parts) < 2 {
			t.Errorf("Fingerprinted name %s does not contain a hash", fingerprintedName)
		}

		// Verify the file contents
		origContent := testFiles[origPath]
		fingerprintedContent, err := os.ReadFile(fingerprintedPath)
		if err != nil {
			t.Errorf("Failed to read fingerprinted file %s: %v", fingerprintedPath, err)
			continue
		}

		// JS files should be minified (smaller than original)
		if strings.HasSuffix(origPath, ".js") {
			if len(fingerprintedContent) >= len(origContent) {
				t.Errorf("JavaScript file %s does not appear to be minified", fingerprintedPath)
			}
		}

		// CSS files should NOT be minified (should match original content)
		if strings.HasSuffix(origPath, ".css") {
			// Content should be identical to the original
			if string(fingerprintedContent) != origContent {
				t.Errorf("CSS file %s was modified. Expected unmodified content.", fingerprintedPath)
			}
		}
	}
}

func TestCalculateFileHash(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "assetid-hash-test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	t.Cleanup(func() {
		os.Remove(tmpFile.Name())
	})

	// Write some content to the file
	testContent := "test content for hashing"
	if _, err := tmpFile.Write([]byte(testContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Calculate the hash
	hash, err := calculateFileHash(tmpFile.Name())
	if err != nil {
		t.Fatalf("calculateFileHash failed: %v", err)
	}

	// Check that the hash is 16 characters (FNV-64)
	if len(hash) != 16 {
		t.Errorf("Expected hash length 16, got %d", len(hash))
	}

	// Check that the hash is consistent
	hash2, err := calculateFileHash(tmpFile.Name())
	if err != nil {
		t.Fatalf("Second calculateFileHash failed: %v", err)
	}
	if hash != hash2 {
		t.Errorf("Hashes do not match for same file: %s vs %s", hash, hash2)
	}

	// Test with a non-existent file
	_, err = calculateFileHash("non-existent-file")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestMinifySource(t *testing.T) {
	// Test data for different file types
	testCases := []struct {
		name            string
		content         string
		expectedMinify  bool
		expectError     bool
		checkForContent string
	}{
		{
			name: "JavaScript with comments and whitespace",
			content: `
				// This is a comment
				function hello() {
					console.log("Hello, world!");
					return {
						message: "Hello",
						value: 42
					};
				}
			`,
			expectedMinify:  true,
			expectError:     false,
			checkForContent: "This is a comment",
		},
		{
			name: "CSS content",
			content: `
				/* This is a CSS comment */
				body {
					font-family: Arial, sans-serif;
					color: #333;
					line-height: 1.6;
					padding: 20px;
				}
			`,
			expectedMinify:  false,
			expectError:     true, // Should error since we're only set up to minify JS
			checkForContent: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a temporary file
			extension := ".js"
			if !tc.expectedMinify {
				extension = ".css"
			}

			tmpFile, err := os.CreateTemp("", "assetid-minify-test"+extension)
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			t.Cleanup(func() {
				os.Remove(tmpFile.Name())
			})

			// Write content to the file
			if _, err := tmpFile.Write([]byte(tc.content)); err != nil {
				t.Fatalf("Failed to write to temp file: %v", err)
			}
			if err := tmpFile.Close(); err != nil {
				t.Fatalf("Failed to close temp file: %v", err)
			}

			// Read the file to get content bytes
			contentBytes, err := os.ReadFile(tmpFile.Name())
			if err != nil {
				t.Fatalf("Failed to read file contents: %v", err)
			}

			// Minify the source
			minified, err := minifySource(contentBytes)

			// Check error expectation
			if tc.expectError && err == nil {
				t.Errorf("Expected minifySource to return an error for %s, but it didn't", extension)
			}

			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error from minifySource: %v", err)
				return
			}

			// Skip further checks if we expected an error
			if tc.expectError {
				return
			}

			// Check minification
			if tc.expectedMinify {
				// Should be smaller than original
				if len(minified) >= len(tc.content) {
					t.Errorf("Minified content is not smaller than original")
				}

				// Comments should be removed
				if tc.checkForContent != "" && strings.Contains(string(minified), tc.checkForContent) {
					t.Errorf("Comments were not removed in minified content")
				}

				// Excessive whitespace should be removed
				if strings.Contains(string(minified), "    ") {
					t.Errorf("Unnecessary whitespace was not removed in minified content")
				}
			}
		})
	}

	// Test with a non-existent file
	_, err := os.ReadFile("non-existent-file")
	if err == nil {
		t.Error("Expected error when reading non-existent file, got nil")
	}
}

// TestCSSNoMinification specifically tests that CSS files are not minified
func TestCSSNoMinification(t *testing.T) {
	// Create a temporary directory for testing
	sourceDir, err := os.MkdirTemp("", "assetid-css-source")
	if err != nil {
		t.Fatalf("Failed to create temp source dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(sourceDir)
	})

	outputDir, err := os.MkdirTemp("", "assetid-css-output")
	if err != nil {
		t.Fatalf("Failed to create temp output dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(outputDir)
	})

	// Create a CSS file with formatting that would be minified if we were minifying CSS
	cssContent := `
/* This is a CSS comment that would be removed by minification */
body {
    font-family: Arial, sans-serif; /* This is an inline comment */
    color: #333333;  /* Hex color that could be shortened to #333 */
    margin:  20px;   /* Extra spaces that would be removed */
    padding: 15px;
}

/* Another block comment */
.container {
    max-width: 1200px;
    margin: 0 auto;
}
`
	cssPath := filepath.Join(sourceDir, "styles.css")
	if err := os.WriteFile(cssPath, []byte(cssContent), 0644); err != nil {
		t.Fatalf("Failed to write CSS file: %v", err)
	}

	// Process the assets
	err = processAssets(sourceDir, outputDir)
	if err != nil {
		t.Fatalf("processAssets failed: %v", err)
	}

	// Get the fingerprinted filename from the manifest
	manifestPath := filepath.Join(outputDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("Failed to read manifest file: %v", err)
	}

	var manifest AssetManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("Failed to unmarshal manifest: %v", err)
	}

	fingerprintedName, ok := manifest.Assets["styles.css"]
	if !ok {
		t.Fatalf("CSS file not found in manifest")
	}

	// Read the fingerprinted file
	fingerprintedPath := filepath.Join(outputDir, fingerprintedName)
	processedContent, err := os.ReadFile(fingerprintedPath)
	if err != nil {
		t.Fatalf("Failed to read fingerprinted CSS file: %v", err)
	}

	// Verify that CSS is completely unchanged
	if string(processedContent) != cssContent {
		t.Errorf("CSS content was modified. Expected it to be unchanged.")
		t.Errorf("Original:\n%s", cssContent)
		t.Errorf("Processed:\n%s", string(processedContent))
	}

	// Check specifically that comments are still present
	if !strings.Contains(string(processedContent), "/* This is a CSS comment") {
		t.Errorf("CSS comments were removed, but should be preserved")
	}

	// Check that whitespace is preserved
	if !strings.Contains(string(processedContent), "    font-family") {
		t.Errorf("CSS indentation was modified, but should be preserved")
	}

	// Check that extra spaces are preserved
	if !strings.Contains(string(processedContent), "margin:  20px") {
		t.Errorf("CSS extra spaces were removed, but should be preserved")
	}
}

func TestWriteMinifiedFile(t *testing.T) {
	// Create a directory relative to the test file
	testDir := "./test-assets-output"

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Clean up after the test
	t.Cleanup(func() {
		os.RemoveAll(testDir)
	})

	// Test data
	testContent := []byte("minified content")
	testPath := filepath.Join(testDir, "test-output.js")

	// Write the file
	err := writeMinifiedFile(testContent, testPath)
	if err != nil {
		t.Fatalf("writeMinifiedFile failed: %v", err)
	}

	// Check that the file exists
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Errorf("Output file %s does not exist", testPath)
	}

	// Check the file content
	readContent, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}
	if string(readContent) != string(testContent) {
		t.Errorf("File content mismatch. Expected %q, got %q", testContent, readContent)
	}

	// Check file permissions
	info, err := os.Stat(testPath)
	if err != nil {
		t.Fatalf("Failed to stat output file: %v", err)
	}

	// Verify file permissions (since we're controlling the directory ourselves)
	if info.Mode().Perm() != 0644 {
		t.Errorf("Expected file permissions 0644, got %d", info.Mode().Perm())
	}

	// Test writing to a directory that doesn't exist
	nonExistentDir := filepath.Join(testDir, "non-existent", "dir", "file.js")
	err = writeMinifiedFile(testContent, nonExistentDir)
	if err == nil {
		// The parent directories should be created by the calling function (processAssets),
		// not by writeMinifiedFile, so we expect an error here
		t.Error("Expected error when writing to non-existent directory, got nil")
	}
}
