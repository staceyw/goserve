package main

import (
	"archive/zip"
	"compress/gzip"
	"encoding/json"
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
	"sync"
	"time"

	"github.com/russross/blackfriday/v2"
	"golang.org/x/net/webdav"
)

var version = "dev"

// Shared mutable base directory (changed via :command in search box)
var (
	currentBaseDir string
	baseDirMu      sync.RWMutex
)

// isUnderDir checks if fullPath is equal to or under baseDir.
// Handles drive roots like C:\ where baseDir already ends with a separator.
func isUnderDir(fullPath, baseDir string) bool {
	base := strings.TrimRight(baseDir, string(filepath.Separator)) + string(filepath.Separator)
	return strings.HasPrefix(fullPath+string(filepath.Separator), base)
}

func getBaseDir() string {
	baseDirMu.RLock()
	defer baseDirMu.RUnlock()
	return currentBaseDir
}

func setBaseDir(dir string) {
	baseDirMu.Lock()
	defer baseDirMu.Unlock()
	currentBaseDir = dir
}

type FileInfo struct {
	Name       string
	Path       string
	Size       string
	ModTime    string
	IsDir      bool
	Icon       string
	IsEditable bool
	RawSize    int64
	RawMod     int64
}

type PageData struct {
	Path        string
	FullPath    string
	Files       []FileInfo
	Breadcrumbs []Breadcrumb
	CanUpload   bool
	CanModify   bool
	Version     string
}

type Breadcrumb struct {
	Name string
	Path string
}

type User struct {
	Username   string
	Password   string
	Permission string // readonly, readwrite, all
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
            --bg-primary: #dce0e8;
            --bg-secondary: #eff1f5;

            --text-primary: #4c4f69;
            --text-secondary: #6c6f85;
            --border-color: #ccd0da;
            --hover-bg: #e6e9ef;
            --accent: #1e66f5;
        }
        [data-theme="catppuccin-mocha"] {
            --bg-primary: #11111b;
            --bg-secondary: #1e1e2e;

            --text-primary: #cdd6f4;
            --text-secondary: #a6adc8;
            --border-color: #313244;
            --hover-bg: #313244;
            --accent: #89b4fa;
        }
        [data-theme="dracula"] {
            --bg-primary: #1e1f29;
            --bg-secondary: #282a36;

            --text-primary: #f8f8f2;
            --text-secondary: #6272a4;
            --border-color: #44475a;
            --hover-bg: #44475a;
            --accent: #bd93f9;
        }
        [data-theme="nord"] {
            --bg-primary: #242933;
            --bg-secondary: #2e3440;

            --text-primary: #d8dee9;
            --text-secondary: #4c566a;
            --border-color: #3b4252;
            --hover-bg: #3b4252;
            --accent: #88c0d0;
        }
        [data-theme="solarized-dark"] {
            --bg-primary: #001e26;
            --bg-secondary: #002b36;

            --text-primary: #839496;
            --text-secondary: #586e75;
            --border-color: #073642;
            --hover-bg: #073642;
            --accent: #268bd2;
        }
        [data-theme="solarized-light"] {
            --bg-primary: #fdf6e3;
            --bg-secondary: #eee8d5;

            --text-primary: #657b83;
            --text-secondary: #93a1a1;
            --border-color: #ddd6c1;
            --hover-bg: #fdf6e3;
            --accent: #268bd2;
        }
        [data-theme="one-dark"] {
            --bg-primary: #1b1f23;
            --bg-secondary: #21252b;

            --text-primary: #abb2bf;
            --text-secondary: #5c6370;
            --border-color: #181a1f;
            --hover-bg: #2c313a;
            --accent: #61afef;
        }
        [data-theme="gruvbox"] {
            --bg-primary: #1d2021;
            --bg-secondary: #282828;

            --text-primary: #ebdbb2;
            --text-secondary: #a89984;
            --border-color: #3c3836;
            --hover-bg: #3c3836;
            --accent: #b8bb26;
        }
        [data-theme="monokai-dimmed"] {
            --bg-primary: #1e1e1e;
            --bg-secondary: #272727;

            --text-primary: #c5c8c6;
            --text-secondary: #b0b0b0;
            --border-color: #303030;
            --hover-bg: #383838;
            --accent: #707070;
        }
        [data-theme="abyss"] {
            --bg-primary: #000c18;
            --bg-secondary: #000c18;

            --text-primary: #6688cc;
            --text-secondary: #384887;
            --border-color: #082050;
            --hover-bg: #082050;
            --accent: #225588;
        }
        [data-theme="github-light"] {
            --bg-primary: #f0f3f6;
            --bg-secondary: #ffffff;

            --text-primary: #1f2328;
            --text-secondary: #656d76;
            --border-color: #d0d7de;
            --hover-bg: #f6f8fa;
            --accent: #0969da;
        }
        [data-theme="ibm-3278"] {
            --bg-primary: #010401;
            --bg-secondary: #020602;

            --text-primary: #33ff33;
            --text-secondary: #1a9a1a;
            --border-color: #0a3a0a;
            --hover-bg: #0a1a0a;
            --accent: #33ff33;
        }
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: var(--bg-primary);
            color: var(--text-primary);
            margin: 0;
            height: 100vh;
            overflow: hidden;
            transition: background 0.3s, color 0.3s;
        }
        .container {
            display: flex;
            flex-direction: column;
            height: 100vh;
            background: var(--bg-secondary);
            overflow: hidden;
        }
        header {
            background: var(--bg-secondary);
            color: var(--text-primary);
            padding: 12px 20px;
            display: flex;
            align-items: center;
            gap: 6px;
            border-bottom: 1px solid var(--border-color);
        }
        .title {
            font-size: 18px;
            font-weight: 700;
            color: var(--text-primary);
        }
        .title .accent {
            color: var(--accent);
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
            padding: 0 20px;
            border-bottom: 1px solid var(--border-color);
            display: flex;
            gap: 15px;
            align-items: center;
            height: 44px;
        }
        .selection-bar {
            display: none;
            align-items: center;
            gap: 6px;
            margin-left: auto;
        }
        .selection-bar.active { display: flex; }
        .selection-count {
            font-size: 14px;
            font-weight: 500;
            color: var(--text-primary);
            margin-right: 8px;
            white-space: nowrap;
        }
        .sel-btn {
            background: none;
            border: none;
            color: var(--text-secondary);
            cursor: pointer;
            padding: 4px;
            border-radius: 4px;
            font-size: 18px;
            line-height: 1;
            transition: all 0.15s;
            display: flex;
            align-items: center;
        }
        .sel-btn:hover { background: var(--hover-bg); color: var(--text-primary); }
        .sel-btn.danger:hover { color: #dc3545; }
        .search-box {
            width: 33%;
            min-width: 150px;
            margin-left: auto;
            padding: 6px 12px;
            border: 1px solid var(--border-color);
            border-radius: 4px;
            background: var(--bg-primary);
            color: var(--text-primary);
            font-size: 13px;
        }
        .btn-primary {
            background: var(--bg-secondary);
            border: 1px solid var(--border-color);
            color: var(--accent);
            padding: 8px 16px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 13px;
            font-weight: 500;
            transition: all 0.2s;
        }
        .btn-primary:hover { background: var(--hover-bg); border-color: var(--accent); }
        .btn-secondary {
            background: var(--bg-secondary);
            border: 1px solid var(--border-color);
            color: var(--accent);
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
        .breadcrumb-caret {
            background: none;
            border: none;
            color: var(--accent);
            cursor: pointer;
            font-size: 14px;
            padding: 0 2px;
            margin-left: 2px;
            transition: opacity 0.2s;
        }
        .breadcrumb-caret:hover { opacity: 0.7; }
        .context-menu {
            display: none;
            position: fixed;
            background: var(--bg-secondary);
            border: 1px solid var(--border-color);
            border-radius: 6px;
            box-shadow: 0 4px 12px rgba(0,0,0,0.15);
            z-index: 500;
            min-width: 180px;
            padding: 4px 0;
        }
        .context-menu.show { display: block; }
        .context-menu-item {
            padding: 8px 16px;
            cursor: pointer;
            font-size: 13px;
            color: var(--text-primary);
            display: flex;
            align-items: center;
            gap: 10px;
            width: 100%;
            border: none;
            background: none;
            text-align: left;
        }
        .context-menu-item svg { flex-shrink: 0; }
        .context-menu-item:hover { background: var(--hover-bg); }
        .context-menu-separator {
            height: 1px;
            background: var(--border-color);
            margin: 4px 0;
        }
        .modal-input {
            width: 100%;
            padding: 8px 12px;
            border: 1px solid var(--border-color);
            border-radius: 4px;
            background: var(--bg-secondary);
            color: var(--text-primary);
            font-size: 14px;
            margin: 15px 0;
            box-sizing: border-box;
        }
        .modal-buttons {
            display: flex;
            gap: 10px;
            justify-content: flex-end;
        }
        .dialog-overlay {
            display: none;
            position: fixed;
            top: 0; left: 0; right: 0; bottom: 0;
            background: rgba(0,0,0,0.5);
            z-index: 2000;
            align-items: center;
            justify-content: center;
        }
        .dialog-overlay.active { display: flex; }
        .dialog-box {
            background: var(--bg-secondary);
            border: 1px solid var(--border-color);
            border-radius: 8px;
            padding: 24px;
            min-width: 340px;
            max-width: 480px;
            box-shadow: 0 8px 32px rgba(0,0,0,0.3);
        }
        .dialog-title {
            font-size: 16px;
            font-weight: 600;
            color: var(--text-primary);
            margin: 0 0 8px 0;
        }
        .dialog-message {
            font-size: 14px;
            color: var(--text-secondary);
            margin: 0 0 20px 0;
            line-height: 1.5;
            white-space: pre-wrap;
            word-break: break-word;
        }
        .dialog-input {
            width: 100%;
            padding: 8px 12px;
            border: 1px solid var(--border-color);
            border-radius: 4px;
            background: var(--bg-primary);
            color: var(--text-primary);
            font-size: 14px;
            margin-bottom: 20px;
            box-sizing: border-box;
        }
        .dialog-input:focus { outline: none; border-color: var(--accent); }
        .dialog-buttons {
            display: flex;
            gap: 8px;
            justify-content: flex-end;
        }
        .dialog-btn {
            padding: 8px 20px;
            border-radius: 4px;
            font-size: 13px;
            font-weight: 500;
            cursor: pointer;
            border: 1px solid var(--border-color);
            background: var(--bg-primary);
            color: var(--text-primary);
            transition: all 0.15s;
        }
        .dialog-btn:hover { background: var(--hover-bg); }
        .dialog-btn.primary {
            background: var(--accent);
            color: white;
            border-color: var(--accent);
        }
        .dialog-btn.primary:hover { opacity: 0.9; }
        .dialog-btn.danger {
            background: #dc3545;
            color: white;
            border-color: #dc3545;
        }
        .dialog-btn.danger:hover { opacity: 0.9; }
        table {
            width: 100%;
            border-collapse: collapse;
        }
        .table-container {
            flex: 1;
            overflow-y: auto;
            min-height: 0;
        }
        thead {
            background: var(--hover-bg);
            border-bottom: 2px solid var(--border-color);
            position: sticky;
            top: 0;
            z-index: 5;
        }
        th {
            text-align: left;
            padding: 8px 20px;
            font-weight: 600;
            color: var(--text-primary);
            font-size: 14px;
            cursor: pointer;
            user-select: none;
        }
        th:hover { background: var(--border-color); }
        .sort-arrow { display: inline-block; width: 18px; height: 18px; vertical-align: middle; margin-left: 4px; border-radius: 50%; text-align: center; line-height: 18px; font-size: 13px; }
        th.sorted .sort-arrow { background: var(--accent); color: white; }
        td {
            padding: 6px 20px;
            border-bottom: 1px solid var(--border-color);
        }
        tbody tr { cursor: default; user-select: none; }
        tr:hover { background: var(--hover-bg); }
        tr.selected { background: var(--accent); }
        tr.selected td { color: white; }
        tr.selected .file-link { color: white; }
        tr.selected .size, tr.selected .modified { color: rgba(255,255,255,0.8); }
        tr.selected:hover { background: var(--accent); }
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
        footer {
            padding: 4px 16px;
            display: flex;
            align-items: center;
            justify-content: space-between;
            color: var(--text-secondary);
            font-size: 12px;
            border-top: 1px solid var(--border-color);
            background: var(--bg-secondary);
            flex-shrink: 0;
            position: relative;
        }
        .footer-left { display: flex; align-items: center; gap: 8px; }
        .footer-right { display: flex; align-items: center; gap: 12px; }
        .footer-btn {
            background: none;
            border: none;
            color: var(--text-secondary);
            cursor: pointer;
            padding: 4px;
            display: flex;
            align-items: center;
            border-radius: 4px;
        }
        .footer-btn:hover { color: var(--text-primary); background: var(--hover-bg); }
        .footer-menu {
            display: none;
            position: absolute;
            bottom: 100%;
            left: 8px;
            background: var(--bg-secondary);
            border: 1px solid var(--border-color);
            border-radius: 6px;
            box-shadow: 0 -4px 16px rgba(0,0,0,0.2);
            min-width: 200px;
            padding: 6px 0;
            z-index: 100;
            margin-bottom: 4px;
        }
        .footer-menu.active { display: block; }
        .footer-menu-item {
            padding: 8px 14px;
            display: flex;
            align-items: center;
            gap: 10px;
            cursor: pointer;
            font-size: 13px;
            color: var(--text-primary);
            border: none;
            background: none;
            width: 100%;
            text-align: left;
        }
        .footer-menu-item:hover { background: var(--hover-bg); }
        .footer-menu-item select {
            flex: 1;
            background: var(--bg-primary);
            color: var(--text-primary);
            border: 1px solid var(--border-color);
            border-radius: 4px;
            padding: 4px 8px;
            font-size: 12px;
            cursor: pointer;
        }
        .footer-menu-separator { height: 1px; background: var(--border-color); margin: 4px 0; }
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
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <svg width="36" height="36" viewBox="0 0 48 48" xmlns="http://www.w3.org/2000/svg">
                <path d="M8 24 Q16 12, 24 24 T40 24" stroke="var(--accent)" stroke-width="4" fill="none" stroke-linecap="round" opacity="0.7"/>
                <path d="M8 30 Q16 20, 24 30 T40 30" stroke="var(--accent)" stroke-width="4" fill="none" stroke-linecap="round" opacity="0.5"/>
                <circle cx="24" cy="24" r="8" fill="var(--accent)"/>
                <circle cx="24" cy="24" r="5" fill="var(--bg-secondary)"/>
                <path d="M24 20 L24 28 M24 20 L22 22 M24 20 L26 22" stroke="var(--accent)" stroke-width="2" stroke-linecap="round" fill="none"/>
            </svg>
            <span class="title">Go<span class="accent">Serve</span></span>
            <input type="text" class="search-box" id="searchBox" placeholder="⌕ Search  |  : command" onkeyup="filterFiles()" onkeydown="handleSearchKey(event)">
        </header>

        <div class="toolbar" id="toolbar">
            <div class="breadcrumb" id="breadcrumbBar">
                <a href="/">Home</a>
                {{range .Breadcrumbs}}
                    <span>/</span>
                    <a href="{{.Path}}">{{.Name}}</a>
                {{end}}
            </div>
            <div class="selection-bar" id="selectionBar">
                <button class="sel-btn" onclick="clearSelection()" title="Clear selection"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M18 6L6 18M6 6l12 12"/></svg></button>
                <span class="selection-count" id="selectionCount">0 selected</span>
                <button class="sel-btn" onclick="ctxDownloadSelected()" title="Download"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v12m0 0l-5-5m5 5l5-5"/><path d="M5 21h14"/></svg></button>
                <button class="sel-btn" onclick="ctxCopyLink()" title="Copy link"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10 13a5 5 0 007.54.54l3-3a5 5 0 00-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 00-7.54-.54l-3 3a5 5 0 007.07 7.07l1.71-1.71"/></svg></button>
                {{if .CanModify}}
                <button class="sel-btn" id="selEditBtn" onclick="ctxEditSelected()" title="Edit"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="transform:scaleX(-1)"><path d="M12 20h9"/><path d="M16.5 3.5a2.12 2.12 0 013 3L7 19l-4 1 1-4L16.5 3.5z"/></svg></button>
                <button class="sel-btn" id="selRenameBtn" onclick="ctxRenameSelected()" title="Rename"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M7 4v16"/><path d="M4 4h6"/><path d="M4 20h6"/><path d="M14 4h6"/><path d="M14 20h6"/><path d="M17 4v16"/><path d="M10 12h4"/></svg></button>
                <button class="sel-btn danger" onclick="ctxDeleteSelected()" title="Delete"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18M8 6V4h8v2"/><path d="M5 6v14a2 2 0 002 2h10a2 2 0 002-2V6"/><path d="M10 11v6M14 11v6"/></svg></button>
                {{end}}
            </div>
            {{if .CanUpload}}
            <input type="file" name="files" multiple id="fileInput" style="display:none;">
            <input type="file" name="directory" webkitdirectory directory id="dirInput" style="display:none;">
            {{end}}
        </div>

        <div id="folderContextMenu" class="context-menu">
            {{if .CanModify}}
            <button class="context-menu-item" onclick="showNewFolderModal()"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/><path d="M12 11v6M9 14h6"/></svg>New Folder</button>
            {{end}}
            {{if .CanUpload}}
            {{if .CanModify}}<div class="context-menu-separator"></div>{{end}}
            <button class="context-menu-item" onclick="triggerFileUpload()"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/><path d="M12 18v-6M9 15l3-3 3 3"/></svg>File Upload</button>
            <button class="context-menu-item" onclick="triggerFolderUpload()"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/><path d="M12 11v6M9 12l3-3 3 3"/></svg>Folder Upload</button>
            {{end}}
            <div class="context-menu-separator"></div>
            <button class="context-menu-item" onclick="copyFolderLink()"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10 13a5 5 0 007.54.54l3-3a5 5 0 00-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 00-7.54-.54l-3 3a5 5 0 007.07 7.07l1.71-1.71"/></svg>Copy Link</button>
        </div>

        <div id="rowContextMenu" class="context-menu">
            <button class="context-menu-item" id="ctxDownload" onclick="ctxDownloadSelected()"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v12m0 0l-5-5m5 5l5-5"/><path d="M5 21h14"/></svg>Download</button>
            <button class="context-menu-item" onclick="ctxCopyLink()"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10 13a5 5 0 007.54.54l3-3a5 5 0 00-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 00-7.54-.54l-3 3a5 5 0 007.07 7.07l1.71-1.71"/></svg>Copy Link</button>
            {{if .CanModify}}
            <div class="context-menu-separator"></div>
            <button class="context-menu-item" id="ctxEdit" onclick="ctxEditSelected()"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="transform:scaleX(-1)"><path d="M12 20h9"/><path d="M16.5 3.5a2.12 2.12 0 013 3L7 19l-4 1 1-4L16.5 3.5z"/></svg>Edit</button>
            <button class="context-menu-item" id="ctxRename" onclick="ctxRenameSelected()"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M7 4v16"/><path d="M4 4h6"/><path d="M4 20h6"/><path d="M14 4h6"/><path d="M14 20h6"/><path d="M17 4v16"/><path d="M10 12h4"/></svg>Rename</button>
            <button class="context-menu-item" id="ctxDelete" onclick="ctxDeleteSelected()"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18M8 6V4h8v2"/><path d="M5 6v14a2 2 0 002 2h10a2 2 0 002-2V6"/><path d="M10 11v6M14 11v6"/></svg>Delete</button>
            {{end}}
        </div>

        <div class="table-container">
        <table id="fileTable">
            <thead>
                <tr>
                    <th onclick="sortTable(0)">Name <span class="sort-arrow"></span></th>
                    <th onclick="sortTable(1)">Size <span class="sort-arrow"></span></th>
                    <th class="modified" onclick="sortTable(2)">Modified <span class="sort-arrow"></span></th>
                </tr>
            </thead>
            <tbody>
                {{range .Files}}
                <tr data-path="{{.Path}}" data-name="{{.Name}}" data-isdir="{{.IsDir}}" data-size="{{.RawSize}}" data-mod="{{.RawMod}}" {{if .IsEditable}}data-editable="true"{{end}}>
                    <td>
                        <a href="{{.Path}}" class="file-link">
                            <span class="icon">{{.Icon}}</span>
                            <span class="name">{{.Name}}</span>
                        </a>
                    </td>
                    <td class="size">{{.Size}}</td>
                    <td class="modified">{{.ModTime}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>
        </div>

        <footer>
            <div class="footer-left">
                <button class="footer-btn" onclick="toggleFooterMenu(event)" title="Settings">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 01-2.83 2.83l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 008.7 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83-2.83l.06-.06A1.65 1.65 0 004.6 15a1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 8.7a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 012.83-2.83l.06.06A1.65 1.65 0 009 4.6a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 2.83l-.06.06A1.65 1.65 0 0019.4 9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z"/></svg>
                </button>
                <div id="footerMenu" class="footer-menu" onclick="event.stopPropagation()">
                    <div class="footer-menu-item">
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="5"/><path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42"/></svg>
                        <select id="themeSelect" onchange="changeTheme(this.value)">
                            <option value="light">Catppuccin Latte</option>
                            <option value="catppuccin-mocha">Catppuccin Mocha</option>
                            <option value="dracula">Dracula</option>
                            <option value="nord">Nord</option>
                            <option value="solarized-dark">Solarized Dark</option>
                            <option value="solarized-light">Solarized Light</option>
                            <option value="one-dark">One Dark</option>
                            <option value="gruvbox">Gruvbox</option>
                            <option value="monokai-dimmed">Monokai Dimmed</option>
                            <option value="abyss">Abyss</option>
                            <option value="github-light">GitHub Light</option>
                            <option value="ibm-3278">IBM 3278 Retro</option>
                        </select>
                    </div>
                    <div class="footer-menu-separator"></div>
                    <button class="footer-menu-item" onclick="showAbout(); closeFooterMenu();">
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4M12 8h.01"/></svg>
                        About
                    </button>
                </div>
            </div>
            <div class="footer-right">
                <span id="itemCount">Items: {{len .Files}}</span>
            </div>
        </footer>
    </div>

    <div id="previewModal" class="preview-modal" onclick="closePreview()">
        <div class="preview-content" onclick="event.stopPropagation()">
            <div id="previewBody"></div>
        </div>
    </div>

    <div id="editorModal" class="preview-modal" onclick="closeEditor()">
        <div class="preview-content" onclick="event.stopPropagation()" style="max-width: 90%; max-height: 90%; display: flex; flex-direction: column; overflow: hidden;">
            <div style="display: flex; justify-content: space-between; align-items: center; padding: 10px; background: var(--hover-bg); border-radius: 4px; flex-shrink: 0;">
                <span id="editorFileName" style="font-weight: 600; color: var(--text-primary);"></span>
                <div>
                    <button class="btn-primary" onclick="saveFile()" style="margin-right: 10px; display: inline-flex; align-items: center; gap: 6px;"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M19 21H5a2 2 0 01-2-2V5a2 2 0 012-2h11l5 5v11a2 2 0 01-2 2z"/><polyline points="17 21 17 13 7 13 7 21"/><polyline points="7 3 7 8 15 8"/></svg>Save</button>
                    <button class="btn-secondary" onclick="closeEditor()" style="display: inline-flex; align-items: center; gap: 6px;"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M18 6L6 18M6 6l12 12"/></svg>Cancel</button>
                </div>
            </div>
            <textarea id="editor" style="flex: 1; min-height: 0; margin-top: 10px; resize: none;"></textarea>
        </div>
    </div>

    <div id="aboutModal" class="preview-modal" onclick="closeAbout()" style="padding-top: 10px;">
        <div class="preview-content" onclick="event.stopPropagation()" style="max-width: 600px; padding-top: 16px;">
            <div id="aboutBody" style="padding: 0 20px 20px;">
                <div style="display: inline-flex; align-items: center; gap: 10px; margin-bottom: 12px;">
                    <svg width="36" height="36" viewBox="0 0 48 48" xmlns="http://www.w3.org/2000/svg">
                        <path d="M8 24 Q16 12, 24 24 T40 24" stroke="var(--accent)" stroke-width="4" fill="none" stroke-linecap="round" opacity="0.7"/>
                        <path d="M8 30 Q16 20, 24 30 T40 30" stroke="var(--accent)" stroke-width="4" fill="none" stroke-linecap="round" opacity="0.5"/>
                        <circle cx="24" cy="24" r="8" fill="var(--accent)"/>
                        <circle cx="24" cy="24" r="5" fill="var(--bg-secondary)"/>
                        <path d="M24 20 L24 28 M24 20 L22 22 M24 20 L26 22" stroke="var(--accent)" stroke-width="2" stroke-linecap="round" fill="none"/>
                    </svg>
                    <span class="title">Go<span class="accent">Serve</span></span>
                    <span style="font-size: 11px; color: var(--text-secondary); margin-left: 4px;">{{.Version}}</span>
                </div>
                <p style="color: var(--text-secondary); margin-bottom: 20px;">Lightweight HTTP file server with WebDAV support</p>
                
                <h3 style="color: var(--accent); margin-bottom: 10px;">✨ Features</h3>
                <ul style="color: var(--text-secondary); line-height: 1.8; margin-bottom: 20px;">
                    <li>📁 Directory browsing with modern UI</li>
                    <li>📂 Directory upload with structure preservation</li>
                    <li>💾 WebDAV server - mount as network drive</li>
                    <li>🔍 Search & filter with wildcards (* and ?)</li>
                    <li>👁️ File preview (images, text, markdown, code)</li>
                    <li>📦 ZIP download for directories</li>
                    <li>🔒 Optional authentication (readonly/readwrite/all)</li>
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
                    <a href="https://github.com/staceyw/goserve" target="_blank" style="color: var(--accent); text-decoration: none;">📦 GitHub Repository</a><br>
                    <span style="color: var(--text-secondary); font-size: 0.9em; opacity: 0.7;">MIT License • Built with Go 1.25+</span>
                </p>
            </div>
        </div>
    </div>

    <div id="newFolderModal" class="preview-modal" onclick="closeNewFolderModal()">
        <div class="preview-content" onclick="event.stopPropagation()" style="max-width: 450px;">
            <span class="preview-close" onclick="closeNewFolderModal()">&times;</span>
            <h3 style="color: var(--accent); margin-top: 0;">New Folder</h3>
            <p style="color: var(--text-secondary); font-size: 13px; margin: 0;">Enter a name for the new folder</p>
            <input type="text" id="newFolderName" class="modal-input" placeholder="Folder name"
                   onkeydown="if(event.key==='Enter') createNewFolder()">
            <div class="modal-buttons">
                <button class="btn" onclick="closeNewFolderModal()">Cancel</button>
                <button class="btn-primary" onclick="createNewFolder()">Create</button>
            </div>
        </div>
    </div>

    <div id="dialogOverlay" class="dialog-overlay" onclick="dialogCancel()">
        <div class="dialog-box" onclick="event.stopPropagation()">
            <div class="dialog-title" id="dialogTitle"></div>
            <div class="dialog-message" id="dialogMessage"></div>
            <input type="text" class="dialog-input" id="dialogInput" style="display:none;"
                   onkeydown="if(event.key==='Enter') dialogOk()">
            <div class="dialog-buttons" id="dialogButtons"></div>
        </div>
    </div>

    <script>
        // --- Custom dialog system ---
        var _dialogResolve = null;

        function dialogCancel() {
            document.getElementById('dialogOverlay').classList.remove('active');
            if (_dialogResolve) { _dialogResolve(null); _dialogResolve = null; }
        }

        function dialogOk() {
            var overlay = document.getElementById('dialogOverlay');
            var input = document.getElementById('dialogInput');
            overlay.classList.remove('active');
            if (_dialogResolve) {
                var val = input.style.display === 'none' ? true : input.value;
                _dialogResolve(val);
                _dialogResolve = null;
            }
        }

        function showAlert(msg, title) {
            return new Promise(function(resolve) {
                _dialogResolve = resolve;
                document.getElementById('dialogTitle').textContent = title || '';
                document.getElementById('dialogTitle').style.display = title ? '' : 'none';
                document.getElementById('dialogMessage').textContent = msg;
                document.getElementById('dialogInput').style.display = 'none';
                document.getElementById('dialogButtons').innerHTML =
                    '<button class="dialog-btn primary" onclick="dialogOk()">OK</button>';
                document.getElementById('dialogOverlay').classList.add('active');
            });
        }

        function showConfirm(msg, title, danger) {
            return new Promise(function(resolve) {
                _dialogResolve = resolve;
                document.getElementById('dialogTitle').textContent = title || 'Confirm';
                document.getElementById('dialogTitle').style.display = '';
                document.getElementById('dialogMessage').textContent = msg;
                document.getElementById('dialogInput').style.display = 'none';
                var btnClass = danger ? 'danger' : 'primary';
                document.getElementById('dialogButtons').innerHTML =
                    '<button class="dialog-btn" onclick="dialogCancel()">Cancel</button>' +
                    '<button class="dialog-btn ' + btnClass + '" onclick="dialogOk()">' + (danger ? 'Delete' : 'OK') + '</button>';
                document.getElementById('dialogOverlay').classList.add('active');
            });
        }

        function showPrompt(msg, defaultVal, title) {
            return new Promise(function(resolve) {
                _dialogResolve = resolve;
                document.getElementById('dialogTitle').textContent = title || '';
                document.getElementById('dialogTitle').style.display = title ? '' : 'none';
                document.getElementById('dialogMessage').textContent = msg;
                var input = document.getElementById('dialogInput');
                input.style.display = '';
                input.value = defaultVal || '';
                document.getElementById('dialogButtons').innerHTML =
                    '<button class="dialog-btn" onclick="dialogCancel()">Cancel</button>' +
                    '<button class="dialog-btn primary" onclick="dialogOk()">OK</button>';
                document.getElementById('dialogOverlay').classList.add('active');
                setTimeout(function() { input.focus(); input.select(); }, 100);
            });
        }

        // Theme system
        function isDarkTheme(theme) {
            return theme !== 'light' && theme !== 'solarized-light' && theme !== 'github-light';
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
        if (savedTheme === 'dark' || savedTheme === 'goserve-dark') savedTheme = 'catppuccin-mocha';
        if (savedTheme === 'vs-dark') savedTheme = 'one-dark';
        changeTheme(savedTheme);

        // Search/filter with wildcard support
        function filterFiles() {
            const input = document.getElementById('searchBox');
            const filter = input.value;

            // Command mode: ":" prefix stops filtering
            if (filter.startsWith(':')) {
                input.style.borderColor = 'var(--accent)';
                return;
            }
            input.style.borderColor = '';

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
            updateItemCount();
        }

        function handleSearchKey(e) {
            var input = document.getElementById('searchBox');
            if (e.key === 'Enter' && input.value.startsWith(':')) {
                e.preventDefault();
                var cmd = input.value.substring(1).trim();
                if (!cmd) return;
                fetch('/_api/chdir', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({dir: cmd})
                })
                .then(function(r) { return r.json(); })
                .then(function(d) {
                    if (d.success) {
                        window.location.href = '/';
                    } else {
                        input.style.borderColor = '#f38ba8';
                        input.value = ':' + cmd + '  (' + d.error + ')';
                        setTimeout(function() {
                            input.value = ':' + cmd;
                            input.style.borderColor = 'var(--accent)';
                        }, 2000);
                    }
                })
                .catch(function() {
                    input.style.borderColor = '#f38ba8';
                });
                return;
            }
            if (e.key === 'Escape') {
                input.value = '';
                input.style.borderColor = '';
                input.blur();
                filterFiles();
            }
        }

        function updateItemCount() {
            var rows = document.querySelectorAll('#fileTable tbody tr');
            var visible = 0;
            rows.forEach(function(r) { if (r.style.display !== 'none') visible++; });
            document.getElementById('itemCount').textContent = 'Items: ' + visible;
        }

        // Sort table
        var currentSortCol = -1;
        var currentSortDir = 'asc';

        function sortTable(n) {
            var tbody = document.querySelector('#fileTable tbody');
            var rows = Array.from(tbody.querySelectorAll('tr'));

            // Determine direction
            if (currentSortCol === n) {
                currentSortDir = currentSortDir === 'asc' ? 'desc' : 'asc';
            } else {
                currentSortCol = n;
                currentSortDir = 'asc';
            }

            rows.sort(function(a, b) {
                var av, bv;
                if (n === 0) {
                    // Name: directories first, then alphabetical
                    var aDir = a.dataset.isdir === 'true' ? 0 : 1;
                    var bDir = b.dataset.isdir === 'true' ? 0 : 1;
                    if (aDir !== bDir) return aDir - bDir;
                    av = (a.dataset.name || '').toLowerCase();
                    bv = (b.dataset.name || '').toLowerCase();
                    return currentSortDir === 'asc' ? av.localeCompare(bv) : bv.localeCompare(av);
                } else if (n === 1) {
                    // Size: numeric
                    av = parseInt(a.dataset.size) || 0;
                    bv = parseInt(b.dataset.size) || 0;
                    return currentSortDir === 'asc' ? av - bv : bv - av;
                } else {
                    // Modified: numeric timestamp
                    av = parseInt(a.dataset.mod) || 0;
                    bv = parseInt(b.dataset.mod) || 0;
                    return currentSortDir === 'asc' ? av - bv : bv - av;
                }
            });

            // Re-append in order
            rows.forEach(function(r) { tbody.appendChild(r); });

            // Update header arrows
            var ths = document.querySelectorAll('#fileTable thead th');
            ths.forEach(function(th, i) {
                var arrow = th.querySelector('.sort-arrow');
                if (i === n) {
                    th.classList.add('sorted');
                    arrow.textContent = currentSortDir === 'asc' ? '↑' : '↓';
                } else {
                    th.classList.remove('sorted');
                    arrow.textContent = '';
                }
            });
        }

        // File operations
        function deleteFile(path, name) {
            showConfirm('Delete ' + name + '?', 'Delete', true).then(function(ok) {
                if (!ok) return;
                fetch('?delete=' + encodeURIComponent(path), { method: 'POST' })
                    .then(r => r.json())
                    .then(data => {
                        if (data.success) location.reload();
                        else showAlert('Error: ' + data.error);
                    });
            });
        }

        function renameFile(path, oldName) {
            showPrompt('Rename to:', oldName, 'Rename').then(function(newName) {
                if (!newName || newName === oldName) return;
                fetch('?rename=' + encodeURIComponent(path) + '&newname=' + encodeURIComponent(newName), { method: 'POST' })
                    .then(r => r.json())
                    .then(data => {
                        if (data.success) location.reload();
                        else showAlert('Error: ' + data.error);
                    });
            });
        }

        // Text editor
        var editor = null;
        var currentEditPath = '';

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
                .catch(err => showAlert('Error loading file: ' + err.message));
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
                    showAlert('File saved successfully!', 'Saved').then(function() { closeEditor(); });
                } else {
                    showAlert('Error: ' + data.error);
                }
            })
            .catch(err => showAlert('Error saving file: ' + err.message));
        }

        function closeEditor() {
            document.getElementById('editorModal').style.display = 'none';
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
            urlInput.setSelectionRange(0, 99999);

            try {
                navigator.clipboard.writeText(urlInput.value).then(() => {
                    showAlert('WebDAV URL copied to clipboard!', 'Copied');
                }).catch(() => {
                    document.execCommand('copy');
                    showAlert('WebDAV URL copied!', 'Copied');
                });
            } catch (err) {
                document.execCommand('copy');
                showAlert('WebDAV URL copied!', 'Copied');
            }
        }

        function escapeHtml(text) {
            const map = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' };
            return text.replace(/[&<>"']/g, m => map[m]);
        }

        // --- File list selection and navigation ---
        var selectedRows = [];
        var lastSelectedRow = null;

        function getVisibleRows() {
            var rows = Array.from(document.querySelectorAll('#fileTable tbody tr'));
            return rows.filter(r => r.style.display !== 'none');
        }

        function updateSelectionBar() {
            var bar = document.getElementById('selectionBar');
            var count = selectedRows.length;
            if (count > 0) {
                bar.classList.add('active');
                document.getElementById('selectionCount').textContent = count + ' selected';
                var single = count === 1;
                var editBtn = document.getElementById('selEditBtn');
                var renameBtn = document.getElementById('selRenameBtn');
                if (editBtn) editBtn.style.display = (single && selectedRows[0].dataset.editable) ? '' : 'none';
                if (renameBtn) renameBtn.style.display = single ? '' : 'none';
            } else {
                bar.classList.remove('active');
            }
        }

        function clearSelection() {
            selectedRows.forEach(r => r.classList.remove('selected'));
            selectedRows = [];
            updateSelectionBar();
        }

        function selectRow(tr, keepExisting) {
            if (!keepExisting) clearSelection();
            if (!tr.classList.contains('selected')) {
                tr.classList.add('selected');
                selectedRows.push(tr);
            }
            lastSelectedRow = tr;
            tr.scrollIntoView({block: 'nearest'});
            updateSelectionBar();
        }

        function selectRange(fromTr, toTr) {
            var visible = getVisibleRows();
            var i1 = visible.indexOf(fromTr);
            var i2 = visible.indexOf(toTr);
            if (i1 < 0 || i2 < 0) return;
            var start = Math.min(i1, i2), end = Math.max(i1, i2);
            clearSelection();
            for (var i = start; i <= end; i++) {
                visible[i].classList.add('selected');
                selectedRows.push(visible[i]);
            }
            lastSelectedRow = toTr;
            updateSelectionBar();
        }

        function navigateUp() {
            // Remember current folder name so parent page can select it
            var parts = window.location.pathname.replace(/\/+$/, '').split('/');
            var current = parts[parts.length - 1];
            if (current) sessionStorage.setItem('goserve_select', current);
            window.location.href = '../';
        }

        function openRow(tr) {
            var path = tr.dataset.path;
            var isDir = tr.dataset.isdir === 'true';
            var name = tr.dataset.name || '';
            if (!path) return;
            if (path === '../') {
                navigateUp();
                return;
            }
            if (isDir) {
                window.location.href = path;
            } else {
                // Trigger preview or download
                var ext = name.split('.').pop().toLowerCase();
                var images = ['jpg','jpeg','png','gif','svg','webp'];
                var previewable = ['txt','md','json','js','go','py','html','css','xml','log'];
                if (images.includes(ext)) {
                    document.getElementById('previewBody').innerHTML = '<img src="' + path + '" style="max-width:100%;height:auto;">';
                    document.getElementById('previewModal').style.display = 'block';
                } else if (ext === 'md') {
                    fetch(path + '?markdown=1').then(r => r.ok ? r.text() : Promise.reject('Failed'))
                        .then(html => { document.getElementById('previewBody').innerHTML = '<div class="markdown-body">' + html + '</div>'; document.getElementById('previewModal').style.display = 'block'; })
                        .catch(err => showAlert('Error: ' + err));
                } else if (previewable.includes(ext)) {
                    fetch(path).then(r => r.ok ? r.text() : Promise.reject('Failed'))
                        .then(text => { document.getElementById('previewBody').innerHTML = '<pre>' + escapeHtml(text) + '</pre>'; document.getElementById('previewModal').style.display = 'block'; })
                        .catch(err => showAlert('Error: ' + err));
                } else {
                    window.open(path, '_blank');
                }
            }
        }

        // Single-click: select row. Prevent <a> navigation.
        document.querySelector('#fileTable tbody')?.addEventListener('click', function(e) {
            // Don't intercept action button clicks

            e.preventDefault();
            var tr = e.target.closest('tr');
            if (!tr || !tr.dataset.path) return;

            if (e.shiftKey && lastSelectedRow) {
                selectRange(lastSelectedRow, tr);
            } else if (e.ctrlKey || e.metaKey) {
                if (tr.classList.contains('selected')) {
                    tr.classList.remove('selected');
                    selectedRows = selectedRows.filter(r => r !== tr);
                    updateSelectionBar();
                } else {
                    selectRow(tr, true);
                }
            } else {
                selectRow(tr, false);
            }
        });

        // Double-click: open file/folder
        document.querySelector('#fileTable tbody')?.addEventListener('dblclick', function(e) {

            var tr = e.target.closest('tr');
            if (!tr || !tr.dataset.path) return;
            e.preventDefault();
            openRow(tr);
        });

        // Keyboard navigation
        document.addEventListener('keydown', function(e) {
            // Close modals/menus on Escape
            if (e.key === 'Escape') {
                dialogCancel();
                closePreview();
                closeAbout();
                closeEditor();
                closeNewFolderModal();
                hideAllMenus();
                clearSelection();
                return;
            }

            // Don't handle keys when typing in inputs or modals open
            var tag = document.activeElement.tagName;
            if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
            if (document.querySelector('.preview-modal[style*="display: block"]')) return;
            if (document.getElementById('dialogOverlay').classList.contains('active')) return;

            if (e.key === 'ArrowLeft') {
                e.preventDefault();
                navigateUp();
                return;
            }

            if (e.key === 'Delete' && selectedRows.length > 0) {
                e.preventDefault();
                ctxDeleteSelected();
                return;
            }

            var visible = getVisibleRows();
            if (visible.length === 0) return;

            var currentIdx = lastSelectedRow ? visible.indexOf(lastSelectedRow) : -1;

            if (e.key === 'ArrowDown') {
                e.preventDefault();
                var next = currentIdx < visible.length - 1 ? currentIdx + 1 : currentIdx;
                if (next < 0) next = 0;
                if (e.shiftKey && lastSelectedRow) {
                    var tr = visible[next];
                    if (tr.classList.contains('selected') && next !== currentIdx) {
                        // Shrink selection: deselect current if moving back
                        lastSelectedRow.classList.remove('selected');
                        selectedRows = selectedRows.filter(r => r !== lastSelectedRow);
                    } else {
                        tr.classList.add('selected');
                        if (!selectedRows.includes(tr)) selectedRows.push(tr);
                    }
                    lastSelectedRow = tr;
                    tr.scrollIntoView({block: 'nearest'});
                    updateSelectionBar();
                } else {
                    selectRow(visible[next], false);
                }
            } else if (e.key === 'ArrowUp') {
                e.preventDefault();
                var prev = currentIdx > 0 ? currentIdx - 1 : 0;
                if (e.shiftKey && lastSelectedRow) {
                    var tr = visible[prev];
                    if (tr.classList.contains('selected') && prev !== currentIdx) {
                        lastSelectedRow.classList.remove('selected');
                        selectedRows = selectedRows.filter(r => r !== lastSelectedRow);
                    } else {
                        tr.classList.add('selected');
                        if (!selectedRows.includes(tr)) selectedRows.push(tr);
                    }
                    lastSelectedRow = tr;
                    tr.scrollIntoView({block: 'nearest'});
                    updateSelectionBar();
                } else {
                    selectRow(visible[prev], false);
                }
            } else if (e.key === 'ArrowRight') {
                if (lastSelectedRow && lastSelectedRow.dataset.isdir === 'true') {
                    e.preventDefault();
                    window.location.href = lastSelectedRow.dataset.path;
                }
            } else if (e.key === 'Enter') {
                if (lastSelectedRow) {
                    e.preventDefault();
                    openRow(lastSelectedRow);
                }
            } else if (e.key === 'Home') {
                e.preventDefault();
                if (visible.length > 0) selectRow(visible[0], false);
            } else if (e.key === 'End') {
                e.preventDefault();
                if (visible.length > 0) selectRow(visible[visible.length - 1], false);
            }
        });

        // Footer menu
        function toggleFooterMenu(e) {
            e.stopPropagation();
            var menu = document.getElementById('footerMenu');
            menu.classList.toggle('active');
        }
        function closeFooterMenu() {
            document.getElementById('footerMenu').classList.remove('active');
        }

        // Context menus
        function hideAllMenus() {
            document.querySelectorAll('.context-menu').forEach(m => m.classList.remove('show'));
            closeFooterMenu();
        }

        function showMenuAt(menu, x, y) {
            hideAllMenus();
            menu.classList.add('show');
            var mw = menu.offsetWidth, mh = menu.offsetHeight;
            menu.style.left = Math.min(x, window.innerWidth - mw - 4) + 'px';
            menu.style.top = Math.min(y, window.innerHeight - mh - 4) + 'px';
        }

        // Folder context menu (breadcrumb caret)
        function toggleContextMenu(e) {
            e.stopPropagation();
            var menu = document.getElementById('folderContextMenu');
            if (menu.classList.contains('show')) {
                hideAllMenus();
                return;
            }
            var rect = e.target.getBoundingClientRect();
            showMenuAt(menu, rect.left, rect.bottom + 2);
        }

        document.addEventListener('click', function() { hideAllMenus(); });

        // Right-click on table header → folder context menu
        document.querySelector('#fileTable thead')?.addEventListener('contextmenu', function(e) {
            if (!document.querySelector('#folderContextMenu .context-menu-item')) return;
            e.preventDefault();
            showMenuAt(document.getElementById('folderContextMenu'), e.clientX, e.clientY);
        });

        // Right-click on table row → row context menu
        document.querySelector('#fileTable tbody')?.addEventListener('contextmenu', function(e) {
            e.preventDefault();
            var tr = e.target.closest('tr');
            if (!tr || !tr.dataset.path) return;
            // Select row if not already selected
            if (!tr.classList.contains('selected')) {
                selectRow(tr, false);
            }
            // Show/hide items based on single vs multi selection
            var single = selectedRows.length === 1;
            var renameBtn = document.getElementById('ctxRename');
            var editBtn = document.getElementById('ctxEdit');
            if (renameBtn) renameBtn.style.display = single ? '' : 'none';
            if (editBtn) editBtn.style.display = (single && selectedRows[0].dataset.editable) ? '' : 'none';
            showMenuAt(document.getElementById('rowContextMenu'), e.clientX, e.clientY);
        });

        // Row context menu actions
        function ctxDownloadSelected() {
            hideAllMenus();
            if (selectedRows.length === 0) return;
            if (selectedRows.length === 1) {
                var path = selectedRows[0].dataset.path;
                var isDir = selectedRows[0].dataset.isdir === 'true';
                if (isDir) {
                    window.location.href = path + '?zip=1';
                } else {
                    // Direct file download
                    var a = document.createElement('a');
                    a.href = path;
                    a.download = selectedRows[0].dataset.name || '';
                    document.body.appendChild(a);
                    a.click();
                    document.body.removeChild(a);
                }
            } else {
                // Multi-file: POST paths to get a ZIP
                var paths = selectedRows.map(r => r.dataset.path);
                var form = document.createElement('form');
                form.method = 'POST';
                form.action = window.location.pathname + '?zipfiles=1';
                form.style.display = 'none';
                paths.forEach(p => {
                    var input = document.createElement('input');
                    input.type = 'hidden';
                    input.name = 'files';
                    input.value = p;
                    form.appendChild(input);
                });
                document.body.appendChild(form);
                form.submit();
                document.body.removeChild(form);
            }
        }

        function ctxEditSelected() {
            hideAllMenus();
            if (selectedRows.length !== 1) return;
            var tr = selectedRows[0];
            editFile(tr.dataset.path, tr.dataset.name);
        }

        function ctxRenameSelected() {
            hideAllMenus();
            if (selectedRows.length !== 1) return;
            var tr = selectedRows[0];
            renameFile(tr.dataset.path, tr.dataset.name);
        }

        function ctxCopyLink() {
            hideAllMenus();
            if (selectedRows.length === 0) return;
            var base = window.location.origin;
            var urls = selectedRows.map(r => base + r.dataset.path);
            var text = urls.join('\n');
            navigator.clipboard.writeText(text).then(function() {
                // Brief visual feedback
                var count = document.getElementById('selectionCount');
                if (count) { var orig = count.textContent; count.textContent = 'Link copied!'; setTimeout(function() { count.textContent = orig; }, 1500); }
            }).catch(function() {
                showPrompt('Copy link:', text, 'Copy Link');
            });
        }

        function copyFolderLink() {
            hideAllMenus();
            var url = window.location.origin + window.location.pathname;
            navigator.clipboard.writeText(url).then(function() {
                // silent copy
            }).catch(function() {
                showPrompt('Copy link:', url, 'Copy Link');
            });
        }

        function ctxDeleteSelected() {
            hideAllMenus();
            if (selectedRows.length === 0) return;
            var names = selectedRows.map(r => r.dataset.name || r.dataset.path);
            var msg = selectedRows.length === 1
                ? 'Delete ' + names[0] + '?'
                : 'Delete ' + selectedRows.length + ' items?\n' + names.join('\n');
            showConfirm(msg, 'Delete', true).then(function(ok) {
                if (!ok) return;
                var paths = selectedRows.map(r => r.dataset.path);
                var chain = Promise.resolve();
                paths.forEach(function(p) {
                    chain = chain.then(function() {
                        return fetch('?delete=' + encodeURIComponent(p), { method: 'POST' })
                            .then(r => r.json())
                            .then(data => { if (!data.success) showAlert('Error deleting ' + p + ': ' + data.error); });
                    });
                });
                chain.then(function() { location.reload(); });
            });
        }

        // New Folder modal
        function showNewFolderModal() {
            hideAllMenus();
            document.getElementById('newFolderName').value = '';
            document.getElementById('newFolderModal').style.display = 'block';
            setTimeout(() => document.getElementById('newFolderName').focus(), 100);
        }

        function closeNewFolderModal() {
            document.getElementById('newFolderModal').style.display = 'none';
        }

        function createNewFolder() {
            const name = document.getElementById('newFolderName').value.trim();
            if (!name) return;
            if (name.includes('/') || name.includes('\\') || name.includes('..')) {
                showAlert('Invalid folder name');
                return;
            }
            fetch(window.location.pathname + '?mkdir=' + encodeURIComponent(name), { method: 'POST' })
                .then(r => r.json())
                .then(data => {
                    if (data.success) { closeNewFolderModal(); location.reload(); }
                    else showAlert('Error: ' + data.error);
                })
                .catch(err => showAlert('Error creating folder: ' + err.message));
        }

        // File/folder upload via context menu
        function triggerFileUpload() {
            hideAllMenus();
            document.getElementById('fileInput')?.click();
        }

        function triggerFolderUpload() {
            hideAllMenus();
            document.getElementById('dirInput')?.click();
        }

        function uploadFiles(files) {
            const formData = new FormData();
            files.forEach(file => {
                const path = file.webkitRelativePath || file.name;
                formData.append('files', file, path);
            });
            fetch(window.location.pathname + '?upload=1', {
                method: 'POST',
                body: formData
            }).then(response => {
                if (response.ok) window.location.reload();
                else showAlert('Upload failed');
            }).catch(err => {
                showAlert('Upload error: ' + err.message);
            });
        }

        document.getElementById('fileInput')?.addEventListener('change', function(e) {
            const files = Array.from(e.target.files);
            if (files.length > 0) uploadFiles(files);
            e.target.value = '';
        });

        document.getElementById('dirInput')?.addEventListener('change', function(e) {
            const files = Array.from(e.target.files);
            if (files.length > 0) uploadFiles(files);
            e.target.value = '';
        });

        // Inject breadcrumb caret on last item (only if folder context menu has items)
        (function() {
            if (!document.querySelector('#folderContextMenu .context-menu-item')) return;
            const crumbs = document.querySelectorAll('.breadcrumb a');
            if (crumbs.length > 0) {
                const last = crumbs[crumbs.length - 1];
                const caret = document.createElement('button');
                caret.className = 'breadcrumb-caret';
                caret.textContent = '\u25BE';
                caret.title = 'Actions';
                caret.onclick = function(e) { toggleContextMenu(e); };
                last.insertAdjacentElement('afterend', caret);
            }
        })();

        // Auto-select folder we navigated up from
        (function() {
            var name = sessionStorage.getItem('goserve_select');
            if (!name) return;
            sessionStorage.removeItem('goserve_select');
            var rows = document.querySelectorAll('#fileTable tbody tr');
            for (var i = 0; i < rows.length; i++) {
                if (rows[i].dataset.name === name) {
                    selectRow(rows[i], false);
                    return;
                }
            }
        })();
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
		return "0 bytes"
	}
	if size < 1024 {
		return fmt.Sprintf("%d bytes", size)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	val := float64(size) / 1024
	for i := 0; i < len(units)-1; i++ {
		if val < 1024 {
			return fmt.Sprintf("%.1f %s", val, units[i])
		}
		val /= 1024
	}
	return fmt.Sprintf("%.1f %s", val, units[len(units)-1])
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

func dirHandler(tmpl *template.Template, verbose bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Log request
		if verbose {
			fmt.Printf("[%s] %s %s\n", r.Method, r.RemoteAddr, r.URL.Path)
		}

		baseDir := getBaseDir()
		urlPath := filepath.Clean(r.URL.Path)
		fullPath := filepath.Join(baseDir, urlPath)

		// Security check - prevent path traversal outside baseDir
		if !isUnderDir(fullPath, baseDir) {
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
			case "all":
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

		// Handle mkdir
		if r.URL.Query().Get("mkdir") != "" && r.Method == "POST" {
			if !canModify {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"success": false, "error": "Forbidden: Modify not allowed"}`)
				return
			}
			handleMkdir(w, r, fullPath)
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

		// Handle multi-file ZIP download
		if r.URL.Query().Get("zipfiles") != "" && r.Method == "POST" {
			handleMultiZipDownload(w, r, fullPath, baseDir)
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

			rawSize := int64(0)
			if !entry.IsDir() {
				rawSize = info.Size()
			}
			files = append(files, FileInfo{
				Name:       name,
				Path:       urlPath,
				Size:       size,
				ModTime:    info.ModTime().Format("2006-01-02 15:04:05"),
				IsDir:      entry.IsDir(),
				Icon:       getIcon(name, entry.IsDir()),
				IsEditable: !entry.IsDir() && isEditableFile(name),
				RawSize:    rawSize,
				RawMod:     info.ModTime().Unix(),
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
			Breadcrumbs: buildBreadcrumbs(r.URL.Path),
			CanUpload:   canUpload,
			CanModify:   canModify,
			Version:     version,
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

	if !isUnderDir(fullPath, baseDir) {
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

	if !isUnderDir(oldFullPath, baseDir) || !isUnderDir(newFullPath, baseDir) {
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

func handleMkdir(w http.ResponseWriter, r *http.Request, parentDir string) {
	dirName := r.URL.Query().Get("mkdir")

	if dirName == "" || strings.Contains(dirName, "/") || strings.Contains(dirName, "\\") || strings.Contains(dirName, "..") {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success": false, "error": "Invalid directory name"}`)
		return
	}

	newPath := filepath.Join(parentDir, dirName)

	if !strings.HasPrefix(newPath+string(filepath.Separator), parentDir+string(filepath.Separator)) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success": false, "error": "Invalid path"}`)
		return
	}

	if _, err := os.Stat(newPath); err == nil {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success": false, "error": "Directory already exists"}`)
		return
	}

	err := os.Mkdir(newPath, 0755)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		fmt.Fprintf(w, `{"success": false, "error": "%s"}`, err.Error())
	} else {
		fmt.Fprintf(w, `{"success": true}`)
	}
}

func handleEdit(w http.ResponseWriter, r *http.Request, fullPath, baseDir string) {
	// Security check
	if !isUnderDir(fullPath, baseDir) {
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

func handleMultiZipDownload(w http.ResponseWriter, r *http.Request, currentDir, baseDir string) {
	r.ParseForm()
	filePaths := r.Form["files"]
	if len(filePaths) == 0 {
		http.Error(w, "No files specified", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=download.zip")

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	for _, fp := range filePaths {
		// Resolve relative to current directory
		fullPath := filepath.Join(currentDir, filepath.Base(fp))

		// Security check
		if !isUnderDir(fullPath, baseDir) {
			continue
		}

		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}

		if info.IsDir() {
			// Walk directory
			filepath.Walk(fullPath, func(path string, fi os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				relPath, _ := filepath.Rel(currentDir, path)
				header, err := zip.FileInfoHeader(fi)
				if err != nil {
					return err
				}
				header.Name = relPath
				header.Method = zip.Deflate
				if fi.IsDir() {
					header.Name += "/"
				}
				writer, err := zipWriter.CreateHeader(header)
				if err != nil {
					return err
				}
				if !fi.IsDir() {
					file, err := os.Open(path)
					if err != nil {
						return err
					}
					defer file.Close()
					io.Copy(writer, file)
				}
				return nil
			})
		} else {
			// Single file
			header, err := zip.FileInfoHeader(info)
			if err != nil {
				continue
			}
			header.Name = filepath.Base(fullPath)
			header.Method = zip.Deflate
			writer, err := zipWriter.CreateHeader(header)
			if err != nil {
				continue
			}
			file, err := os.Open(fullPath)
			if err != nil {
				continue
			}
			io.Copy(writer, file)
			file.Close()
		}
	}
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
		fmt.Fprintf(os.Stderr, "GoServe - Lightweight HTTP File Server\n\n")
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
		fmt.Fprintf(os.Stderr, "  Enable uploads:\n")
		fmt.Fprintf(os.Stderr, "    go run main.go -permlevel readwrite\n\n")
		fmt.Fprintf(os.Stderr, "  Full file management (upload + delete/rename):\n")
		fmt.Fprintf(os.Stderr, "    go run main.go -permlevel all\n\n")
		fmt.Fprintf(os.Stderr, "  Per-user authentication:\n")
		fmt.Fprintf(os.Stderr, "    go run main.go -logins logins.txt\n\n")
		fmt.Fprintf(os.Stderr, "  Verbose mode (log every request):\n")
		fmt.Fprintf(os.Stderr, "    go run main.go -verbose\n\n")
		fmt.Fprintf(os.Stderr, "  Combined example:\n")
		fmt.Fprintf(os.Stderr, "    go run main.go -listen :8000 -dir /var/www -permlevel all\n\n")
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
	verbose := flag.Bool("verbose", false, "Log every HTTP request to the console")
	permLevel := flag.String("permlevel", "readonly", "Permission level: readonly, readwrite, all")
	maxSize := flag.Int64("maxsize", 100, "Max upload size in MB")
	loginFile := flag.String("logins", "", "Enable authentication with login file (format: username:password:permission)")
	flag.Parse()

	if len(listenAddrs) == 0 {
		listenAddrs = stringSlice{"localhost:8080"}
	}

	// Set permissions from -permlevel
	switch *permLevel {
	case "readonly":
		allowUpload = false
		allowModify = false
	case "readwrite":
		allowUpload = true
		allowModify = false
	case "all":
		allowUpload = true
		allowModify = true
	default:
		log.Fatalf("Invalid -permlevel %q. Valid: readonly, readwrite, all", *permLevel)
	}
	maxUploadSize = *maxSize * 1024 * 1024

	// Load users if authentication is enabled (ignored if -permlevel is not readonly)
	if *loginFile != "" && *permLevel == "readonly" {
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
			if err != nil {
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
	setBaseDir(absPath)
	handler := dirHandler(tmpl, *verbose)
	if requireAuth {
		handler = authMiddleware(handler)
	}
	http.HandleFunc("/", gzipMiddleware(handler))

	// Change directory API
	http.HandleFunc("/_api/chdir", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Dir string `json:"dir"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"success":false,"error":"Invalid request"}`)
			return
		}
		newPath, err := filepath.Abs(req.Dir)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"success":false,"error":"Invalid path"}`)
			return
		}
		info, err := os.Stat(newPath)
		if err != nil || !info.IsDir() {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"success":false,"error":"Directory does not exist"}`)
			return
		}
		setBaseDir(newPath)
		webdavHandler.FileSystem = webdav.Dir(newPath)
		fmt.Printf("📂 Changed directory: %s\n", newPath)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success":true,"dir":"%s"}`, strings.ReplaceAll(newPath, `\`, `\\`))
	})

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
	fmt.Printf("\nGoServe %s\n", version)
	fmt.Printf("📂 Serving: %s\n", absPath)
	fmt.Printf("⏰ Started: %s\n", time.Now().Format("2006-01-02 15:04:05"))

	fmt.Printf("\n⚙️  Permissions: %s\n", *permLevel)
	if requireAuth {
		fmt.Printf("   Auth: %d users\n", len(users))
	}
	if allowUpload {
		fmt.Printf("   Max upload size: %dMB\n", *maxSize)
	}

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

	fmt.Println("\n💡 Press Ctrl+C to stop")
	fmt.Println()

	// Start server on all listeners
	errc := make(chan error, 1)
	for _, ln := range listeners {
		go func(l net.Listener) {
			errc <- http.Serve(l, nil)
		}(ln)
	}
	log.Fatal(<-errc)
}
