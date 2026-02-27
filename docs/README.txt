goserve - Lightweight HTTP File Server
======================================

Single binary, 12 themes, WebDAV support, no dependencies.

Quick Start
-----------
  goserve                        Serve current directory (read-only)
  goserve -dir /path/to/folder   Serve a specific directory
  goserve -permlevel readwrite   Enable uploads
  goserve -permlevel all         Full file management (upload/delete/rename/edit)
  goserve -listen :3000          Custom port

Then open http://localhost:8080 in your browser.

Flags
-----
  -listen      Address to listen on (default localhost:8080, repeatable)
  -dir         Directory to serve (default .)
  -permlevel   Permission level: readonly, readwrite, all (default readonly)
  -maxsize     Max upload size in MB (default 100)
  -logins      Path to authentication file
  -verbose     Log every HTTP request to the console

Permission Levels
-----------------
  readonly     Browse and view files
  readwrite    Browse, view, and upload files
  all          Browse, view, upload, delete, rename, and edit files

Authentication
--------------
  goserve -logins logins.txt

  Login file format (one user per line):
    username:password:permission

  Example:
    admin:secret:all
    user:pass123:readwrite
    guest:guest:readonly

WebDAV
------
Built-in WebDAV server at /webdav/

  Windows:  File Explorer > Map network drive > http://localhost:8080/webdav/
  macOS:    Finder > Go > Connect to Server > http://localhost:8080/webdav/
  Linux:    sudo mount -t davfs http://localhost:8080/webdav/ /mnt/goserve

Linux Service (systemd)
----------------------
Install as a background service (e.g., Raspberry Pi):

  curl -fsSL https://raw.githubusercontent.com/staceyw/goserve/main/scripts/install-service.sh | sudo bash

The script prompts for directory, port, and permissions, then sets up a
systemd service that starts on boot. After install, manage with:

  sudo systemctl status goserve       # check status
  sudo systemctl stop goserve         # stop
  sudo systemctl restart goserve      # restart
  journalctl -u goserve -f            # view logs

To change startup flags, edit /etc/systemd/system/goserve.service then:
  sudo systemctl daemon-reload && sudo systemctl restart goserve

More Info
---------
  https://github.com/staceyw/goserve
