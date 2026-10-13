@echo off
rem ============================================================
rem  Younblock build script (run on Windows)
rem  Builds younblock-core.exe and the younblock.exe app and
rem  prints the exact output paths.
rem ============================================================
setlocal
cd /d "%~dp0"

echo === Younblock build ===
echo.

echo [1/3] Building the headless engine (younblock-core.exe)...
if not exist dist mkdir dist
go build -trimpath -ldflags "-s -w" -o dist\younblock-core.exe .\cmd\core
if errorlevel 1 goto :fail

echo [2/3] Building the app (younblock.exe)...
call wails build -platform windows/amd64 -trimpath -ldflags "-s -w"
if errorlevel 1 goto :fail
if not exist build\bin\younblock.exe goto :fail
copy /y build\bin\younblock.exe dist\younblock.exe >nul
if errorlevel 1 goto :fail

echo [3/3] Done. Output files:
echo.
for %%F in (dist\younblock-core.exe dist\younblock.exe) do (
    echo   %CD%\%%F
)
echo.
echo SHA-256 checksums (verify downloads with these):
for %%F in (dist\younblock-core.exe dist\younblock.exe) do (
    certutil -hashfile "%%F" SHA256 | findstr /v /i "hash certutil"
)
echo.
echo Build finished successfully.
goto :eof

:fail
echo.
echo BUILD FAILED. See the messages above.
exit /b 1
