# Go-Serve Development Notes

## Project Evolution

### Phase 1: Go Learning & Basics
- Started with Hello World (`My.go`)
- Created number guessing game (`game.go`)
- Learned about `go.mod`, Delve debugger, and DAP server

### Phase 2: Basic HTTP Server
- Built initial file server inspired by Node.js http-server
- Created `server.go` → renamed to `go-serve2.go`
- Added styled UI with gradient header, file tables, icons

### Phase 3: Feature Expansion
- Added dark mode toggle (persisted in localStorage)
- Implemented breadcrumb navigation
- Added search/filter with wildcard support (`*.ext`)
- File upload functionality with size limits
- Delete and rename operations
- File preview modal (images, text, markdown, code)
- ZIP download for directories
- GZIP compression middleware

### Phase 4: Authentication & Security
- Created main.go (formerly go-serve3.go) with optional authentication
- Implemented three permission levels:
  - `readonly`: Browse and download only
  - `readwrite`: Browse, download, upload
  - `admin`: Full access including delete/rename
- Authentication via HTTP Basic Auth
- Credential storage in `logins.txt` (format: `username:password:permission`)

### Phase 5: Polish & Documentation
- Cross-platform build script (`build.ps1`)
- Comprehensive `README.md`
- Custom help with usage examples (`-h` flag)
- Banner: "GoServe v1.0"
- Published to GitHub as v1.0

### Phase 6: Version 1.1 Features (February 2026)
- Added directory upload with structure preservation
- Implemented WebDAV server for network drive mounting
- Enhanced upload handler to support multiple files
- Added `golang.org/x/net/webdav` dependency
- Updated UI with separate buttons for files vs folders
- JavaScript FormData handling for complex uploads
- WebDAV authentication integration

## Key Architectural Decisions

### URL Path Construction: `path.Join` vs `filepath.Join`
**Problem**: Subdirectory file previews broke when clicking files in nested folders.

**Root Cause**: Windows uses backslashes (`\`) for file paths, but URLs always require forward slashes (`/`). `filepath.Join()` uses OS-specific separators.

**Solution**: Use `path.Join()` for all URL construction, `filepath.Join()` only for actual filesystem operations.

```go
// ✅ Correct for URLs
relPath := path.Join(strings.TrimPrefix(r.URL.Path, "/"))

// ❌ Wrong for URLs (breaks on Windows)
relPath := filepath.Join(strings.TrimPrefix(r.URL.Path, "/"))
```

**Impact**: Fixed navigation in subdirectories on Windows.

### Icon Choice: Emojis vs Unicode Symbols
**Evolution**:
1. Started with colorful emojis (📁 📄 🖼️)
2. Tried "professional" Unicode symbols (▪ for generic files)
3. Reverted back to emojis

**Reason**: Unicode symbols like `▪` were too small and hard to see in browser. Emojis provide better visibility and user experience despite being less "professional."

**Final Choice**: Emojis for visibility and usability.

### Port Binding: Listen Before Success Messages
**Problem**: Server printed "Server running on http://localhost:8080" even when port was in use (Exit Code 1 but no error visible).

**Solution**: Create listener with `net.Listen()` first, then pass to `http.Serve()`:

```go
listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
if err != nil {
    if strings.Contains(err.Error(), "address already in use") {
        fmt.Printf("❌ Error: Port %d is already in use.\n", *port)
        fmt.Printf("   Try a different port: go run go-serve3.go -port 3000\n")
    } else {
        log.Fatal(err)
    }
    return
}

// Only print success messages after successful bind
fmt.Println("✅ Server started successfully!")
```

**Impact**: Clear error messages with helpful suggestions when port conflicts occur.

### Authentication: Optional Middleware Pattern
**Design Choice**: Make authentication optional via `-logins` flag.

**Implementation**:
```go
if *loginsFile != "" {
    // Wrap handlers with authMiddleware
    http.HandleFunc("/upload", authMiddleware(handleUpload, "readwrite"))
} else {
    // Direct handlers without auth
    http.HandleFunc("/upload", handleUpload)
}
```

**Benefit**: Single codebase supports both public and authenticated deployments.

### Markdown Rendering: Blackfriday v2
**Choice**: `github.com/russross/blackfriday/v2`

**Reason**: Pure Go implementation, no external dependencies, works well for file previews in browser.

**Usage**: Renders `.md` files in preview modal alongside syntax highlighting for code files.

## Known Issues & Solutions

### Issue: Duplicate Variable Declaration
**Error**: `no new variables on left side of :=`

**Cause**: Variable `user` was declared twice with `:=` in the same scope.

**Solution**: Removed duplicate permission-checking code block in `dirHandler()`.

### Issue: Banner Alignment
**Problem**: Emoji in banner caused misalignment.

**Attempted Solutions**:
1. Adjusted spacing to account for emoji width
2. Tried different emojis
3. Tried Unicode symbols

**Final Solution**: Removed emoji from banner entirely: `"Go-Serve v1.0"`

**Lesson**: Terminal/console emoji rendering is inconsistent across platforms.

### Issue: Wildcard Search
**Challenge**: Users expect `*.txt` pattern matching.

**Solution**: Convert wildcards to regex in JavaScript:
```javascript
const pattern = searchTerm.replace(/\*/g, '.*').replace(/\?/g, '.');
const regex = new RegExp(pattern, 'i');
```

**Impact**: Intuitive search with `*.go`, `test*.txt`, etc.

## Dependencies

### Required
- **Go 1.16+**: For `embed` package support (if used), `io.ReadAll()`, modern http package
- **github.com/russross/blackfriday/v2**: Markdown rendering

### Optional
- None - all other functionality uses Go standard library

## Build & Deployment

### Development
```bash
go run main.go -h           # See help
go run main.go              # Test basic functionality
go run main.go -upload -modify -logins logins.txt  # Test all features
```

### Production Build
```bash
go build main.go            # Single platform
.\build.ps1                      # Multi-platform (Windows/Linux/ARM)
```

### Deployment Targets
- **Windows**: `goserve-windows-amd64.exe`
- **Debian/Linux**: `goserve-linux-amd64`
- **Raspberry Pi**: `goserve-linux-arm64`

## Security Considerations

### Authentication
- Uses HTTP Basic Auth (base64 encoded, not secure over HTTP)
- **Recommendation**: Use with reverse proxy (nginx/caddy) with HTTPS in production
- Credentials stored in plain text (`logins.txt`)
- **Recommendation**: Use environment variables or encrypted storage for production

### File Operations
- Upload size limit configurable via `-maxsize` flag (default: 100MB)
- Path traversal prevention via `filepath.Clean()`
- Permission checks on all file operations

### GZIP Compression
- Automatic for responses > 1KB
- Reduces bandwidth usage
- Slight CPU overhead

## Performance Notes

### File Listing
- Sorts files by type (directories first) then alphabetically
- Efficient for < 10,000 files per directory
- For larger directories, consider pagination

### ZIP Download
- Creates ZIP in memory before sending
- May consume significant RAM for large directories
- **Limitation**: Not suitable for multi-GB directories

### GZIP Middleware
- Compression level: `gzip.BestCompression`
- Minor CPU overhead, significant bandwidth savings
- Automatically skips compression for small responses

## Testing Notes

### Verified Scenarios
✅ Basic file serving on port 8080  
✅ Custom port with `-port 3000`  
✅ Upload with size limits  
✅ Delete/rename operations  
✅ Authentication with all three permission levels  
✅ Dark mode toggle persistence  
✅ Wildcard search (`*.go`, `test*`)  
✅ File previews (images, text, markdown, code)  
✅ ZIP download of directories  
✅ GZIP compression  
✅ Subdirectory navigation (Windows)  
✅ Port conflict detection  
✅ Quiet mode (`-quiet`)  
✅ Build script for multiple platforms  
✅ Directory upload (drag & drop folders) - v1.1
✅ WebDAV support for network drive mounting - v1.1

### v1.1 Release Notes (February 2026)

**New Features:**
- **Directory Upload**: Upload entire folders with preserved directory structure via web interface
- **WebDAV Server**: Mount the file server as a network drive on Windows/Mac/Linux
  - Full PROPFIND, MKCOL, COPY, MOVE support
  - Integrated with existing authentication system
  - Available at `/webdav/` endpoint

**Technical Changes:**
- Added `golang.org/x/net/webdav` dependency
- Modified upload handler to support multiple files with relative paths
- Enhanced JavaScript to handle directory selection with `webkitdirectory`
- Created automatic directory structure on upload
- WebDAV lock system using memory-based LockSystem

### Future Enhancements
- [ ] HTTPS/TLS support with Let's Encrypt
- [ ] Rate limiting to prevent abuse
- [ ] Custom 404 pages
- [ ] Syntax highlighting in code preview
- [ ] File/directory size statistics
- [ ] Session-based auth (JWT) instead of Basic Auth
- [ ] File versioning/backup
- [ ] Resume support for large file uploads
- [ ] Custom themes/branding

## Command Line Reference

### All Flags
```
-port int        Port to listen on (default 8080)
-upload          Enable file uploads
-modify          Enable delete/rename operations
-logins string   Path to logins file for authentication
-maxsize int     Max upload size in MB (default 100)
-quiet           Suppress startup banner
```

### Common Usage Patterns
```bash
# Public read-only server
go run main.go

# Public server with uploads
go run main.go -upload

# Private authenticated server
go run main.go -logins logins.txt -upload -modify

# Production-ready configuration
go run main.go -port 80 -logins logins.txt -upload -modify -maxsize 500
```

## File Structure

```
Go/
├── My.go              # Hello World learning example
├── game.go            # Number guessing game
├── go-serve2.go       # Basic file server (no auth)
├── main.go            # Full-featured server (with auth)
├── logins.txt         # Authentication database
├── build.ps1          # Multi-platform build script
├── README.md          # User documentation
├── DEVELOPMENT.md     # This file - developer notes
├── go.mod             # Go module definition
└── go.sum             # Dependency checksums
```

## Git Workflow (Recommended)

```bash
# Initialize repository
git init

# Add all files
git add .

# Initial commit
git commit -m "Initial commit: Go-Serve v1.0

Features:
- Basic HTTP file server
- Optional authentication (readonly/readwrite/admin)
- File upload/delete/rename
- Dark mode, search, previews
- GZIP compression
- Cross-platform builds"

# Tag version
git tag -a v1.0 -m "Go-Serve version 1.0"
```

## Lessons Learned

1. **Path Separators Matter**: Use `path.Join()` for URLs, `filepath.Join()` for filesystem
2. **Visual Feedback**: Emojis improve UX even if they seem unprofessional
3. **Error Messages**: Clear errors with suggestions are better than technical output
4. **Port Binding**: Check socket availability before announcing success
5. **Permission Model**: Three permission levels cover most use cases
6. **Documentation**: Custom help examples (`-h`) significantly improve discoverability
7. **Optional Features**: Flags for uploads/auth make single binary serve multiple use cases

## Contact & Contributions

This project was developed through iterative learning and refinement. Key decisions were made based on practical testing rather than theoretical best practices.

For questions about architectural decisions, refer to this document first.

---

**Last Updated**: February 9, 2026  
**Version**: 1.1  
**Status**: Production Ready  
**New in v1.1**: Directory Upload, WebDAV Server
