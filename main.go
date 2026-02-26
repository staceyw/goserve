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
	"golang.org/x/net/webdav"
)

type FileInfo struct {
	Name       string
	Path       string
	Size       string
	ModTime    string
	IsDir      bool
	Icon       string
	IsEditable bool
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

type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
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
    <title>GoServe - {{.Path}}</title>
    <link rel="icon" type="image/svg+xml" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 48 48'%3E%3Cdefs%3E%3ClinearGradient id='g' x1='0%25' y1='0%25' x2='100%25' y2='100%25'%3E%3Cstop offset='0%25' style='stop-color:%2300ADD8'/%3E%3Cstop offset='100%25' style='stop-color:%235DC9E2'/%3E%3C/linearGradient%3E%3C/defs%3E%3Cpath d='M8 24 Q16 12 24 24 T40 24' stroke='url(%23g)' stroke-width='4' fill='none' stroke-linecap='round' opacity='0.7'/%3E%3Cpath d='M8 30 Q16 20 24 30 T40 30' stroke='url(%23g)' stroke-width='4' fill='none' stroke-linecap='round' opacity='0.5'/%3E%3Ccircle cx='24' cy='24' r='8' fill='url(%23g)'/%3E%3Ccircle cx='24' cy='24' r='5' fill='%23fff'/%3E%3Cpath d='M24 20 L24 28 M24 20 L22 22 M24 20 L26 22' stroke='url(%23g)' stroke-width='2' stroke-linecap='round' fill='none'/%3E%3C/svg%3E">
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/codemirror/5.65.2/codemirror.min.css">
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/codemirror/5.65.2/theme/monokai.min.css">
    <script src="https://cdnjs.cloudflare.com/ajax/libs/codemirror/5.65.2/codemirror.min.js"></script>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/codemirror/5.65.2/mode/javascript/javascript.min.js"></script>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/codemirror/5.65.2/mode/python/python.min.js"></script>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/codemirror/5.65.2/mode/go/go.min.js"></script>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/codemirror/5.65.2/mode/xml/xml.min.js"></script>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/codemirror/5.65.2/mode/css/css.min.js"></script>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/codemirror/5.65.2/mode/markdown/markdown.min.js"></script>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/codemirror/5.65.2/mode/shell/shell.min.js"></script>
    <style>
        :root {
            --bg-primary: #f5f5f5;
            --bg-secondary: white;
            --bg-header: linear-gradient(135deg, #00ADD8 0%, #5DC9E2 100%);
            --text-primary: #495057;
            --text-secondary: #868e96;
            --border-color: #dee2e6;
            --hover-bg: #f8f9fa;
            --accent: #00ADD8;
        }
        [data-theme="goserve-dark"] {
            --bg-primary: #1a1a1a;
            --bg-secondary: #2d2d2d;
            --bg-header: linear-gradient(135deg, #0891b2 0%, #06b6d4 100%);
            --text-primary: #e2e8f0;
            --text-secondary: #a0aec0;
            --border-color: #4a5568;
            --hover-bg: #3d3d3d;
            --accent: #22d3ee;
        }
        [data-theme="vs-dark"] {
            --bg-primary: #1e1e1e;
            --bg-secondary: #252526;
            --bg-header: linear-gradient(135deg, #264f78 0%, #37699e 100%);
            --text-primary: #d4d4d4;
            --text-secondary: #858585;
            --border-color: #3c3c3c;
            --hover-bg: #2a2d2e;
            --accent: #007acc;
        }
        [data-theme="monokai-dimmed"] {
            --bg-primary: #1e1e1e;
            --bg-secondary: #272822;
            --bg-header: linear-gradient(135deg, #62532e 0%, #8a753f 100%);
            --text-primary: #c5c8c6;
            --text-secondary: #75715e;
            --border-color: #464741;
            --hover-bg: #3e3d32;
            --accent: #e6db74;
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
            padding: 20px 30px;
            display: flex;
            justify-content: center;
            align-items: center;
            gap: 12px;
        }
        .title {
            font-size: 18px;
            font-weight: 500;
            opacity: 0.95;
        }
        .action-bar {
            padding: 12px 20px;
            border-bottom: 1px solid var(--border-color);
            display: flex;
            justify-content: space-between;
            align-items: center;
            flex-wrap: wrap;
            gap: 15px;
            background: var(--bg-secondary);
        }
        .breadcrumb {
            display: flex;
            gap: 8px;
            flex-wrap: wrap;
            font-size: 14px;
            align-items: center;
        }
        .breadcrumb a {
            color: var(--accent);
            text-decoration: none;
            transition: opacity 0.2s;
        }
        .breadcrumb a:hover { opacity: 0.7; }
        .breadcrumb span { 
            opacity: 0.5;
            color: var(--text-secondary);
        }
        .controls { 
            display: flex;
            gap: 8px;
        }
        .btn {
            background: var(--bg-primary);
            border: 1px solid var(--border-color);
            color: var(--text-primary);
            padding: 6px 12px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 13px;
            transition: all 0.2s;
        }
        .btn:hover {
            background: var(--hover-bg);
            border-color: var(--accent);
        }
        h1 { font-size: 24px; margin-bottom: 10px; }
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
        .btn-secondary {
            background: var(--bg-secondary);
            border: 1px solid var(--border-color);
            color: var(--text-primary);
            padding: 8px 16px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 13px;
            transition: all 0.2s;
        }
        .btn-secondary:hover {
            background: var(--hover-bg);
            border-color: var(--accent);
        }
        .segmented-control {
            display: inline-flex;
            border: 1px solid var(--border-color);
            border-radius: 4px;
            overflow: hidden;
        }
        .segmented-control button {
            background: var(--bg-secondary);
            border: none;
            border-right: 1px solid var(--border-color);
            color: var(--text-primary);
            padding: 8px 16px;
            cursor: pointer;
            font-size: 13px;
            transition: all 0.2s;
            min-width: 100px;
        }
        .segmented-control button:last-child {
            border-right: none;
        }
        .segmented-control button:hover {
            background: var(--hover-bg);
        }
        .segmented-control button.active {
            background: var(--accent);
            color: white;
        }
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
        kbd {
            background: var(--hover-bg);
            border: 1px solid var(--border-color);
            border-radius: 4px;
            padding: 2px 8px;
            font-family: monospace;
            font-size: 0.9em;
            box-shadow: 0 2px 0 rgba(0,0,0,0.1);
        }
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
            <svg width="36" height="36" viewBox="0 0 48 48" xmlns="http://www.w3.org/2000/svg">
                <defs>
                    <linearGradient id="logoGrad" x1="0%" y1="0%" x2="100%" y2="100%">
                        <stop offset="0%" style="stop-color:#ffffff;stop-opacity:0.95" />
                        <stop offset="100%" style="stop-color:#ffffff;stop-opacity:0.85" />
                    </linearGradient>
                </defs>
                <path d="M8 24 Q16 12, 24 24 T40 24" stroke="url(#logoGrad)" stroke-width="4" fill="none" stroke-linecap="round" opacity="0.7"/>
                <path d="M8 30 Q16 20, 24 30 T40 30" stroke="url(#logoGrad)" stroke-width="4" fill="none" stroke-linecap="round" opacity="0.5"/>
                <circle cx="24" cy="24" r="8" fill="url(#logoGrad)"/>
                <circle cx="24" cy="24" r="5" fill="rgba(0,0,0,0.15)"/>
                <path d="M24 20 L24 28 M24 20 L22 22 M24 20 L26 22" stroke="rgba(0,173,216,0.8)" stroke-width="2" stroke-linecap="round" fill="none"/>
            </svg>
            <span class="title">GoServe v1.1</span>
        </header>
        
        <div class="action-bar">
            <div class="breadcrumb">
                <a href="/">◈ Home</a>
                {{range .Breadcrumbs}}
                    <span>/</span>
                    <a href="{{.Path}}">{{.Name}}</a>
                {{end}}
            </div>
            <div class="controls">
                <select id="themeSelect" class="btn" onchange="changeTheme(this.value)" style="cursor:pointer;">
                    <option value="light">GoServe Light</option>
                    <option value="goserve-dark">GoServe Dark</option>
                    <option value="vs-dark">VS Dark</option>
                    <option value="monokai-dimmed">Monokai Dimmed</option>
                </select>
                <button class="btn-primary" onclick="showAbout()">ℹ️ About</button>
            </div>
        </div>
        
        <div class="toolbar">
            <input type="text" class="search-box" id="searchBox" placeholder="⌕ Search files (supports *.ext wildcards)..." onkeyup="filterFiles()">
            {{if .CanUpload}}
            <form class="upload-form" id="uploadForm" method="POST" enctype="multipart/form-data" action="?upload=1">
                <input type="file" name="files" class="file-input" multiple id="fileInput" style="display:none;">
                <input type="file" name="directory"  class="file-input" webkitdirectory directory id="dirInput" style="display:none;">
                <div class="segmented-control">
                    <button type="button" id="filesBtn" class="active" onclick="selectFilesMode()">📄 Files</button>
                    <button type="button" id="folderBtn" onclick="selectFolderMode()">📁 Folder</button>
                </div>
                <button type="submit" class="btn-primary" id="uploadBtn" disabled>↑ Upload</button>
            </form>
            {{end}}
            <button class="btn-primary" onclick="downloadZip()" style="margin-left: auto;">↓ Download ZIP</button>
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
                        {{if .IsEditable}}<button class="action-btn" onclick="editFile('{{.Path}}', '{{.Name}}')" title="Edit file">✎</button>{{end}}
                        <button class="action-btn" onclick="renameFile('{{.Path}}', '{{.Name}}')" title="Rename file">⎆</button>
                        <button class="action-btn danger" onclick="deleteFile('{{.Path}}', '{{.Name}}')" title="Delete file">×</button>
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

    <div id="editorModal" class="preview-modal" onclick="closeEditor()">
        <div class="preview-content" onclick="event.stopPropagation()" style="max-width: 90%; max-height: 90%;">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; padding: 10px; background: var(--hover-bg); border-radius: 4px;">
                <span id="editorFileName" style="font-weight: 600; color: var(--text-primary);"></span>
                <div>
                    <button class="btn-primary" onclick="saveFile()" style="margin-right: 10px;">💾 Save</button>
                    <button class="btn-secondary" onclick="closeEditor()">Cancel</button>
                </div>
            </div>
            <textarea id="editor"></textarea>
        </div>
    </div>

    <div id="aboutModal" class="preview-modal" onclick="closeAbout()">
        <div class="preview-content" onclick="event.stopPropagation()" style="max-width: 600px;">
            <span class="preview-close" onclick="closeAbout()">&times;</span>
            <div id="aboutBody" style="padding: 20px;">
                <div style="text-align: center; margin-bottom: 20px;">
                    <svg id="aboutLogo" width="80" height="80" viewBox="0 0 48 48" xmlns="http://www.w3.org/2000/svg">
                        <defs>
                            <linearGradient id="aboutGrad" x1="0%" y1="0%" x2="100%" y2="100%">
                                <stop offset="0%" style="stop-color:#00ADD8;stop-opacity:1" />
                                <stop offset="100%" style="stop-color:#5DC9E2;stop-opacity:1" />
                            </linearGradient>
                        </defs>
                        <path d="M8 24 Q16 12, 24 24 T40 24" stroke="url(#aboutGrad)" stroke-width="4" fill="none" stroke-linecap="round" opacity="0.7"/>
                        <path d="M8 30 Q16 20, 24 30 T40 30" stroke="url(#aboutGrad)" stroke-width="4" fill="none" stroke-linecap="round" opacity="0.5"/>
                        <circle cx="24" cy="24" r="8" fill="url(#aboutGrad)"/>
                        <circle cx="24" cy="24" r="5" fill="#ffffff" id="aboutLogoCenter"/>
                        <path d="M24 20 L24 28 M24 20 L22 22 M24 20 L26 22" stroke="url(#aboutGrad)" stroke-width="2" stroke-linecap="round" fill="none"/>
                    </svg>
                </div>
                <h2 style="margin-top: 0; color: var(--accent); text-align: center;">GoServe v1.1</h2>
                <p style="color: var(--text-secondary); margin-bottom: 20px; text-align: center;">Lightweight HTTP file server with WebDAV support</p>
                
                <h3 style="color: var(--accent); margin-bottom: 10px;">✨ Features</h3>
                <ul style="color: var(--text-secondary); line-height: 1.8; margin-bottom: 20px;">
                    <li>📁 Directory browsing with modern UI</li>
                    <li>📂 Directory upload with structure preservation</li>
                    <li>💾 WebDAV server - mount as network drive</li>
                    <li>🔍 Search & filter with wildcards (* and ?)</li>
                    <li>👁️ File preview (images, text, markdown, code)</li>
                    <li>📦 ZIP download for directories</li>
                    <li>🔒 Optional authentication (readonly/readwrite/admin)</li>
                    <li>🌓 Dark mode toggle</li>
                    <li>🗜️ Automatic GZIP compression</li>
                </ul>
                
                <h3 style="color: var(--accent); margin-bottom: 10px;">⌨️ Keyboard Shortcuts</h3>
                <ul style="color: var(--text-secondary); line-height: 1.8; margin-bottom: 20px;">
                    <li><kbd>ESC</kbd> - Close preview/about modal</li>
                    <li><kbd>Ctrl+F</kbd> - Focus search (browser default)</li>
                </ul>
                
                <h3 style="color: var(--accent); margin-bottom: 10px;">💾 WebDAV Mount URL</h3>
                <p style="color: var(--text-secondary); margin-bottom: 5px; font-size: 0.9em;">Copy and paste this URL to mount as network drive:</p>
                <input type="text" id="webdavUrl" readonly 
                    style="width: 100%; padding: 10px; margin-bottom: 10px; font-family: monospace; background: var(--hover-bg); border: 1px solid var(--border-color); border-radius: 4px; font-size: 14px; cursor: text; color: var(--text-primary);"
                    onclick="this.select()">
                <button onclick="copyWebDAVUrl()" style="padding: 8px 16px; background: var(--accent); color: white; border: none; border-radius: 4px; cursor: pointer; margin-bottom: 20px;">
                    📋 Copy URL
                </button>
                <script>
                    document.getElementById('webdavUrl').value = window.location.protocol + '//' + window.location.host + '/webdav/';
                </script>
                
                <h3 style="color: var(--accent); margin-bottom: 10px;">🌐 Share via Tailscale</h3>
                <p style="color: var(--text-secondary); margin-bottom: 10px; font-size: 0.9em;">
                    Share this server securely over your Tailscale network or publicly via HTTPS:
                </p>
                <div style="background: var(--hover-bg); border: 1px solid var(--border-color); border-radius: 6px; padding: 15px; margin-bottom: 20px;">
                    <p style="color: var(--text-secondary); margin: 0 0 10px 0; font-weight: 600;">📱 Tailscale Serve (Private - Tailnet only):</p>
                    <pre style="background: #1f2937; color: #10b981; padding: 10px; border-radius: 4px; overflow-x: auto; margin: 0 0 10px 0; font-size: 13px;">tailscale serve --bg 8080</pre>
                    <p style="color: var(--text-secondary); margin: 0 0 15px 0; font-size: 0.85em;">
                        Share with devices on your Tailscale network. Access via your machine's Tailscale hostname.
                    </p>
                    
                    <p style="color: var(--text-secondary); margin: 0 0 10px 0; font-weight: 600;">🔓 Tailscale Funnel (Public HTTPS):</p>
                    <pre style="background: #1f2937; color: #10b981; padding: 10px; border-radius: 4px; overflow-x: auto; margin: 0 0 10px 0; font-size: 13px;">tailscale funnel --bg 8080</pre>
                    <p style="color: var(--text-secondary); margin: 0; font-size: 0.85em;">
                        Share publicly via HTTPS tunnel. Anyone with the link can access (use with <code style="background: var(--hover-bg); padding: 2px 6px; border-radius: 3px; color: var(--text-primary);">-logins</code> for auth).
                    </p>
                </div>
                
                <h3 style="color: var(--accent); margin-bottom: 10px;">🔗 Links</h3>
                <p style="color: var(--text-secondary);">
                    <a href="https://github.com/staceyw/GoServe" target="_blank" style="color: var(--accent); text-decoration: none;">📦 GitHub Repository</a><br>
                    <span style="color: var(--text-secondary); font-size: 0.9em; opacity: 0.7;">MIT License • Built with Go 1.25+</span>
                </p>
            </div>
        </div>
    </div>

    <script>
        // Theme system
        function isDarkTheme(theme) {
            return theme !== 'light';
        }

        function updateAboutLogo(theme) {
            const logoCenter = document.getElementById('aboutLogoCenter');
            if (logoCenter) {
                logoCenter.setAttribute('fill', isDarkTheme(theme) ? '#2d2d2d' : '#ffffff');
            }
        }

        function changeTheme(theme) {
            if (theme === 'light') {
                document.documentElement.removeAttribute('data-theme');
            } else {
                document.documentElement.setAttribute('data-theme', theme);
            }
            localStorage.setItem('theme', theme);
            document.getElementById('themeSelect').value = theme;
            updateAboutLogo(theme);
            if (editor) {
                editor.setOption('theme', isDarkTheme(theme) ? 'monokai' : 'default');
            }
        }

        // Load saved theme
        let savedTheme = localStorage.getItem('theme') || 'light';
        if (savedTheme === 'dark') savedTheme = 'goserve-dark';
        changeTheme(savedTheme);

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

        // Text editor
        let editor = null;
        let currentEditPath = '';

        function editFile(path, name) {
            currentEditPath = path;
            document.getElementById('editorFileName').textContent = name;
            
            fetch(path)
                .then(r => {
                    if (!r.ok) throw new Error('Failed to load file');
                    return r.text();
                })
                .then(content => {
                    document.getElementById('editor').value = content;
                    document.getElementById('editorModal').style.display = 'block';
                    
                    // Initialize CodeMirror if not already initialized
                    if (!editor) {
                        const currentTheme = localStorage.getItem('theme') || 'light';
                        editor = CodeMirror.fromTextArea(document.getElementById('editor'), {
                            lineNumbers: true,
                            theme: isDarkTheme(currentTheme) ? 'monokai' : 'default',
                            mode: getMode(name),
                            indentUnit: 4,
                            lineWrapping: true
                        });
                        editor.setSize('100%', '70vh');
                    } else {
                        editor.setValue(content);
                        editor.setOption('mode', getMode(name));
                    }
                })
                .catch(err => alert('Error loading file: ' + err.message));
        }

        function getMode(filename) {
            const ext = filename.split('.').pop().toLowerCase();
            const modes = {
                'js': 'javascript',
                'json': 'javascript',
                'py': 'python',
                'go': 'go',
                'html': 'xml',
                'xml': 'xml',
                'css': 'css',
                'md': 'markdown',
                'sh': 'shell',
                'bash': 'shell',
                'txt': 'text/plain'
            };
            return modes[ext] || 'text/plain';
        }

        function saveFile() {
            const content = editor.getValue();
            fetch(currentEditPath + '?edit=1', {
                method: 'POST',
                headers: { 'Content-Type': 'text/plain' },
                body: content
            })
            .then(r => r.json())
            .then(data => {
                if (data.success) {
                    alert('✓ File saved successfully!');
                    closeEditor();
                } else {
                    alert('Error: ' + data.error);
                }
            })
            .catch(err => alert('Error saving file: ' + err.message));
        }

        function closeEditor() {
            document.getElementById('editorModal').style.display = 'none';
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

        function showAbout() {
            updateAboutLogo(localStorage.getItem('theme') || 'light');
            document.getElementById('aboutModal').style.display = 'block';
        }

        function closeAbout() {
            document.getElementById('aboutModal').style.display = 'none';
        }

        function copyWebDAVUrl() {
            const urlInput = document.getElementById('webdavUrl');
            urlInput.select();
            urlInput.setSelectionRange(0, 99999); // For mobile
            
            try {
                navigator.clipboard.writeText(urlInput.value).then(() => {
                    alert('✓ WebDAV URL copied to clipboard!');
                }).catch(() => {
                    document.execCommand('copy');
                    alert('✓ WebDAV URL copied!');
                });
            } catch (err) {
                document.execCommand('copy');
                alert('✓ WebDAV URL copied!');
            }
        }

        function escapeHtml(text) {
            const map = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' };
            return text.replace(/[&<>"']/g, m => map[m]);
        }

        // Close modals with Escape key
        document.addEventListener('keydown', e => {
            if (e.key === 'Escape') {
                closePreview();
                closeAbout();
                closeEditor();
            }
        });

        // Handle file/folder uploads
        let selectedFiles = [];
        
        function selectFilesMode() {
            document.getElementById('filesBtn').classList.add('active');
            document.getElementById('folderBtn').classList.remove('active');
            document.getElementById('fileInput').click();
        }
        
        function selectFolderMode() {
            document.getElementById('folderBtn').classList.add('active');
            document.getElementById('filesBtn').classList.remove('active');
            document.getElementById('dirInput').click();
        }
        
        document.getElementById('fileInput')?.addEventListener('change', function(e) {
            selectedFiles = Array.from(e.target.files);
            updateUploadButton();
        });
        
        document.getElementById('dirInput')?.addEventListener('change', function(e) {
            selectedFiles = Array.from(e.target.files);
            updateUploadButton();
        });
        
        function updateUploadButton() {
            const btn = document.getElementById('uploadBtn');
            if (selectedFiles.length > 0) {
                btn.disabled = false;
                btn.textContent = '↑ Upload ' + selectedFiles.length + ' file(s)';
            } else {
                btn.disabled = true;
                btn.textContent = '↑ Upload';
            }
        }
        
        document.getElementById('uploadForm')?.addEventListener('submit', function(e) {
            e.preventDefault();
            if (selectedFiles.length === 0) return;
            
            const formData = new FormData();
            selectedFiles.forEach(file => {
                const path = file.webkitRelativePath || file.name;
                formData.append('files', file, path);
            });
            
            fetch(window.location.pathname + '?upload=1', {
                method: 'POST',
                body: formData
            }).then(response => {
                if (response.ok) {
                    window.location.reload();
                } else {
                    alert('Upload failed');
                }
            }).catch(err => {
                alert('Upload error: ' + err.message);
            });
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

func isEditableFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	editableExts := map[string]bool{
		".txt": true, ".md": true, ".markdown": true,
		".go": true, ".py": true, ".js": true, ".ts": true,
		".html": true, ".htm": true, ".css": true, ".scss": true,
		".json": true, ".xml": true, ".yaml": true, ".yml": true,
		".toml": true, ".ini": true, ".conf": true, ".config": true,
		".sh": true, ".bash": true, ".zsh": true, ".fish": true,
		".ps1": true, ".bat": true, ".cmd": true,
		".c": true, ".cpp": true, ".h": true, ".hpp": true,
		".java": true, ".kt": true, ".scala": true,
		".rb": true, ".php": true, ".pl": true, ".lua": true,
		".rs": true, ".swift": true, ".m": true,
		".sql": true, ".csv": true, ".tsv": true,
		".log": true, ".env": true, ".gitignore": true,
		".dockerfile": true, ".makefile": true,
	}
	// Also check for files without extension or common text file names
	if ext == "" {
		baseName := strings.ToLower(filepath.Base(name))
		commonTextFiles := map[string]bool{
			"readme": true, "license": true, "makefile": true,
			"dockerfile": true, "gemfile": true, "rakefile": true,
		}
		return commonTextFiles[baseName]
	}
	return editableExts[ext]
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

		// Security check - use baseDir + separator to prevent prefix collision
		// e.g., baseDir="/data" must not match "/database"
		if !strings.HasPrefix(fullPath+string(filepath.Separator), baseDir+string(filepath.Separator)) {
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

		// Handle file edit
		if r.URL.Query().Get("edit") != "" && r.Method == "POST" {
			if !canModify {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"success": false, "error": "Forbidden: Edit not allowed"}`)
				return
			}
			handleEdit(w, r, fullPath, baseDir)
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
			}

			size := ""
			if !entry.IsDir() {
				size = formatSize(info.Size())
			} else {
				size = "-"
			}

			files = append(files, FileInfo{
				Name:       name,
				Path:       urlPath,
				Size:       size,
				ModTime:    info.ModTime().Format("2006-01-02 15:04:05"),
				IsDir:      entry.IsDir(),
				Icon:       getIcon(name, entry.IsDir()),
				IsEditable: !entry.IsDir() && isEditableFile(name),
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
	r.ParseMultipartForm(maxUploadSize * 10) // Allow larger total size for multiple files

	// Get all uploaded files
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "No files uploaded", http.StatusBadRequest)
		return
	}

	uploadedCount := 0
	var lastError error

	for _, fileHeader := range files {
		// Check file size
		if fileHeader.Size > maxUploadSize {
			lastError = fmt.Errorf("file %s too large", fileHeader.Filename)
			continue
		}

		// Open uploaded file
		file, err := fileHeader.Open()
		if err != nil {
			lastError = err
			continue
		}

		// Extract relative path from filename (for directory uploads)
		// For regular files, this is just the filename
		relativePath := filepath.FromSlash(fileHeader.Filename)

		// Prevent path traversal attacks
		relativePath = filepath.Clean(relativePath)
		if strings.Contains(relativePath, "..") {
			file.Close()
			lastError = fmt.Errorf("invalid path: %s", relativePath)
			continue
		}

		// Full destination path
		destPath := filepath.Join(targetDir, relativePath)

		// Create parent directories if needed
		destDir := filepath.Dir(destPath)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			file.Close()
			lastError = err
			continue
		}

		// Save file
		dst, err := os.Create(destPath)
		if err != nil {
			file.Close()
			lastError = err
			continue
		}

		if _, err := io.Copy(dst, file); err != nil {
			dst.Close()
			file.Close()
			lastError = err
			continue
		}

		dst.Close()
		file.Close()
		uploadedCount++
	}

	// Return response
	if uploadedCount == 0 && lastError != nil {
		http.Error(w, fmt.Sprintf("Upload failed: %v", lastError), http.StatusInternalServerError)
		return
	}

	// Success - redirect back to directory
	http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
}

func handleDelete(w http.ResponseWriter, r *http.Request, baseDir string) {
	path := r.URL.Query().Get("delete")
	fullPath := filepath.Join(baseDir, path)

	if !strings.HasPrefix(fullPath+string(filepath.Separator), baseDir+string(filepath.Separator)) {
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

	if !strings.HasPrefix(oldFullPath+string(filepath.Separator), baseDir+string(filepath.Separator)) ||
		!strings.HasPrefix(newFullPath+string(filepath.Separator), baseDir+string(filepath.Separator)) {
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

func handleEdit(w http.ResponseWriter, r *http.Request, fullPath, baseDir string) {
	// Security check
	if !strings.HasPrefix(fullPath+string(filepath.Separator), baseDir+string(filepath.Separator)) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success": false, "error": "Invalid path"}`)
		return
	}

	// Read the new content from request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success": false, "error": "Failed to read content"}`)
		return
	}

	// Write to file
	err = os.WriteFile(fullPath, body, 0644)
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
		fmt.Fprintf(os.Stderr, "GoServe v1.1 - Lightweight HTTP File Server\n\n")
		fmt.Fprintf(os.Stderr, "USAGE:\n")
		fmt.Fprintf(os.Stderr, "  go run main.go [options]\n\n")
		fmt.Fprintf(os.Stderr, "OPTIONS:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEXAMPLES:\n")
		fmt.Fprintf(os.Stderr, "  Basic usage (serve current directory):\n")
		fmt.Fprintf(os.Stderr, "    go run main.go\n\n")
		fmt.Fprintf(os.Stderr, "  Listen on custom address:\n")
		fmt.Fprintf(os.Stderr, "    go run main.go -listen :3000\n\n")
		fmt.Fprintf(os.Stderr, "  Listen on specific interface:\n")
		fmt.Fprintf(os.Stderr, "    go run main.go -listen 127.0.0.1:8080\n\n")
		fmt.Fprintf(os.Stderr, "  Multiple listeners:\n")
		fmt.Fprintf(os.Stderr, "    go run main.go -listen :8080 -listen 127.0.0.1:9090\n\n")
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
		fmt.Fprintf(os.Stderr, "    go run main.go -listen :8000 -dir /var/www -upload -modify -logins logins.txt\n\n")
		fmt.Fprintf(os.Stderr, "TAILSCALE SHARING:\n")
		fmt.Fprintf(os.Stderr, "  Share privately on your Tailscale network:\n")
		fmt.Fprintf(os.Stderr, "    go run main.go &\n")
		fmt.Fprintf(os.Stderr, "    tailscale serve --bg 8080\n\n")
		fmt.Fprintf(os.Stderr, "  Share publicly via HTTPS (use with -logins):\n")
		fmt.Fprintf(os.Stderr, "    go run main.go -logins logins.txt &\n")
		fmt.Fprintf(os.Stderr, "    tailscale funnel --bg 8080\n\n")
	}

	// Command line flags
	var listenAddrs stringSlice
	flag.Var(&listenAddrs, "listen", "Address to listen on in host:port format (repeatable, default :8080)")
	dir := flag.String("dir", ".", "Directory to serve")
	quiet := flag.Bool("quiet", false, "Quiet mode - only show errors")
	uploadFlag := flag.Bool("upload", false, "Allow file uploads")
	modifyFlag := flag.Bool("modify", false, "Allow file deletion and renaming")
	maxSize := flag.Int64("maxsize", 100, "Max upload size in MB")
	loginFile := flag.String("logins", "", "Enable authentication with login file (format: username:password:permission)")
	flag.Parse()

	if len(listenAddrs) == 0 {
		listenAddrs = stringSlice{"localhost:8080"}
	}

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

	// Setup WebDAV handler
	webdavHandler := &webdav.Handler{
		FileSystem: webdav.Dir(absPath),
		LockSystem: webdav.NewMemLS(),
		Logger: func(r *http.Request, err error) {
			if err != nil && !*quiet {
				log.Printf("WebDAV: %s %s - %v", r.Method, r.URL.Path, err)
			}
		},
	}

	// WebDAV handler with authentication
	webdavHTTP := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strip /webdav prefix for the webdav handler
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/webdav")
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}
		webdavHandler.ServeHTTP(w, r)
	})

	if requireAuth {
		http.HandleFunc("/webdav/", authMiddleware(webdavHTTP))
	} else {
		http.HandleFunc("/webdav/", webdavHTTP)
	}

	// Setup handler with authentication and GZIP
	handler := dirHandler(absPath, tmpl, *quiet)
	if requireAuth {
		handler = authMiddleware(handler)
	}
	http.HandleFunc("/", gzipMiddleware(handler))

	// Create listeners
	var listeners []net.Listener
	for _, addr := range listenAddrs {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			for _, l := range listeners {
				l.Close()
			}
			fmt.Printf("\n❌ ERROR: Cannot listen on %s\n", addr)
			if strings.Contains(err.Error(), "address already in use") ||
				strings.Contains(err.Error(), "Only one usage") {
				fmt.Printf("   %s is already in use by another application.\n", addr)
			} else {
				fmt.Printf("   %v\n", err)
			}
			fmt.Println()
			os.Exit(1)
		}
		listeners = append(listeners, ln)
	}

	// Display startup info
	fmt.Println("╔════════════════════════════════════════╗")
	fmt.Println("║              GoServe v1.1               ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Printf("\n📂 Serving: %s\n", absPath)
	fmt.Printf("⏰ Started: %s\n", time.Now().Format("2006-01-02 15:04:05"))

	fmt.Println("\n🌐 Listeners:")
	var wildcardPorts []string
	for _, ln := range listeners {
		host, port, _ := net.SplitHostPort(ln.Addr().String())
		if host == "::" || host == "0.0.0.0" || host == "" {
			fmt.Printf("   • http://localhost:%s\n", port)
			wildcardPorts = append(wildcardPorts, port)
		} else {
			fmt.Printf("   • http://%s:%s\n", host, port)
		}
	}
	if len(wildcardPorts) > 0 {
		ifaces, err := net.InterfaceAddrs()
		if err == nil {
			for _, a := range ifaces {
				if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
					for _, port := range wildcardPorts {
						fmt.Printf("   • http://%s:%s (LAN)\n", ipnet.IP.String(), port)
					}
				}
			}
		}
	}

	fmt.Println("\n📁 WebDAV:")
	for _, ln := range listeners {
		host, port, _ := net.SplitHostPort(ln.Addr().String())
		if host == "::" || host == "0.0.0.0" || host == "" {
			fmt.Printf("   • http://localhost:%s/webdav/\n", port)
		} else {
			fmt.Printf("   • http://%s:%s/webdav/\n", host, port)
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

	// Start server on all listeners
	errc := make(chan error, 1)
	for _, ln := range listeners {
		go func(l net.Listener) {
			errc <- http.Serve(l, nil)
		}(ln)
	}
	log.Fatal(<-errc)
}
