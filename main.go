package main

import (
	"crypto/md5"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/js"
)

// AssetManifest stores the mapping between original and fingerprinted filenames
type AssetManifest struct {
	Assets map[string]string `json:"assets"`
}

func main() {
	var (
		sourceDir string
		outputDir string
	)
	flag.StringVar(&sourceDir, "source", "src", "Source directory containing assets")
	flag.StringVar(&outputDir, "output", "src", "Directory to output fingerprinted assets")
	flag.Parse()

	if err := processAssets(sourceDir, outputDir); err != nil {
		log.Fatal(err)
	}
}

// processAssets handles fingerprinting, minifying, and manifest generation for assets
func processAssets(sourceDir, outputDir string) error {
	// remove dist directory to ensure the only fingerprinted files are the one we need
	err := os.RemoveAll(outputDir)
	if err != nil {
		return fmt.Errorf("failed to remove dist directory: %w", err)
	}

	manifestPath := filepath.Join(outputDir, "manifest.json")

	manifest := AssetManifest{
		Assets: make(map[string]string),
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Walk through all files in source directory
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Calculate file hash
		hash, err := calculateFileHash(path)
		if err != nil {
			return fmt.Errorf("failed to calculate hash for %s: %w", path, err)
		}

		// Create fingerprinted filename
		ext := filepath.Ext(relPath)
		baseWithoutExt := strings.TrimSuffix(relPath, ext)
		fingerprintedName := fmt.Sprintf("%s-%s%s", baseWithoutExt, hash[:8], ext)

		// Create output path
		outputPath := filepath.Join(outputDir, fingerprintedName)
		outputDir := filepath.Dir(outputPath)

		// Ensure output directory exists
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", outputDir, err)
		}

		sourceCode, err := openAndReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read source file %s: %w", path, err)
		}

		if ext == ".js" {
			sourceCode, err = minifySource(sourceCode)
			if err != nil {
				return fmt.Errorf("failed to minify source: %w", err)
			}
		}

		// Copy file to output directory
		if err := writeMinifiedFile(sourceCode, outputPath); err != nil {
			return fmt.Errorf("failed to write minified file: %w", err)
		}

		// Add to manifest
		manifest.Assets[relPath] = fingerprintedName
		log.Printf("Processed: %s -> %s", relPath, fingerprintedName)

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to process assets: %w", err)
	}

	// Write manifest file
	manifestFile, err := os.Create(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to create manifest file: %w", err)
	}
	defer manifestFile.Close()

	encoder := json.NewEncoder(manifestFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(manifest); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	log.Printf("Asset manifest written to: %s", manifestPath)
	return nil
}

func openAndReadFile(src string) ([]byte, error) {
	source, err := os.Open(src)
	if err != nil {
		return nil, err
	}
	defer source.Close()

	sourceCode, err := io.ReadAll(source)
	if err != nil {
		return nil, err
	}
	return sourceCode, nil
}

func calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func minifySource(sourceCode []byte) ([]byte, error) {
	m := minify.New()
	m.AddFunc("text/javascript", js.Minify)

	minified, err := m.Bytes("text/javascript", sourceCode)
	if err != nil {
		return nil, err
	}

	return minified, nil
}

func writeMinifiedFile(src []byte, dst string) error {
	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	err = os.WriteFile(dst, src, 0644)
	if err != nil {
		return err
	}
	return nil
}
