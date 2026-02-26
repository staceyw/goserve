# Scripts

| Script | Description |
|--------|-------------|
| `build.ps1` | Local dev build. Cross-compiles Windows, Linux, and macOS binaries into the repo root. |
| `release.ps1` | Cross-compiles into `dist/`, creates a GitHub release, and uploads all binaries. Use `-DryRun` to build without uploading. |
| `clean-releases.ps1` | Deletes all old GitHub releases and tags, keeping only the latest. |
| `install.ps1` | End-user Windows installer. Downloads the latest release binary into the current directory. |
| `install.sh` | End-user Linux/macOS installer. Same as above, supports curl and wget. |

## Usage

```powershell
# Dev build (outputs binaries to repo root)
.\scripts\build.ps1

# Create a release (prompts for tag if omitted)
.\scripts\release.ps1 v1.4.0

# Build only, skip GitHub upload
.\scripts\release.ps1 v1.4.0 -DryRun

# Remove old releases
.\scripts\clean-releases.ps1
```

### Install scripts (run by end users)

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/staceyw/goserve/main/scripts/install.sh | bash
```

```powershell
# Windows
iex (irm https://raw.githubusercontent.com/staceyw/goserve/main/scripts/install.ps1)
```
