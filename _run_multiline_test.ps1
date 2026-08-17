cd "d:\code\filebrowser-release-20260814 (2)"
powershell -NoProfile -ExecutionPolicy Bypass -File .\_probe_args.ps1 -Mode Run `
  -DbFile .\filebrowser.db `
  -JRootPath "D:\BaiduNetdiskDownload\图纸\图纸" `
  -KBindAddr 127.0.0.1 `
  -QListenPort 8080
