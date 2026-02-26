# GoServe

A lightweight, feature-rich HTTP file server written in Go with WebDAV support. Single binary, 12 themes, no dependencies. Repatriate your Cloud Drives. 

## Features

- **Directory browsing** — Clean, modern interface with file icons and sortable columns
- **Search & filter** — Real-time search with wildcard support (`*.ext`, `test*`)
- **File upload** — Upload single files, multiple files, or entire folders
- **File management** — Rename, delete, and edit text files with syntax highlighting
- **File preview** — Preview images, text, markdown, and code in the browser
- **12 themes** — Catppuccin, Dracula, Nord, Solarized, Gruvbox, and more
- **ZIP download** — Download entire directories as ZIP archives
- **GZIP compression** — Automatic response compression
- **WebDAV server** — Mount as a network drive on Windows, macOS, or Linux
- **Authentication** — Optional per-user auth with permission levels
- **Single binary** — All HTML, CSS, and JS embedded. ~8 MB, cross-platform

## Install

**Linux / macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/staceyw/goserve/main/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
iex (irm https://raw.githubusercontent.com/staceyw/goserve/main/scripts/install.ps1)
```

Or download a binary from [Releases](https://github.com/staceyw/goserve/releases).

### Build from source

```bash
go build -ldflags="-s -w" -o goserve main.go
```

## Usage

```bash
# Serve current directory (read-only)
./goserve

# Serve a specific directory
./goserve -dir /path/to/folder

# Enable uploads
./goserve -permlevel readwrite

# Full file management (upload + delete/rename)
./goserve -permlevel all

# Custom listener address
./goserve -listen :3000

# Multiple listeners
./goserve -listen :8080 -listen 192.168.1.10:9090
```

## Command Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-listen` | `localhost:8080` | Address to listen on (repeatable) |
| `-dir` | `.` | Directory to serve |
| `-permlevel` | `readonly` | Permission level: `readonly`, `readwrite`, `all` |
| `-maxsize` | `100` | Max upload size in MB |
| `-logins` | | Path to authentication file |
| `-quiet` | `false` | Suppress request logs |

### Permission Levels

| Level | Browse | View | Upload | Delete | Rename | Edit |
|-------|--------|------|--------|--------|--------|------|
| `readonly` | yes | yes | — | — | — | — |
| `readwrite` | yes | yes | yes | — | — | — |
| `all` | yes | yes | yes | yes | yes | yes |

## Authentication

For per-user permissions, create a login file and use `-logins`:

```bash
./goserve -logins logins.txt
```

### Login file format

```
# format: username:password:permission
all:all123:all
user:password:readwrite
guest:guest:readonly
```

See [docs/logins.sample.txt](docs/logins.sample.txt) for an example.

When `-permlevel` is set to anything other than `readonly`, the `-logins` flag is ignored.

## WebDAV

GoServe includes a built-in WebDAV server at `/webdav/`.

**Windows:** File Explorer > Map network drive > `http://localhost:8080/webdav/`

**macOS:** Finder > Go > Connect to Server > `http://localhost:8080/webdav/`

**Linux:**
```bash
sudo mount -t davfs http://localhost:8080/webdav/ /mnt/goserve
```

## Tailscale Sharing

```bash
# Private (tailnet only)
./goserve &
tailscale serve --bg 8080

# Public HTTPS tunnel (use with -logins for auth)
./goserve -logins logins.txt &
tailscale funnel --bg 8080
```

## Building

Use the build script to cross-compile for all platforms:

```powershell
.\scripts\build.ps1
```

Produces: `goserve-windows-amd64.exe`, `goserve-linux-amd64`, `goserve-linux-arm64`, `goserve-darwin-arm64`

## License

MIT
