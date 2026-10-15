@echo off
rem ============================================================
rem  Younblock one-click build (build mode)
rem
rem  Double-click this file. All the logic lives in setup.ps1,
rem  which must sit next to this file - keep them together
rem  (the younblock-setup zip from the Releases page has both).
rem
rem  It installs anything missing (Git, Go, Node.js LTS, WebView2,
rem  the Wails CLI) - using mirrors when the usual sites are
rem  unreachable - then builds every exe and opens the dist folder.
rem ============================================================
if not exist "%~dp0setup.ps1" goto :nops1
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0setup.ps1" -Mode build
echo.
echo Press any key to close this window...
pause >nul
exit /b %errorlevel%

:nops1
echo.
echo SETUP STOPPED: setup.ps1 was not found next to this file.
echo The two files travel together: download the "younblock-setup"
echo zip from the Releases page, extract ALL of its files into one
echo folder, then double-click this file again.
echo.
pause
exit /b 1
