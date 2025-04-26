package assetid

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestNewLoader(t *testing.T) {
	// Create test cases
	tests := []struct {
		name         string
		manifestFS   fstest.MapFS
		manifestPath string
		wantErr      bool
		assets       map[string]string
	}{
		{
			name: "valid manifest",
			manifestFS: fstest.MapFS{
				"manifest.json": &fstest.MapFile{
					Data: []byte(`{"assets":{"app.js":"app-12345678.js","style.css":"style-87654321.css"}}`),
				},
			},
			manifestPath: "manifest.json",
			wantErr:      false,
			assets: map[string]string{
				"app.js":    "app-12345678.js",
				"style.css": "style-87654321.css",
			},
		},
		{
			name:         "file not found",
			manifestFS:   fstest.MapFS{},
			manifestPath: "manifest.json",
			wantErr:      true,
		},
		{
			name: "invalid json",
			manifestFS: fstest.MapFS{
				"manifest.json": &fstest.MapFile{
					Data: []byte(`{"assets":invalid_json}`),
				},
			},
			manifestPath: "manifest.json",
			wantErr:      true,
		},
	}

	// Run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader, err := NewLoader(tt.manifestFS, tt.manifestPath)

			// Check error expectations
			if (err != nil) != tt.wantErr {
				t.Errorf("NewLoader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Skip further checks if we expected an error
			if tt.wantErr {
				return
			}

			// Verify assets were loaded correctly
			if len(loader.manifest.Assets) != len(tt.assets) {
				t.Errorf("NewLoader() manifest has %d assets, want %d",
					len(loader.manifest.Assets), len(tt.assets))
			}

			// Check each asset
			for key, expectedValue := range tt.assets {
				if actualValue, ok := loader.manifest.Assets[key]; !ok || actualValue != expectedValue {
					t.Errorf("NewLoader() manifest[%s] = %s, want %s",
						key, actualValue, expectedValue)
				}
			}
		})
	}
}

func TestLoader_Path(t *testing.T) {
	// Create a loader with a test manifest
	loader := &Loader{
		manifest: AssetManifest{
			Assets: map[string]string{
				"app.js":    "app-12345678.js",
				"style.css": "style-87654321.css",
			},
		},
	}

	// Create test cases
	tests := []struct {
		name      string
		assetPath string
		want      string
	}{
		{
			name:      "fingerprinted js asset",
			assetPath: "app.js",
			want:      filepath.Join("/dist", "app-12345678.js"),
		},
		{
			name:      "fingerprinted css asset",
			assetPath: "style.css",
			want:      filepath.Join("/dist", "style-87654321.css"),
		},
		{
			name:      "non-existent asset",
			assetPath: "unknown.js",
			want:      filepath.Join("/dist", "unknown.js"),
		},
	}

	// Run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := loader.Path(tt.assetPath); got != tt.want {
				t.Errorf("Loader.Path() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConcurrentAccess(t *testing.T) {
	// Create a test manifest file
	manifest := AssetManifest{
		Assets: map[string]string{
			"app.js": "app-12345678.js",
		},
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Failed to marshal manifest: %v", err)
	}

	fs := fstest.MapFS{
		"manifest.json": &fstest.MapFile{
			Data: data,
		},
	}

	// Create loader
	loader, err := NewLoader(fs, "manifest.json")
	if err != nil {
		t.Fatalf("Failed to create loader: %v", err)
	}

	// Test concurrent access
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func() {
			// Access the path concurrently
			path := loader.Path("app.js")
			if path != filepath.Join("/dist", "app-12345678.js") {
				t.Errorf("Concurrent Loader.Path() = %v, want %v",
					path, filepath.Join("/dist", "app-12345678.js"))
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 100; i++ {
		<-done
	}
}

// TestWithRealFS tests the loader with a real filesystem
func TestWithRealFS(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "assetid-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	// Create a test manifest
	manifest := AssetManifest{
		Assets: map[string]string{
			"app.js":    "app-12345678.js",
			"style.css": "style-87654321.css",
		},
	}

	// Write manifest to file
	manifestPath := filepath.Join(tempDir, "manifest.json")
	file, err := os.Create(manifestPath)
	if err != nil {
		t.Fatalf("Failed to create manifest file: %v", err)
	}

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(manifest); err != nil {
		file.Close()
		t.Fatalf("Failed to write manifest: %v", err)
	}
	file.Close()

	// Open the directory as an fs.FS
	dirFS := os.DirFS(tempDir)

	// Create loader
	loader, err := NewLoader(dirFS, "manifest.json")
	if err != nil {
		t.Fatalf("Failed to create loader: %v", err)
	}

	// Test path resolution
	if got := loader.Path("app.js"); got != filepath.Join("/dist", "app-12345678.js") {
		t.Errorf("Loader.Path() = %v, want %v", got, filepath.Join("/dist", "app-12345678.js"))
	}

	if got := loader.Path("unknown.js"); got != filepath.Join("/dist", "unknown.js") {
		t.Errorf("Loader.Path() = %v, want %v", got, filepath.Join("/dist", "unknown.js"))
	}
}

