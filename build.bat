@echo off
rem ============================================================
rem  Younblock build script (run on Windows)
rem  Builds every version from this one source tree:
rem    - younblock.exe / younblock-core.exe       (64-bit Windows, new screen)
rem    - younblock-x86.exe / younblock-core-x86.exe (32-bit Windows)
rem    - younblock-classic.exe / -x86.exe         (the classic simple screen)
rem  and prints the exact output paths.
rem  (The Linux single-executables are built by build.sh on Linux.)
rem ============================================================
setlocal
cd /d "%~dp0"

echo === Younblock build (all Windows versions) ===
echo.

echo [1/5] Building the headless engine, 64-bit (younblock-core.exe)...
if not exist dist mkdir dist
go build -trimpath -ldflags "-s -w" -o dist\younblock-core.exe .\cmd\core
if errorlevel 1 goto :fail

echo [2/5] Building the headless engine, 32-bit (younblock-core-x86.exe)...
set GOARCH=386
go build -trimpath -ldflags "-s -w" -o dist\younblock-core-x86.exe .\cmd\core
set GOARCH=
if errorlevel 1 goto :fail

echo [3/5] Building the app, 64-bit (younblock.exe)...
call wails build -platform windows/amd64 -trimpath -ldflags "-s -w"
if errorlevel 1 goto :fail
if not exist build\bin\younblock.exe goto :fail
copy /y build\bin\younblock.exe dist\younblock.exe >nul
if errorlevel 1 goto :fail

echo [4/5] Building the app, 32-bit (younblock-x86.exe)...
call wails build -platform windows/386 -trimpath -ldflags "-s -w" -o younblock-x86.exe
if errorlevel 1 goto :fail
if not exist build\bin\younblock-x86.exe goto :fail
copy /y build\bin\younblock-x86.exe dist\younblock-x86.exe >nul
if errorlevel 1 goto :fail

echo [5/7] Building the classic app, 64-bit (younblock-classic.exe)...
set VITE_UI_VARIANT=classic
call wails build -platform windows/amd64 -trimpath -ldflags "-s -w -X main.windowSize=360x760" -o younblock-classic.exe
if errorlevel 1 goto :fail
if not exist build\bin\younblock-classic.exe goto :fail
copy /y build\bin\younblock-classic.exe dist\younblock-classic.exe >nul
if errorlevel 1 goto :fail

echo [6/7] Building the classic app, 32-bit (younblock-classic-x86.exe)...
set GOARCH=386
call wails build -platform windows/386 -trimpath -ldflags "-s -w -X main.windowSize=360x760" -o younblock-classic-x86.exe
set GOARCH=
set VITE_UI_VARIANT=
if errorlevel 1 goto :fail
if not exist build\bin\younblock-classic-x86.exe goto :fail
copy /y build\bin\younblock-classic-x86.exe dist\younblock-classic-x86.exe >nul
if errorlevel 1 goto :fail

echo [7/7] Done. Output files:
echo.
for %%F in (dist\younblock-core.exe dist\younblock-core-x86.exe dist\younblock.exe dist\younblock-x86.exe dist\younblock-classic.exe dist\younblock-classic-x86.exe) do (
    echo   %CD%\%%F
)
echo.
echo SHA-256 checksums (verify downloads with these):
for %%F in (dist\younblock-core.exe dist\younblock-core-x86.exe dist\younblock.exe dist\younblock-x86.exe dist\younblock-classic.exe dist\younblock-classic-x86.exe) do (
    certutil -hashfile "%%F" SHA256 | findstr /v /i "hash certutil"
)
echo.
echo Build finished successfully.
goto :eof

:fail
echo.
echo BUILD FAILED. See the messages above.
exit /b 1
