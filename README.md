# GoServe

A lightweight, feature-rich HTTP file server written in Go.

## Features

- 📁 **Directory browsing** - Clean, modern interface with file icons
- 🔍 **Search & filter** - Real-time search with wildcard support (`*.ext`)
- ⬆️ **File upload** - Upload files via web interface (optional)
- ✏️ **File management** - Rename and delete files (optional)
- 🌓 **Dark mode** - Toggle between light and dark themes
- 👁️ **File preview** - Preview images, text files, markdown, and code
- 📦 **ZIP download** - Download entire directories as ZIP
- 🗜️ **GZIP compression** - Automatic compression for faster transfers
- 🔒 **Authentication** - Optional user authentication with permission levels
- 🎨 **Breadcrumb navigation** - Easy directory navigation

## Installation

### Prerequisites

- Go 1.16 or higher

### Install Dependencies

```bash
go mod init goserver
go get github.com/russross/blackfriday/v2
```

## Usage

### Basic Usage

Serve files from current directory:

```bash
go run main.go
```

Serve files from a specific directory:

```bash
go run main.go -dir /path/to/folder
```

### Command Line Arguments

| Flag | Default | Description |
|------|---------|-------------|
| `-port` | `8080` | Port to listen on |
| `-dir` | `.` | Directory to serve |
| `-upload` | `false` | Enable file uploads |
| `-modify` | `false` | Enable file deletion and renaming |
| `-maxsize` | `100` | Maximum upload size in MB |
| `-quiet` | `false` | Quiet mode - only show errors |
| `-logins` | `""` | Path to login file for authentication |

### Examples

#### Basic Usage

**Start server on default port (8080):**
```bash
go run main.go
```

**Start server on port 3000:**
```bash
go run main.go -port 3000
```

**Serve specific directory:**
```bash
go run main.go -dir C:\Downloads
```

#### File Management

**Enable uploads (max 50MB):**
```bash
go run main.go -upload -maxsize 50
```

**Enable full file management (upload + delete/rename):**
```bash
go run main.go -upload -modify
```

**Serve directory with uploads:**
```bash
go run main.go -dir /var/www -upload
```

#### Authentication

**Basic authentication:**
```bash
go run main.go -logins logins.txt
```

**Authentication with full permissions:**
```bash
go run main.go -logins logins.txt -upload -modify
```

#### Advanced

**Quiet mode (no request logs):**
```bash
go run main.go -quiet
```

**Combined example (production-ready):**
```bash
go run main.go -port 8000 -dir /var/www -upload -modify -maxsize 200 -logins logins.txt
```

**Serve downloads folder with no write access:**
```bash
go run main.go -dir C:\Downloads -port 9000
```

## Authentication

Enable user authentication by specifying a login file:

```bash
go run go-serve3.go -logins logins.txt -upload -modify
```

### Login File Format

Create a text file with the following format (one user per line):

```
username:password:permission
```

### Permission Levels

| Permission | Browse | View | Upload | Delete | Rename |
|------------|--------|------|--------|--------|--------|
| `readonly` | ✓ | ✓ | ✗ | ✗ | ✗ |
| `readwrite` | ✓ | ✓ | ✓ | ✗ | ✗ |
| `admin` | ✓ | ✓ | ✓ | ✓ | ✓ |

### Example Login File (`logins.txt`)

```
# Admin user with full access
admin:admin123:admin

# Regular user with upload permission
user:password:readwrite

# Guest user with read-only access
guest:guest:readonly
```

- Lines starting with `#` are comments
- Empty lines are ignored
- Format: `username:password:permission`

When authentication is enabled, users will be prompted to log in via HTTP Basic Authentication.

## Search & Filter

The search bar supports wildcard patterns:

- `*.go` - All Go files
- `*.txt` - All text files
- `test*` - Files starting with "test"
- `*config*` - Files containing "config"
- `file?.txt` - Files like `file1.txt`, `fileA.txt`, etc.

Regular substring searches work without wildcards.

## File Preview

Click on supported file types to preview them without downloading:

**Supported formats:**
- **Images**: jpg, jpeg, png, gif, svg, webp
- **Text**: txt, md, json, js, go, py, html, css, xml, log
- **Markdown**: Rendered with syntax highlighting

Press `ESC` or click outside to close the preview.

## Building

### Compile for Current Platform

```bash
go build main.go
```

This creates an executable: `main.exe` (Windows) or `main` (Linux/Mac)

### Cross-Platform Compilation

**Windows (64-bit):**
```bash
go build -o goserve-windows-amd64.exe main.go
```

**Linux (64-bit):**
```bash
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o goserve-linux-amd64 main.go
```

**Linux ARM (Raspberry Pi):**
```bash
$env:GOOS="linux"; $env:GOARCH="arm64"; go build -o goserve-linux-arm64 main.go
```

**Mac (64-bit):**
```bash
$env:GOOS="darwin"; $env:GOARCH="amd64"; go build -o goserve-darwin-amd64 main.go
```

### Build Script

Use the included `build.ps1` script to build for multiple platforms:

```bash
.\build.ps1
```

## Running the Executable

After building, run without Go installed:

```bash
# Windows
.\go-serve3.exe -port 8080 -upload

# Linux/Mac
./go-serve-linux-amd64 -port 8080 -upload
```

## Tips

1. **Security**: Only enable `-upload` and `-modify` flags on trusted networks
2. **Authentication**: Use the `-logins` flag for access control in production
3. **Large files**: Adjust `-maxsize` based on your needs (default 100MB)
4. **Performance**: GZIP compression is automatic for faster transfers
5. **Port conflicts**: If port 8080 is busy, use `-port` to specify another port

## Keyboard Shortcuts

- `ESC` - Close file preview modal

## Browser Support

Works with all modern browsers:
- Chrome/Edge
- Firefox
- Safari
- Opera

## Security Notes

- Directory traversal protection is built-in
- Authentication uses HTTP Basic Auth (use HTTPS in production)
- File operations require appropriate permissions
- By default, server is read-only (safest option)

## License

Open source - feel free to modify and distribute.

## Version

Go-Serve v1.0

---

**Need help?** Run with no arguments to see default settings:
```bash
go run go-serve3.go
```
