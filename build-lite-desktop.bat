@echo off
rem build-lite-desktop.bat - the lite desktop edition on a Windows
rem machine (younblock-lite-desktop, DECISIONS.md D114). The whole
rem build needs ONLY Go on PATH - no Node, no WebView2, no wails CLI:
rem the lite UI is plain Win32 GDI, so this is the one edition whose
rem build is a single command. The icon and version info are committed
rem (cmd\younblock-lite\rsrc_windows_*.syso); go build links them.
rem
rem Output: dist\younblock-lite-desktop.exe (+ the 32-bit x86 twin).

setlocal
cd /d "%~dp0"

where go >nul 2>nul
if errorlevel 1 (
  echo Go is not on PATH - install it from https://go.dev/dl/ first.
  exit /b 1
)

if not exist dist mkdir dist

echo Building younblock-lite-desktop.exe (64-bit)...
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -trimpath -ldflags "-s -w -H windowsgui" -o dist\younblock-lite-desktop.exe .\cmd\younblock-lite
if errorlevel 1 exit /b 1

echo Building younblock-lite-desktop-x86.exe (32-bit)...
set GOARCH=386
go build -trimpath -ldflags "-s -w -H windowsgui" -o dist\younblock-lite-desktop-x86.exe .\cmd\younblock-lite
if errorlevel 1 exit /b 1

echo Done:
dir /b dist\younblock-lite-desktop*.exe
