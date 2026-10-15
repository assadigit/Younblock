@echo off
setlocal EnableExtensions
title Younblock - one-click setup and run

rem ============================================================
rem  Younblock one-click dev setup (Windows 10 1709+ / Windows 11)
rem
rem  What it does:
rem    [1/8] checks winget - the Windows package manager
rem    [2/8] installs Git if missing
rem    [3/8] installs Go if missing
rem    [4/8] installs Node.js LTS if missing - checks the version
rem    [5/8] checks the WebView2 part of Windows
rem    [6/8] installs the Wails CLI if missing
rem    [7/8] gets the code - clone or git pull
rem    [8/8] installs the window's building blocks, then runs: wails dev
rem
rem  How to use: double-click this file.
rem  Keep this window OPEN while the app runs.
rem  First run downloads a lot - expect 5 to 15 minutes.
rem  If Windows asks for permission - the UAC screen - click Yes.
rem  To stop the app: just close this window.
rem ============================================================

set "REPO_URL=https://github.com/assadigit/younblock.git"
set "CLONE_DIR=Younblock"
set "GIT_TERMINAL_PROMPT=0"

cd /d "%~dp0"

echo.
echo === Younblock one-click setup and run ===
echo.
echo   Keep this window OPEN while the app runs.
echo   When Windows asks for permission - UAC - click Yes.
echo   To stop the app later: just close this window.
echo.

rem --- [1/8] winget ------------------------------------------------
where winget >nul 2>&1
if errorlevel 1 goto :no_winget
echo [1/8] winget ok

rem --- [2/8] Git ---------------------------------------------------
where git >nul 2>&1
if not errorlevel 1 goto :git_ok
echo [2/8] Installing Git - please wait...
winget install --id Git.Git -e --silent --accept-package-agreements --accept-source-agreements
call :RefreshPath
where git >nul 2>&1
if errorlevel 1 goto :git_fail
:git_ok
echo [2/8] Git ok

rem --- [3/8] Go ----------------------------------------------------
where go >nul 2>&1
if not errorlevel 1 goto :go_ok
echo [3/8] Installing Go - please wait...
winget install --id GoLang.Go -e --silent --accept-package-agreements --accept-source-agreements
call :RefreshPath
where go >nul 2>&1
if errorlevel 1 goto :go_fail
:go_ok
echo [3/8] Go ok

rem --- [4/8] Node.js LTS + npm -------------------------------------
where node >nul 2>&1
if not errorlevel 1 goto :node_ok
echo [4/8] Installing Node.js LTS - please wait...
winget install --id OpenJS.NodeJS.LTS -e --silent --accept-package-agreements --accept-source-agreements
call :RefreshPath
where node >nul 2>&1
if errorlevel 1 goto :node_fail
:node_ok
echo [4/8] Node.js ok
set "NODE_MAJOR="
for /f "usebackq delims=" %%v in (`node -p "process.versions.node.split('.')[0]"`) do set "NODE_MAJOR=%%v"
if "%NODE_MAJOR%"=="" goto :node_fail
if %NODE_MAJOR% LSS 20 goto :node_old
where npm >nul 2>&1
if errorlevel 1 goto :node_fail
echo [4/8] npm ok - Node %NODE_MAJOR%

rem --- [5/8] WebView2 part of Windows ------------------------------
reg query "HKLM\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" /v pv >nul 2>&1
if not errorlevel 1 goto :wv2_ok
reg query "HKCU\SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" /v pv >nul 2>&1
if not errorlevel 1 goto :wv2_ok
echo [5/8] Installing the WebView2 part of Windows - please wait...
winget install --id Microsoft.EdgeWebView2Runtime -e --silent --accept-package-agreements --accept-source-agreements
reg query "HKLM\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" /v pv >nul 2>&1
if not errorlevel 1 goto :wv2_ok
reg query "HKCU\SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" /v pv >nul 2>&1
if not errorlevel 1 goto :wv2_ok
echo [5/8] WARNING: WebView2 not found and could not be installed.
echo        If the app window turns out blank, install "Microsoft Edge
echo        WebView2 Runtime" from Microsoft, then run this file again.
goto :wv2_done
:wv2_ok
echo [5/8] WebView2 ok
:wv2_done

rem --- [6/8] Wails CLI ---------------------------------------------
set "PATH=%PATH%;%USERPROFILE%\go\bin"
where wails >nul 2>&1
if not errorlevel 1 goto :wails_ok
echo [6/8] Installing the Wails CLI - it compiles with Go - please wait...
go install github.com/wailsapp/wails/v2/cmd/wails@latest
if errorlevel 1 goto :wails_fail
where wails >nul 2>&1
if errorlevel 1 goto :wails_fail
:wails_ok
echo [6/8] Wails CLI ok

rem --- [7/8] the code - clone it or update it ----------------------
set "REPO="
if exist "%~dp0wails.json" set "REPO=%~dp0"
if not defined REPO if exist "%~dp0%CLONE_DIR%\wails.json" set "REPO=%~dp0%CLONE_DIR%"
if defined REPO goto :repo_ready
echo [7/8] Cloning Younblock from GitHub - please wait...
echo        The repository is private, so GitHub may ask you to log in.
git clone "%REPO_URL%" "%~dp0%CLONE_DIR%"
if errorlevel 1 goto :clone_fail
set "REPO=%~dp0%CLONE_DIR%"
goto :repo_ready
:repo_ready
echo [7/8] Code folder: %REPO%
pushd "%REPO%"
if not exist ".git" goto :repo_pulled
echo [7/8] Updating the code - git pull...
git pull --ff-only >nul 2>&1
if errorlevel 1 (
    echo        Could not auto-update - continuing with the code you have.
) else (
    echo        Code is up to date.
)
:repo_pulled
popd

rem --- [8/8] window building blocks, then run ----------------------
pushd "%REPO%frontend"
if not exist "node_modules" goto :fe_first
echo [8/8] Checking the window's building blocks - quick...
call npm install --no-fund --no-audit
if errorlevel 1 goto :npm_fail
goto :fe_done
:fe_first
echo [8/8] Installing the window's building blocks - first time - please wait...
call npm ci --no-fund --no-audit
if errorlevel 1 goto :npm_fail
:fe_done
popd
echo [8/8] Building blocks ok

echo.
echo === Starting the app: wails dev ===
echo.
echo   The first build takes a few minutes - let it work.
echo   The app window - 360 x 760 - opens by itself.
echo   To stop the app: close this window.
echo.
pushd "%REPO%"
call wails dev
popd
echo.
echo The app has stopped.
echo.
echo Tip: to build the everyday exe files instead of dev mode,
echo run build.bat in this same folder.
goto :the_end

rem ============================================================
rem  helpers
rem ============================================================

:RefreshPath
rem Reload PATH from the registry - picks up tools installed
rem by winget a moment ago without restarting this window.
for /f "usebackq delims=" %%p in (`powershell -NoProfile -Command "[Environment]::GetEnvironmentVariable('Path','Machine')+';'+[Environment]::GetEnvironmentVariable('Path','User')"`) do set "PATH=%%p"
goto :eof

:no_winget
echo.
echo SETUP STOPPED: "winget" was not found on this PC.
echo winget comes with Windows 10 1709+ and Windows 11 as "App Installer".
echo Fix: open the Microsoft Store - search "App Installer" - press Update -
echo then run this file again.
goto :the_end

:git_fail
echo.
echo SETUP STOPPED: Git did not install. Read the messages above,
echo fix the problem, then run this file again.
goto :the_end

:go_fail
echo.
echo SETUP STOPPED: Go did not install. Read the messages above,
echo fix the problem, then run this file again.
goto :the_end

:node_fail
echo.
echo SETUP STOPPED: Node.js did not install or is broken. Read the
echo messages above, fix the problem, then run this file again.
goto :the_end

:node_old
echo.
echo SETUP STOPPED: your Node.js is version %NODE_MAJOR%, but this app
echo needs Node 20 or newer.
echo Fix: open Settings - Apps - uninstall the old Node.js - then run
echo this file again and it will install the current one.
goto :the_end

:wails_fail
echo.
echo SETUP STOPPED: the Wails CLI did not install. Check your internet
echo connection, then run this file again.
goto :the_end

:clone_fail
echo.
echo SETUP STOPPED: could not download the code from GitHub.
echo The repository is private: when GitHub asks, log in with your
echo account and allow it. Then run this file again.
goto :the_end

:npm_fail
popd
echo.
echo SETUP STOPPED: the window's building blocks did not install.
echo Check your internet connection, then run this file again.
goto :the_end

:the_end
echo.
echo Press any key to close this window...
pause >nul
endlocal
