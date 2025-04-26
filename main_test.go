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

	// Create some test files
	testFiles := map[string]string{
		"test.js":        "function test() { return 'test'; }",
		"app.js":         "const app = {}; console.log('app loaded');",
		"subdir/util.js": "function util() { return 'utility'; }",
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

		// Verify file was minified
		origContent := testFiles[origPath]
		fingerprintedContent, err := os.ReadFile(fingerprintedPath)
		if err != nil {
			t.Errorf("Failed to read fingerprinted file %s: %v", fingerprintedPath, err)
			continue
		}

		if len(fingerprintedContent) >= len(origContent) && strings.Contains(origPath, ".js") {
			t.Errorf("File %s does not appear to be minified", fingerprintedPath)
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

	// Check that the hash is 32 characters (MD5)
	if len(hash) != 32 {
		t.Errorf("Expected hash length 32, got %d", len(hash))
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
	// Create a temporary file with JavaScript content
	tmpFile, err := os.CreateTemp("", "assetid-minify-test.js")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write some JavaScript content with whitespace and comments
	jsContent := `
		// This is a comment
		function hello() {
			console.log("Hello, world!");
			return {
				message: "Hello",
				value: 42
			};
		}
	`
	if _, err := tmpFile.Write([]byte(jsContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Minify the source
	minified, err := minifySource(tmpFile.Name())
	if err != nil {
		t.Fatalf("minifySource failed: %v", err)
	}

	// Check that the minified content is smaller than the original
	if len(minified) >= len(jsContent) {
		t.Errorf("Minified content is not smaller than original")
	}

	// Check that comments were removed
	if strings.Contains(string(minified), "This is a comment") {
		t.Errorf("Comments were not removed in minified content")
	}

	// Check that unnecessary whitespace was removed
	if strings.Contains(string(minified), "    ") {
		t.Errorf("Unnecessary whitespace was not removed in minified content")
	}

	// Test with a non-JavaScript file (should still work but might not minify much)
	txtFile, err := os.CreateTemp("", "assetid-minify-test.txt")
	if err != nil {
		t.Fatalf("Failed to create temp txt file: %v", err)
	}
	defer os.Remove(txtFile.Name())

	txtContent := "This is plain text content."
	if _, err := txtFile.Write([]byte(txtContent)); err != nil {
		t.Fatalf("Failed to write to temp txt file: %v", err)
	}
	if err := txtFile.Close(); err != nil {
		t.Fatalf("Failed to close temp txt file: %v", err)
	}

	// Try to minify non-JavaScript content
	_, err = minifySource(txtFile.Name())
	if err != nil {
		t.Logf("Note: minifySource returned error for non-JS file: %v", err)
	}

	// Test with a non-existent file
	_, err = minifySource("non-existent-file")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestWriteMinifiedFile(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "assetid-write-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	// Test data
	testContent := []byte("minified content")
	testPath := filepath.Join(tmpDir, "test-output.js")

	// Write the file
	err = writeMinifiedFile(testContent, testPath)
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
	if info.Mode().Perm() != 0755 {
		t.Errorf("Expected file permissions 0755, got %v", info.Mode().Perm())
	}

	// Test writing to a directory that doesn't exist
	nonExistentDir := filepath.Join(tmpDir, "non-existent", "dir", "file.js")
	err = writeMinifiedFile(testContent, nonExistentDir)
	if err == nil {
		// The parent directories should be created by the calling function (processAssets),
		// not by writeMinifiedFile, so we expect an error here
		t.Error("Expected error when writing to non-existent directory, got nil")
	}
}

