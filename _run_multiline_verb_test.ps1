cd "d:\code\filebrowser-release-20260814 (2)"
powershell -NoProfile -ExecutionPolicy Bypass -File .\_probe_args.ps1 -Mode Run `
  -d .\filebrowser.db `
  -r "D:\BaiduNetdiskDownload\图纸\图纸" `
  -a 127.0.0.1 `
  -p 8080 `
  -l debug
