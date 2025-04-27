# AssetID

AssetID is a Go-based asset fingerprinting tool and library that helps you manage static assets for web applications. It generates content-based hashes for your JavaScript and other static assets, creating unique filenames that facilitate long-term caching strategies while ensuring users always get the latest version when content changes.

## Features

- File fingerprinting with content-based FNV hashing from the standard library
- JavaScript minification using tdewolff/minify
- CSS files are fingerprinted but not minified (preserves formatting and comments)
- Manifest generation for mapping original filenames to fingerprinted versions
- Library for resolving fingerprinted assets in Go applications
- Simple command-line interface for build-time integration
- Lightweight with minimal dependencies

## Command Line Usage

AssetID can be used as a standalone command-line tool to process your static assets during build time.

### Installation

```bash
go install github.com/jm96441n/assetid@latest
```

This will install the `assetid` binary to your `$GOPATH/bin` directory.

### Basic Usage

```bash
assetid --source ./src/assets --output ./dist
```

Options:
- `--source`: Directory containing source assets (default: "src")
- `--output`: Directory for fingerprinted output files (default: "src")

### How It Works

1. AssetID processes files in the source directory
2. Each file is hashed using FNV-64a (Fowler-Noll-Vo) based on its content
3. JavaScript files are minified
4. Files are saved with fingerprinted names using the full 16-character hash (e.g., `app-a1b2c3d4e5f67890.js`)
5. A `manifest.json` file is created in the output directory

Example manifest:
```json
{
  "assets": {
    "app.js": "app-a1b2c3d4e5f67890.js",
    "style.css": "style-0123456789abcdef.css"
  }
}
```

## Using as a Library

You can also use AssetID as a library within your Go application to resolve fingerprinted asset paths.

### Installation

```bash
go get github.com/jm96441n/assetid
```

### Basic Usage

```go
package main

import (
    "fmt"
    "log"
    "os"

    "github.com/jm96441n/assetid/assetid"
)

func main() {
    // Initialize the loader with the manifest file
    fs := os.DirFS("./dist")
    loader, err := assetid.NewLoader(fs, "manifest.json")
    if err != nil {
        log.Fatalf("Failed to load asset manifest: %v", err)
    }

    // Get the fingerprinted path for an asset
    jsPath := loader.Path("app.js")
    fmt.Println(jsPath) // Output: /dist/app-a1b2c3d4e5f67890.js
    
    // Use in a web application
    // http.ServeFile(w, r, "." + jsPath)
}
```

### Integration with Web Frameworks

When using AssetID with web frameworks, you can inject the loader into your handlers:

```go
package main

import (
    "html/template"
    "log"
    "net/http"
    "os"

    "github.com/jm96441n/assetid/assetid"
)

// App holds application dependencies
type App struct {
    assets *assetid.Loader
    tmpl   *template.Template
}

func main() {
    // Initialize asset loader
    fs := os.DirFS("./dist")
    assets, err := assetid.NewLoader(fs, "manifest.json")
    if err != nil {
        log.Fatalf("Failed to load asset manifest: %v", err)
    }
    
    // Create template with custom function for asset paths
    tmpl := template.New("index").Funcs(template.FuncMap{
        "asset": func(path string) string {
            return assets.Path(path)
        },
    })
    
    // Parse template
    tmpl, err = tmpl.Parse(`
        <!DOCTYPE html>
        <html>
        <head>
            <script src="{{asset "app.js"}}"></script>
        </head>
        <body>
            <h1>Hello, AssetID!</h1>
        </body>
        </html>
    `)
    if err != nil {
        log.Fatalf("Failed to parse template: %v", err)
    }
    
    // Create app with dependencies
    app := &App{
        assets: assets,
        tmpl:   tmpl,
    }
    
    // Handle requests
    http.HandleFunc("/", app.indexHandler)
    
    // Static file serving
    http.Handle("/dist/", http.StripPrefix("/dist/", http.FileServer(http.Dir("./dist"))))
    
    log.Println("Server started at http://localhost:8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

// indexHandler renders the index template
func (a *App) indexHandler(w http.ResponseWriter, r *http.Request) {
    a.tmpl.Execute(w, nil)
}
```

## Thread Safety

The AssetID library is thread-safe and can be safely used in concurrent applications.

## Contributing

Contributions to AssetID are welcome! Here's how you can contribute:

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-new-feature`
3. Make your changes and add tests
4. Run tests: `go test ./...`
5. Commit your changes: `git commit -am 'Add some feature'`
6. Push to the branch: `git push origin feature/my-new-feature`
7. Submit a pull request

### Development Setup

1. Clone the repository:
   ```bash
   git clone https://github.com/jm96441n/assetid.git
   cd assetid
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Run tests:
   ```bash
   go test ./...
   ```

### Guidelines

- Write tests for new features and bug fixes
- Follow Go code style and best practices
- Update documentation as needed
- Add a description of your changes to the pull request

## License

[MIT License](LICENSE)