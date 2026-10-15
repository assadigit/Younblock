@echo off
setlocal EnableExtensions
title Younblock offline setup

rem ============================================================
rem  Younblock offline setup - needs NO internet at all.
rem
rem  Everything it needs is packed in the "tools" folder next to
rem  this file (the Go toolchain, Node.js, the Wails CLI) and in
rem  the source tree around it (the app, its Go dependencies and
rem  the window building blocks). Double-click this file.
rem
rem    run it plain        - start the app live (wails dev)
rem    run "setup-offline build" - build the exes instead
rem ============================================================

cd /d "%~dp0"
set "TOOLS=%LOCALAPPDATA%\YounblockTools"
set "GOBIN=%USERPROFILE%\go\bin"

if not exist "tools\go1.27.2.windows-amd64.zip" goto :notpack
if not exist "tools\node-v22.23.3-win-x64.zip" goto :notpack
if not exist "tools\wails.exe" goto :notpack
if not exist "frontend\node_modules" goto :notpack

echo.
echo === Younblock offline setup (no internet needed) ===
echo.

rem --- [1/4] Go ---------------------------------------------------
set "USEPACKEDGO="
where go >nul 2>&1
if errorlevel 1 goto :go_packed
set "GOVER="
for /f "tokens=3" %%v in ('go version') do set "GOVER=%%v"
set "GOMAJOR="
set "GOMINOR="
set "GOVER=%GOVER:go=%"
for /f "tokens=1 delims=." %%a in ("%GOVER%") do set "GOMAJOR=%%a"
for /f "tokens=2 delims=." %%b in ("%GOVER%") do set "GOMINOR=%%b"
if not defined GOMAJOR goto :go_packed
if not defined GOMINOR goto :go_packed
if %GOMAJOR% GTR 1 goto :go_ok
if %GOMAJOR% EQU 1 if %GOMINOR% GEQ 26 goto :go_ok
echo [1/4] Your installed Go is older than 1.26 - using the packed one.
:go_packed
if not exist "%TOOLS%\go\bin\go.exe" (
    echo [1/4] Unpacking Go - one time only - please wait...
    mkdir "%TOOLS%" 2>nul
    tar -xf "tools\go1.27.2.windows-amd64.zip" -C "%TOOLS%"
    if errorlevel 1 goto :fail
)
set "PATH=%TOOLS%\go\bin;%PATH%"
set "USEPACKEDGO=1"
:go_ok
if not defined USEPACKEDGO echo [1/4] Go ok

rem --- [2/4] Node.js ----------------------------------------------
set "USEPACKEDNODE="
where node >nul 2>&1
if errorlevel 1 goto :node_packed
set "NODE_MAJOR="
for /f "usebackq delims=" %%v in (`node -p "process.versions.node.split('.')[0]"`) do set "NODE_MAJOR=%%v"
if not defined NODE_MAJOR goto :node_packed
if %NODE_MAJOR% GEQ 20 goto :node_ok
echo [2/4] Your installed Node.js is older than 20 - using the packed one.
:node_packed
if not exist "%TOOLS%\nodejs\node.exe" (
    echo [2/4] Unpacking Node.js - one time only - please wait...
    mkdir "%TOOLS%" 2>nul
    if exist "%TOOLS%\node-extract" rmdir /s /q "%TOOLS%\node-extract"
    mkdir "%TOOLS%\node-extract"
    tar -xf "tools\node-v22.23.3-win-x64.zip" -C "%TOOLS%\node-extract"
    if errorlevel 1 goto :fail
    for /d %%D in ("%TOOLS%\node-extract\node-*") do move /y "%%D" "%TOOLS%\nodejs" >nul
    rmdir /s /q "%TOOLS%\node-extract"
)
set "PATH=%TOOLS%\nodejs;%PATH%"
set "USEPACKEDNODE=1"
:node_ok
if not defined USEPACKEDNODE echo [2/4] Node.js ok

rem --- [3/4] the Wails CLI ----------------------------------------
mkdir "%GOBIN%" 2>nul
copy /y "tools\wails.exe" "%GOBIN%\wails.exe" >nul
if errorlevel 1 goto :fail
set "PATH=%GOBIN%;%PATH%"
echo [3/4] Wails CLI ok

rem --- [4/4] remember the tools for future windows ----------------
powershell -NoProfile -Command "$p=[Environment]::GetEnvironmentVariable('Path','User'); $a=$env:LOCALAPPDATA+'\YounblockTools\go\bin;'+$env:LOCALAPPDATA+'\YounblockTools\nodejs;'+$env:USERPROFILE+'\go\bin'; if($p -notlike '*YounblockTools*'){[Environment]::SetEnvironmentVariable('Path', $p.TrimEnd(';')+';'+$a,'User')}"
echo [4/4] Done - the tools are ready.

rem --- run --------------------------------------------------------
echo.
if /i "%~1"=="build" goto :runbuild
echo === Starting the app: wails dev ===
echo.
echo   Keep this window OPEN while the app runs.
echo   The app window - 360 x 760 - opens by itself.
echo   To stop the app: close this window.
echo.
call wails dev
echo.
echo The app has stopped.
goto :the_end

:runbuild
echo === Building every exe file ===
echo.
if not exist "build.bat" goto :nobuild
call build.bat
if errorlevel 1 goto :fail
if not exist "dist\younblock.exe" goto :fail
echo.
echo Opening the folder with the finished files...
%SystemRoot%\explorer.exe "%CD%\dist"
goto :the_end

:nobuild
echo SETUP STOPPED: build.bat was not found - the pack is incomplete.
goto :the_end

:notpack
echo.
echo SETUP STOPPED: this folder is not a complete offline pack.
echo It needs the "tools" folder (go zip, node zip, wails.exe) and the
echo full source with frontend\node_modules - everything that ships in
echo the "Younblock-offline" zip from the Releases page. Extract that
echo zip whole and run the setup-offline.bat inside it.
goto :the_end

:fail
echo.
echo SETUP FAILED. Read the messages above - they say what went wrong.
goto :the_end

:the_end
echo.
echo Press any key to close this window...
pause >nul
endlocal
