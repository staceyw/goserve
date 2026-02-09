package main

import (
	"archive/zip"
	"compress/gzip"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/russross/blackfriday/v2"
)

type FileInfo struct {
	Name    string
	Path    string
	Size    string
	ModTime string
	IsDir   bool
	Icon    string
}

type PageData struct {
	Path        string
	FullPath    string
	Files       []FileInfo
	ShowParent  bool
	Breadcrumbs []Breadcrumb
	CanUpload   bool
	CanModify   bool
}

type Breadcrumb struct {
	Name string
	Path string
}

type User struct {
	Username   string
	Password   string
	Permission string // readonly, readwrite, admin
}

var (
	maxUploadSize int64
	allowUpload   bool
	allowModify   bool
	users         map[string]User
	requireAuth   bool
)

const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Go-Serve - {{.Path}}</title>
    <style>
        :root {
            --bg-primary: #f5f5f5;
            --bg-secondary: white;
            --bg-header: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            --text-primary: #495057;
            --text-secondary: #868e96;
            --border-color: #dee2e6;
            --hover-bg: #f8f9fa;
            --accent: #667eea;
        }
        [data-theme="dark"] {
            --bg-primary: #1a1a1a;
            --bg-secondary: #2d2d2d;
            --bg-header: linear-gradient(135deg, #4a5568 0%, #2d3748 100%);
            --text-primary: #e2e8f0;
            --text-secondary: #a0aec0;
            --border-color: #4a5568;
            --hover-bg: #3d3d3d;
            --accent: #818cf8;
        }
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: var(--bg-primary);
            color: var(--text-primary);
            padding: 20px;
            transition: background 0.3s, color 0.3s;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background: var(--bg-secondary);
            border-radius: 8px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
            overflow: hidden;
        }
        header {
            background: var(--bg-header);
            color: white;
            padding: 30px;
            position: relative;
        }
        .header-top {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 15px;
        }
        .title { font-size: 13px; opacity: 0.9; }
        .controls { display: flex; gap: 10px; }
        .btn {
            background: rgba(255,255,255,0.2);
            border: 1px solid rgba(255,255,255,0.3);
            color: white;
            padding: 6px 12px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 13px;
            transition: all 0.2s;
        }
        .btn:hover {
            background: rgba(255,255,255,0.3);
        }
        h1 { font-size: 24px; margin-bottom: 10px; }
        .breadcrumb {
            display: flex;
            gap: 8px;
            flex-wrap: wrap;
            font-size: 14px;
            opacity: 0.9;
        }
        .breadcrumb a {
            color: white;
            text-decoration: none;
            transition: opacity 0.2s;
        }
        .breadcrumb a:hover { opacity: 0.7; }
        .breadcrumb span { opacity: 0.6; }
        .toolbar {
            padding: 15px 20px;
            border-bottom: 1px solid var(--border-color);
            display: flex;
            gap: 15px;
            flex-wrap: wrap;
            align-items: center;
        }
        .search-box {
            flex: 1;
            min-width: 200px;
            padding: 8px 12px;
            border: 1px solid var(--border-color);
            border-radius: 4px;
            background: var(--bg-secondary);
            color: var(--text-primary);
            font-size: 14px;
        }
        .upload-form {
            display: flex;
            gap: 10px;
            align-items: center;
        }
        .file-input {
            padding: 6px;
            border: 1px solid var(--border-color);
            border-radius: 4px;
            background: var(--bg-secondary);
            color: var(--text-primary);
            font-size: 13px;
        }
        .btn-primary {
            background: var(--accent);
            border: none;
            color: white;
            padding: 8px 16px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 13px;
            font-weight: 500;
        }
        .btn-primary:hover { opacity: 0.9; }
        table {
            width: 100%;
            border-collapse: collapse;
        }
        thead {
            background: var(--hover-bg);
            border-bottom: 2px solid var(--border-color);
        }
        th {
            text-align: left;
            padding: 15px 20px;
            font-weight: 600;
            color: var(--text-primary);
            font-size: 14px;
            cursor: pointer;
            user-select: none;
        }
        th:hover { background: var(--border-color); }
        td {
            padding: 12px 20px;
            border-bottom: 1px solid var(--border-color);
        }
        tr:hover { background: var(--hover-bg); }
        .icon {
            font-size: 20px;
            margin-right: 10px;
            display: inline-block;
            width: 24px;
            text-align: center;
        }
        .file-link {
            color: var(--text-primary);
            text-decoration: none;
            display: flex;
            align-items: center;
            flex: 1;
        }
        .file-link:hover { color: var(--accent); }
        .name { font-weight: 500; }
        .size, .modified { color: var(--text-secondary); font-size: 14px; }
        .actions {
            display: flex;
            gap: 8px;
        }
        .action-btn {
            padding: 4px 8px;
            font-size: 12px;
            border: 1px solid var(--border-color);
            background: var(--bg-secondary);
            color: var(--text-primary);
            border-radius: 3px;
            cursor: pointer;
        }
        .action-btn:hover { background: var(--hover-bg); }
        .action-btn.danger:hover { background: #dc3545; color: white; border-color: #dc3545; }
        .parent { background: rgba(255, 243, 205, 0.3); }
        footer {
            padding: 20px;
            text-align: center;
            color: var(--text-secondary);
            font-size: 13px;
            border-top: 1px solid var(--border-color);
        }
        .preview-modal {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: rgba(0,0,0,0.8);
            z-index: 1000;
            padding: 40px;
        }
        .preview-content {
            background: var(--bg-secondary);
            border-radius: 8px;
            max-width: 900px;
            max-height: 90vh;
            margin: 0 auto;
            overflow: auto;
            padding: 30px;
        }
        .preview-close {
            float: right;
            font-size: 28px;
            cursor: pointer;
            color: var(--text-secondary);
        }
        .preview-close:hover { color: var(--text-primary); }
        .markdown-body { line-height: 1.6; }
        .markdown-body h1, .markdown-body h2 { margin-top: 24px; margin-bottom: 16px; }
        .markdown-body pre { background: var(--hover-bg); padding: 16px; border-radius: 6px; overflow: auto; }
        .markdown-body code { background: var(--hover-bg); padding: 2px 6px; border-radius: 3px; }
        .hidden { display: none !important; }
        @media (max-width: 768px) {
            .modified { display: none; }
            .actions { display: none; }
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <div class="header-top">
                <div class="title">GoServe v1.0</div>
                <div class="controls">
                    <button class="btn" onclick="toggleTheme()">◐ Theme</button>
                    {{if .Path}}
                    <button class="btn" onclick="downloadZip()">▣ Download ZIP</button>
                    {{end}}
                </div>
            </div>
            <div class="breadcrumb">
                <a href="/">◈ Home</a>
                {{range .Breadcrumbs}}
                    <span>/</span>
                    <a href="{{.Path}}">{{.Name}}</a>
                {{end}}
            </div>
        </header>
        
        <div class="toolbar">
            <input type="text" class="search-box" id="searchBox" placeholder="⌕ Search files (supports *.ext wildcards)..." onkeyup="filterFiles()">
            {{if .CanUpload}}
            <form class="upload-form" method="POST" enctype="multipart/form-data" action="?upload=1">
                <input type="file" name="file" class="file-input" multiple required>
                <button type="submit" class="btn-primary">↑ Upload</button>
            </form>
            {{end}}
        </div>

        <table id="fileTable">
            <thead>
                <tr>
                    <th onclick="sortTable(0)">Name ↕</th>
                    <th onclick="sortTable(1)">Size ↕</th>
                    <th class="modified" onclick="sortTable(2)">Modified ↕</th>
                    {{if .CanModify}}
                    <th>Actions</th>
                    {{end}}
                </tr>
            </thead>
            <tbody>
                {{if .ShowParent}}
                <tr class="parent">
                    <td><a href="../" class="file-link"><span class="icon">↑</span><span class="name">..</span></a></td>
                    <td class="size">-</td>
                    <td class="modified">-</td>
                    {{if .CanModify}}<td></td>{{end}}
                </tr>
                {{end}}
                {{range .Files}}
                <tr>
                    <td>
                        <a href="{{.Path}}" class="file-link" {{if not .IsDir}}onclick="return previewFile(event, '{{.Path}}', '{{.Name}}')"{{end}}>
                            <span class="icon">{{.Icon}}</span>
                            <span class="name">{{.Name}}</span>
                        </a>
                    </td>
                    <td class="size">{{.Size}}</td>
                    <td class="modified">{{.ModTime}}</td>
                    {{if $.CanModify}}
                    <td class="actions">
                        <button class="action-btn" onclick="renameFile('{{.Path}}', '{{.Name}}')">✎</button>
                        <button class="action-btn danger" onclick="deleteFile('{{.Path}}', '{{.Name}}')">×</button>
                    </td>
                    {{end}}
                </tr>
                {{end}}
            </tbody>
        </table>

        <footer>Go-Serve - Simple File Server</footer>
    </div>

    <div id="previewModal" class="preview-modal" onclick="closePreview()">
        <div class="preview-content" onclick="event.stopPropagation()">
            <span class="preview-close" onclick="closePreview()">&times;</span>
            <div id="previewBody"></div>
        </div>
    </div>

    <script>
        // Dark mode
        function toggleTheme() {
            const current = document.documentElement.getAttribute('data-theme');
            const next = current === 'dark' ? 'light' : 'dark';
            document.documentElement.setAttribute('data-theme', next);
            localStorage.setItem('theme', next);
        }
        
        // Load saved theme
        const savedTheme = localStorage.getItem('theme') || 'light';
        document.documentElement.setAttribute('data-theme', savedTheme);

        // Search/filter with wildcard support
        function filterFiles() {
            const input = document.getElementById('searchBox');
            const filter = input.value;
            const table = document.getElementById('fileTable');
            const rows = table.getElementsByTagName('tr');
            
            // Check if filter contains wildcards
            const hasWildcard = filter.includes('*') || filter.includes('?');
            let regex = null;
            
            if (hasWildcard) {
                // Convert wildcard pattern to regex
                // Escape special regex chars except * and ?
                let pattern = filter.replace(/[.+^${}()|[\]\\]/g, '\\$&');
                // Convert wildcards: * -> .* and ? -> .
                pattern = pattern.replace(/\*/g, '.*').replace(/\?/g, '.');
                try {
                    regex = new RegExp('^' + pattern + '$', 'i');
                } catch (e) {
                    // Invalid regex, fall back to substring search
                    regex = null;
                }
            }
            
            for (let i = 1; i < rows.length; i++) {
                const nameCell = rows[i].getElementsByClassName('name')[0];
                if (nameCell) {
                    let txtValue = nameCell.textContent || nameCell.innerText;
                    // Remove trailing slash from directories
                    txtValue = txtValue.replace(/\/$/, '');
                    
                    let matches = false;
                    if (regex) {
                        matches = regex.test(txtValue);
                    } else {
                        matches = txtValue.toLowerCase().indexOf(filter.toLowerCase()) > -1;
                    }
                    
                    rows[i].style.display = matches ? '' : 'none';
                }
            }
        }

        // Sort table
        function sortTable(n) {
            const table = document.getElementById('fileTable');
            let switching = true;
            let dir = 'asc';
            let switchcount = 0;
            
            while (switching) {
                switching = false;
                const rows = table.rows;
                
                for (let i = 1; i < (rows.length - 1); i++) {
                    let shouldSwitch = false;
                    const x = rows[i].getElementsByTagName('TD')[n];
                    const y = rows[i + 1].getElementsByTagName('TD')[n];
                    
                    if (dir === 'asc') {
                        if (x.innerHTML.toLowerCase() > y.innerHTML.toLowerCase()) {
                            shouldSwitch = true;
                            break;
                        }
                    } else if (dir === 'desc') {
                        if (x.innerHTML.toLowerCase() < y.innerHTML.toLowerCase()) {
                            shouldSwitch = true;
                            break;
                        }
                    }
                }
                
                if (shouldSwitch) {
                    rows[i].parentNode.insertBefore(rows[i + 1], rows[i]);
                    switching = true;
                    switchcount++;
                } else {
                    if (switchcount === 0 && dir === 'asc') {
                        dir = 'desc';
                        switching = true;
                    }
                }
            }
        }

        // File operations
        function deleteFile(path, name) {
            if (confirm('Delete ' + name + '?')) {
                fetch('?delete=' + encodeURIComponent(path), { method: 'POST' })
                    .then(r => r.json())
                    .then(data => {
                        if (data.success) location.reload();
                        else alert('Error: ' + data.error);
                    });
            }
        }

        function renameFile(path, oldName) {
            const newName = prompt('Rename to:', oldName);
            if (newName && newName !== oldName) {
                fetch('?rename=' + encodeURIComponent(path) + '&newname=' + encodeURIComponent(newName), { method: 'POST' })
                    .then(r => r.json())
                    .then(data => {
                        if (data.success) location.reload();
                        else alert('Error: ' + data.error);
                    });
            }
        }

        function downloadZip() {
            window.location.href = '?zip=1';
        }

        // Preview files
        function previewFile(e, path, name) {
            const ext = name.split('.').pop().toLowerCase();
            const previewable = ['txt', 'md', 'json', 'js', 'go', 'py', 'html', 'css', 'xml', 'log'];
            const images = ['jpg', 'jpeg', 'png', 'gif', 'svg', 'webp'];
            
            if (images.includes(ext)) {
                e.preventDefault();
                document.getElementById('previewBody').innerHTML = '<img src="' + path + '" style="max-width: 100%; height: auto;">';
                document.getElementById('previewModal').style.display = 'block';
                return false;
            } else if (ext === 'md') {
                e.preventDefault();
                fetch(path + '?markdown=1')
                    .then(r => {
                        if (!r.ok) throw new Error('Failed to load');
                        return r.text();
                    })
                    .then(html => {
                        document.getElementById('previewBody').innerHTML = '<div class="markdown-body">' + html + '</div>';
                        document.getElementById('previewModal').style.display = 'block';
                    })
                    .catch(err => alert('Error loading file: ' + err.message));
                return false;
            } else if (previewable.includes(ext)) {
                e.preventDefault();
                fetch(path)
                    .then(r => {
                        if (!r.ok) throw new Error('Failed to load');
                        return r.text();
                    })
                    .then(text => {
                        document.getElementById('previewBody').innerHTML = '<pre>' + escapeHtml(text) + '</pre>';
                        document.getElementById('previewModal').style.display = 'block';
                    })
                    .catch(err => alert('Error loading file: ' + err.message));
                return false;
            }
            return true;
        }

        function closePreview() {
            document.getElementById('previewModal').style.display = 'none';
        }

        function escapeHtml(text) {
            const map = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' };
            return text.replace(/[&<>"']/g, m => map[m]);
        }

        // Close preview with Escape key
        document.addEventListener('keydown', e => {
            if (e.key === 'Escape') closePreview();
        });
    </script>
</body>
</html>`

func loadUsers(filePath string) error {
	users = make(map[string]User)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Split(line, ":")
		if len(parts) != 3 {
			continue
		}

		username := strings.TrimSpace(parts[0])
		password := strings.TrimSpace(parts[1])
		permission := strings.TrimSpace(parts[2])

		users[username] = User{
			Username:   username,
			Password:   password,
			Permission: permission,
		}
	}

	return nil
}

func getUserFromRequest(r *http.Request) *User {
	if !requireAuth {
		return nil
	}

	username, password, ok := r.BasicAuth()
	if !ok {
		return nil
	}

	user, exists := users[username]
	if !exists || user.Password != password {
		return nil
	}

	return &user
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireAuth {
			next(w, r)
			return
		}

		user := getUserFromRequest(r)
		if user == nil {
			w.Header().Set("WWW-Authenticate", `Basic realm="Go-Serve"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

func getIcon(name string, isDir bool) string {
	if isDir {
		return "📁"
	}
	ext := strings.ToLower(filepath.Ext(name))
	icons := map[string]string{
		".html": "🌐", ".htm": "🌐", ".css": "🎨", ".js": "📜", ".ts": "📘",
		".json": "📋", ".xml": "📋", ".yaml": "📋", ".md": "📝", ".txt": "📄",
		".pdf": "📕", ".jpg": "🖼️", ".jpeg": "🖼️", ".png": "🖼️", ".gif": "🖼️",
		".mp4": "🎬", ".mp3": "🎵", ".zip": "📦", ".tar": "📦", ".gz": "📦",
		".go": "🐹", ".py": "🐍", ".java": "☕", ".php": "🐘", ".rb": "💎",
	}
	if icon, ok := icons[ext]; ok {
		return icon
	}
	return "📄"
}

func formatSize(size int64) string {
	if size == 0 {
		return "-"
	}
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func buildBreadcrumbs(urlPath string) []Breadcrumb {
	if urlPath == "/" || urlPath == "" {
		return nil
	}
	parts := strings.Split(strings.Trim(urlPath, "/"), "/")
	crumbs := make([]Breadcrumb, 0, len(parts))
	currentPath := ""
	for _, part := range parts {
		currentPath += "/" + part
		crumbs = append(crumbs, Breadcrumb{Name: part, Path: currentPath})
	}
	return crumbs
}

// GZIP middleware
type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func gzipMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		next(gzipResponseWriter{Writer: gz, ResponseWriter: w}, r)
	}
}

func dirHandler(baseDir string, tmpl *template.Template, quiet bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Log request
		if !quiet {
			fmt.Printf("[%s] %s %s\n", r.Method, r.RemoteAddr, r.URL.Path)
		}

		urlPath := filepath.Clean(r.URL.Path)
		fullPath := filepath.Join(baseDir, urlPath)

		// Security check
		if !strings.HasPrefix(fullPath, baseDir) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Get user and check permissions
		user := getUserFromRequest(r)
		canUpload := allowUpload
		canModify := allowModify

		if requireAuth && user != nil {
			switch user.Permission {
			case "readonly":
				canUpload = false
				canModify = false
			case "readwrite":
				canUpload = allowUpload
				canModify = false
			case "admin":
				canUpload = allowUpload
				canModify = allowModify
			}
		}

		// Handle upload
		if r.URL.Query().Get("upload") != "" && r.Method == "POST" {
			if !canUpload {
				http.Error(w, "Forbidden: Upload not allowed", http.StatusForbidden)
				return
			}
			handleUpload(w, r, fullPath)
			return
		}

		// Handle delete
		if r.URL.Query().Get("delete") != "" && r.Method == "POST" {
			if !canModify {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"success": false, "error": "Forbidden: Delete not allowed"}`)
				return
			}
			handleDelete(w, r, baseDir)
			return
		}

		// Handle rename
		if r.URL.Query().Get("rename") != "" && r.Method == "POST" {
			if !canModify {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"success": false, "error": "Forbidden: Rename not allowed"}`)
				return
			}
			handleRename(w, r, baseDir)
			return
		}

		// Handle ZIP download
		if r.URL.Query().Get("zip") != "" {
			handleZipDownload(w, fullPath, urlPath)
			return
		}

		// Get file info
		info, err := os.Stat(fullPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		// Handle markdown preview
		if !info.IsDir() && r.URL.Query().Get("markdown") != "" {
			handleMarkdownPreview(w, fullPath)
			return
		}

		// If it's a file, serve it
		if !info.IsDir() {
			http.ServeFile(w, r, fullPath)
			return
		}

		// Read directory
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			http.Error(w, "Cannot read directory", http.StatusInternalServerError)
			return
		}

		// Build file list
		var files []FileInfo
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				continue
			}

			name := entry.Name()
			urlPath := path.Join(r.URL.Path, name)
			if entry.IsDir() {
				urlPath += "/"
				name += "/"
			}

			size := ""
			if !entry.IsDir() {
				size = formatSize(info.Size())
			} else {
				size = "-"
			}

			files = append(files, FileInfo{
				Name:    name,
				Path:    urlPath,
				Size:    size,
				ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
				IsDir:   entry.IsDir(),
				Icon:    getIcon(name, entry.IsDir()),
			})
		}

		// Sort: directories first, then by name
		sort.Slice(files, func(i, j int) bool {
			if files[i].IsDir != files[j].IsDir {
				return files[i].IsDir
			}
			return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
		})

		// Render template
		data := PageData{
			Path:        r.URL.Path,
			FullPath:    fullPath,
			Files:       files,
			ShowParent:  r.URL.Path != "/",
			Breadcrumbs: buildBreadcrumbs(r.URL.Path),
			CanUpload:   canUpload,
			CanModify:   canModify,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.Execute(w, data)
	}
}

func handleUpload(w http.ResponseWriter, r *http.Request, targetDir string) {
	r.ParseMultipartForm(maxUploadSize)

	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Upload failed", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Check file size
	if handler.Size > maxUploadSize {
		http.Error(w, "File too large", http.StatusRequestEntityTooLarge)
		return
	}

	// Save file
	dst, err := os.Create(filepath.Join(targetDir, handler.Filename))
	if err != nil {
		http.Error(w, "Cannot save file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Cannot save file", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
}

func handleDelete(w http.ResponseWriter, r *http.Request, baseDir string) {
	path := r.URL.Query().Get("delete")
	fullPath := filepath.Join(baseDir, path)

	if !strings.HasPrefix(fullPath, baseDir) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success": false, "error": "Invalid path"}`)
		return
	}

	err := os.RemoveAll(fullPath)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		fmt.Fprintf(w, `{"success": false, "error": "%s"}`, err.Error())
	} else {
		fmt.Fprintf(w, `{"success": true}`)
	}
}

func handleRename(w http.ResponseWriter, r *http.Request, baseDir string) {
	oldPath := r.URL.Query().Get("rename")
	newName := r.URL.Query().Get("newname")

	oldFullPath := filepath.Join(baseDir, oldPath)
	newFullPath := filepath.Join(filepath.Dir(oldFullPath), newName)

	if !strings.HasPrefix(oldFullPath, baseDir) || !strings.HasPrefix(newFullPath, baseDir) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success": false, "error": "Invalid path"}`)
		return
	}

	err := os.Rename(oldFullPath, newFullPath)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		fmt.Fprintf(w, `{"success": false, "error": "%s"}`, err.Error())
	} else {
		fmt.Fprintf(w, `{"success": true}`)
	}
}

func handleZipDownload(w http.ResponseWriter, fullPath, urlPath string) {
	info, err := os.Stat(fullPath)
	if err != nil || !info.IsDir() {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	zipName := "download.zip"
	if urlPath != "/" && urlPath != "" {
		zipName = filepath.Base(urlPath) + ".zip"
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", zipName))

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(fullPath, path)
		if relPath == "." {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		header.Name = relPath
		header.Method = zip.Deflate

		if info.IsDir() {
			header.Name += "/"
		}

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(writer, file)
			return err
		}

		return nil
	})
}

func handleMarkdownPreview(w http.ResponseWriter, fullPath string) {
	content, err := os.ReadFile(fullPath)
	if err != nil {
		http.Error(w, "Cannot read file", http.StatusInternalServerError)
		return
	}

	html := blackfriday.Run(content)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(html)
}

func main() {
	// Custom usage function with examples
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "GoServe v1.0 - Lightweight HTTP File Server\n\n")
		fmt.Fprintf(os.Stderr, "USAGE:\n")
		fmt.Fprintf(os.Stderr, "  go run main.go [options]\n\n")
		fmt.Fprintf(os.Stderr, "OPTIONS:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEXAMPLES:\n")
		fmt.Fprintf(os.Stderr, "  Basic usage (serve current directory):\n")
		fmt.Fprintf(os.Stderr, "    go run main.go\n\n")
		fmt.Fprintf(os.Stderr, "  Serve on custom port:\n")
		fmt.Fprintf(os.Stderr, "    go run main.go -port 3000\n\n")
		fmt.Fprintf(os.Stderr, "  Serve specific directory:\n")
		fmt.Fprintf(os.Stderr, "    go run main.go -dir C:\\\\Downloads\n\n")
		fmt.Fprintf(os.Stderr, "  Enable file uploads (max 50MB):\n")
		fmt.Fprintf(os.Stderr, "    go run main.go -upload -maxsize 50\n\n")
		fmt.Fprintf(os.Stderr, "  Enable full file management (upload + delete/rename):\n")
		fmt.Fprintf(os.Stderr, "    go run main.go -upload -modify\n\n")
		fmt.Fprintf(os.Stderr, "  With authentication:\n")
		fmt.Fprintf(os.Stderr, "    go run main.go -logins logins.txt -upload -modify\n\n")
		fmt.Fprintf(os.Stderr, "  Quiet mode (no request logs):\n")
		fmt.Fprintf(os.Stderr, "    go run main.go -quiet\n\n")
		fmt.Fprintf(os.Stderr, "  Combined example:\n")
		fmt.Fprintf(os.Stderr, "    go run main.go -port 8000 -dir /var/www -upload -modify -logins logins.txt\n\n")
	}

	// Command line flags
	port := flag.String("port", "8080", "Port to listen on")
	dir := flag.String("dir", ".", "Directory to serve")
	quiet := flag.Bool("quiet", false, "Quiet mode - only show errors")
	uploadFlag := flag.Bool("upload", false, "Allow file uploads")
	modifyFlag := flag.Bool("modify", false, "Allow file deletion and renaming")
	maxSize := flag.Int64("maxsize", 100, "Max upload size in MB")
	loginFile := flag.String("logins", "", "Enable authentication with login file (format: username:password:permission)")
	flag.Parse()

	allowUpload = *uploadFlag
	allowModify = *modifyFlag
	maxUploadSize = *maxSize * 1024 * 1024

	// Load users if authentication is enabled
	if *loginFile != "" {
		err := loadUsers(*loginFile)
		if err != nil {
			log.Fatalf("Failed to load login file: %v", err)
		}
		requireAuth = true
		fmt.Printf("✓ Loaded %d users from %s\n", len(users), *loginFile)
	}

	// Get absolute path
	absPath, err := filepath.Abs(*dir)
	if err != nil {
		log.Fatal(err)
	}

	// Check if directory exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		log.Fatalf("Directory does not exist: %s", absPath)
	}

	// Parse template
	tmpl, err := template.New("index").Parse(htmlTemplate)
	if err != nil {
		log.Fatal(err)
	}

	// Setup handler with authentication and GZIP
	handler := dirHandler(absPath, tmpl, *quiet)
	if requireAuth {
		handler = authMiddleware(handler)
	}
	http.HandleFunc("/", gzipMiddleware(handler))

	// Try to create listener first
	addr := ":" + *port
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Printf("\n❌ ERROR: Cannot start server on port %s\n", *port)
		if strings.Contains(err.Error(), "address already in use") ||
			strings.Contains(err.Error(), "Only one usage") {
			fmt.Printf("   Port %s is already in use by another application.\n", *port)
			fmt.Println("   Try a different port with: go run server3.go -port 3000")
		} else {
			fmt.Printf("   %v\n", err)
		}
		fmt.Println()
		os.Exit(1)
	}

	// Display startup info
	fmt.Println("╔════════════════════════════════════════╗")
	fmt.Println("║              Go-Serve v1.0              ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Printf("\n📂 Serving: %s\n", absPath)
	fmt.Printf("⏰ Started: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println("\n🌐 Available on:")
	fmt.Printf("   • http://localhost:%s\n", *port)
	fmt.Printf("   • http://127.0.0.1:%s\n", *port)

	// Show network addresses
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					fmt.Printf("   • http://%s:%s\n", ipnet.IP.String(), *port)
				}
			}
		}
	}

	fmt.Println("\n⚙️  Features:")
	if requireAuth {
		fmt.Printf("   ✓ Authentication enabled (%d users)\n", len(users))
	}
	if allowUpload {
		fmt.Printf("   ✓ Upload enabled (max %dMB)\n", *maxSize)
	}
	if allowModify {
		fmt.Println("   ✓ Modify enabled (delete/rename)")
	}
	fmt.Println("   ✓ GZIP compression")
	fmt.Println("   ✓ ZIP download")
	fmt.Println("   ✓ Dark mode")
	fmt.Println("   ✓ File preview")
	fmt.Println("   ✓ Search/filter")

	fmt.Println("\n💡 Press Ctrl+C to stop")
	fmt.Println("════════════════════════════════════════\n")

	// Start server
	if err := http.Serve(listener, nil); err != nil {
		log.Fatal(err)
	}
}
