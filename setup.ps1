# Younblock one-click setup - the engine behind setup-dev.bat and
# setup-build.bat. Double-click those files; do not run this directly
# unless you know why.
#
# It survives filtered networks: every dependency has mirrors, the first
# one that answers is used, and the code can arrive three different ways
# (git clone, token download, or your already-present copy).
#
# Mode dev   : install anything missing, then run the app live (wails dev)
# Mode build : install anything missing, then build every exe (build.bat)
param(
    [ValidateSet('dev', 'build')]
    [string]$Mode = 'dev'
)

$ErrorActionPreference = 'Continue'

# ---------------------------------------------------------------------------
# Configuration - the only lines anyone should ever need to edit.
# ---------------------------------------------------------------------------

# OPTIONAL: your GitHub token, for downloading this private repository
# without the browser login. Get one at github.com > Settings >
# Developer settings > Tokens (classic), with the "repo" permission.
# Leave empty ('') to log in with your browser when GitHub asks.
# Never share a copy of this file with the token filled in.
$GH_TOKEN   = ''
$SourceTag  = 'v0.7.2'    # used only by the no-git download path

# Mirrors, tried in order, first one that answers wins.
$GoProxyMirrors = @(
    'https://proxy.golang.org',
    'https://goproxy.io',
    'https://goproxy.cn'
)
$GoZipMirrors = @(
    'https://mirrors.aliyun.com/golang',
    'https://go.dev/dl'
)
$NodeZipMirrors = @(
    'https://nodejs.org/dist',
    'https://registry.npmmirror.com/-/binary/node'
)
$NpmRegistries = @(
    'https://registry.npmjs.org',
    'https://registry.npmmirror.com'
)

# The pinned fallback versions (what the zip mirrors download when winget
# cannot install). Go >= 1.26 and Node >= 20.19 are the app's requirements.
$GoZipName   = 'go1.27.2.windows-amd64.zip'
$NodeVersion = 'v22.23.3'
$NodeZipName = "node-$NodeVersion-win-x64.zip"

# ---------------------------------------------------------------------------

function Write-Step([string]$Text) { Write-Host $Text -ForegroundColor Cyan }
function Write-Ok([string]$Text)   { Write-Host $Text -ForegroundColor Green }
function Write-Warn2([string]$Text){ Write-Host $Text -ForegroundColor Yellow }
function Write-Stop([string]$Text) { Write-Host $Text -ForegroundColor Red }

function Test-Tool([string]$Name) {
    return [bool](Get-Command $Name -ErrorAction SilentlyContinue)
}

function Test-Url([string]$Url) {
    # Quick "is this mirror alive" check (6 s). curl.exe ships with
    # Windows 10 1803+; the Invoke-WebRequest fallback covers the rest.
    if (Test-Tool 'curl.exe') {
        $code = & curl.exe -s -o NUL -m 6 -L -w '%{http_code}' $Url
        if ($LASTEXITCODE -eq 0 -and "$code" -match '^[23]') { return $true }
        return $false
    }
    try {
        $r = Invoke-WebRequest -Uri $Url -Method Head -TimeoutSec 6 -UseBasicParsing
        return ($r.StatusCode -lt 400)
    } catch {
        return $false
    }
}

function Download-File([string]$Url, [string]$Out, [string]$AuthHeader) {
    if (Test-Tool 'curl.exe') {
        if ($AuthHeader) {
            & curl.exe -sS -L --fail --retry 3 --connect-timeout 20 -H $AuthHeader -o $Out $Url
        } else {
            & curl.exe -sS -L --fail --retry 3 --connect-timeout 20 -o $Out $Url
        }
        if ($LASTEXITCODE -eq 0 -and (Test-Path $Out)) { return $true }
    }
    try {
        if ($AuthHeader) {
            Invoke-WebRequest -Uri $Url -OutFile $Out -UseBasicParsing -TimeoutSec 3600 -Headers @{ Authorization = $AuthHeader }
        } else {
            Invoke-WebRequest -Uri $Url -OutFile $Out -UseBasicParsing -TimeoutSec 3600
        }
        return $true
    } catch {
        return $false
    }
}

function Update-SessionPath {
    $m = [Environment]::GetEnvironmentVariable('Path', 'Machine')
    $u = [Environment]::GetEnvironmentVariable('Path', 'User')
    $env:Path = "$m;$u"
}

function Add-UserPath([string]$Dir) {
    # Persist for future windows (no setx, so no 1024-char truncation),
    # and apply to this window right now.
    try {
        $u = [Environment]::GetEnvironmentVariable('Path', 'User')
        if ($u -notlike "*$Dir*") {
            [Environment]::SetEnvironmentVariable('Path', ($u.TrimEnd(';') + ';' + $Dir), 'User')
        }
    } catch { }
    if (";$env:Path;" -notlike "*;$Dir;*") { $env:Path += ";$Dir" }
}

$ToolsDir = if ($env:LOCALAPPDATA) { Join-Path $env:LOCALAPPDATA 'YounblockTools' } else { Join-Path (Get-Location).Path 'YounblockTools' }

# ---------------------------------------------------------------------------
# Steps
# ---------------------------------------------------------------------------

function Step1-Winget {
    Write-Step '[1/8] checking winget (the Windows package manager)...'
    if (Test-Tool 'winget') { Write-Ok '  winget ok'; return $true }
    Write-Stop @'
SETUP STOPPED: "winget" was not found on this PC.
winget comes with Windows 10 1709+ and Windows 11 as "App Installer".
Fix: open the Microsoft Store, search "App Installer", press Update,
then run this file again.
'@
    return $false
}

function Install-WithWinget([string]$PackageId) {
    & winget install --id $PackageId -e --silent --accept-package-agreements --accept-source-agreements
    Update-SessionPath
}

function Get-GoState {
    if (-not (Test-Tool 'go')) { return 'missing' }
    $v = & go version
    if ("$v" -match 'go version go(\d+)\.(\d+)') {
        $maj = [int]$Matches[1]; $min = [int]$Matches[2]
        if ($maj -gt 1 -or ($maj -eq 1 -and $min -ge 26)) { return 'ok' }
        return 'old'
    }
    return 'old'
}

function Step3-Go {
    Write-Step '[3/8] checking Go (the engine language)...'
    $state = Get-GoState
    if ($state -eq 'ok') { Write-Ok '  Go ok'; return $true }
    if ($state -eq 'old') {
        Write-Stop @'
SETUP STOPPED: your Go is older than 1.26, which this app needs.
Fix: open Settings, Apps, uninstall the old Go, then run this file
again - the current one will be installed for you.
'@
        return $false
    }
    Write-Host '  Go is missing - installing (winget first)...'
    Install-WithWinget 'GoLang.Go'
    $state = Get-GoState
    if ($state -eq 'ok') { Write-Ok '  Go ok (winget)'; return $true }

    Write-Host '  winget could not deliver Go - trying the zip mirrors...'
    $found = $null
    foreach ($m in $GoZipMirrors) {
        $u = "$m/$GoZipName"
        Write-Host "    mirror: $m"
        if (Test-Url $u) { $found = $u; break }
    }
    if (-not $found) { return $false }
    $zip = Join-Path $env:TEMP $GoZipName
    if (-not (Download-File $found $zip)) { return $false }
    try { New-Item -ItemType Directory -Force -Path $ToolsDir -ErrorAction Stop | Out-Null } catch { }
    if (Test-Path "$ToolsDir\go") { Remove-Item -Recurse -Force "$ToolsDir\go" }
    & tar.exe -xf $zip -C "$ToolsDir"
    if ($LASTEXITCODE -ne 0) { return $false }
    Add-UserPath "$ToolsDir\go\bin"
    if ((Get-GoState) -eq 'ok') { Write-Ok "  Go ok (installed from $(Split-Path -Leaf $found) mirror)"; return $true }
    return $false
}

function Get-NodeState {
    if (-not (Test-Tool 'node')) { return 'missing' }
    $v = & node --version
    if ("$v" -match 'v(\d+)\.(\d+)') {
        $maj = [int]$Matches[1]
        if ($maj -ge 20) { return 'ok' }
        return 'old'
    }
    return 'old'
}

function Step4-Node {
    Write-Step '[4/8] checking Node.js (builds the app window)...'
    $state = Get-NodeState
    if ($state -eq 'ok') { Write-Ok '  Node.js ok'; return $true }
    if ($state -eq 'old') {
        Write-Stop @'
SETUP STOPPED: your Node.js is older than version 20, which this app
needs. Fix: open Settings, Apps, uninstall the old Node.js, then run
this file again - the current one will be installed for you.
'@
        return $false
    }
    Write-Host '  Node.js is missing - installing (winget first)...'
    Install-WithWinget 'OpenJS.NodeJS.LTS'
    $state = Get-NodeState
    if ($state -eq 'ok') { Write-Ok '  Node.js ok (winget)'; return $true }

    Write-Host '  winget could not deliver Node.js - trying the zip mirrors...'
    $found = $null
    foreach ($m in $NodeZipMirrors) {
        $u = "$m/$NodeVersion/$NodeZipName"
        Write-Host "    mirror: $m"
        if (Test-Url $u) { $found = $u; break }
    }
    if (-not $found) { return $false }
    $zip = Join-Path $env:TEMP $NodeZipName
    if (-not (Download-File $found $zip)) { return $false }
    try { New-Item -ItemType Directory -Force -Path $ToolsDir -ErrorAction Stop | Out-Null } catch { }
    if (Test-Path "$ToolsDir\nodejs") { Remove-Item -Recurse -Force "$ToolsDir\nodejs" }
    $ex = Join-Path $env:TEMP 'younblock-node-extract'
    if (Test-Path $ex) { Remove-Item -Recurse -Force $ex }
    try { New-Item -ItemType Directory -Force -Path $ex -ErrorAction Stop | Out-Null } catch { }
    & tar.exe -xf $zip -C "$ex"
    if ($LASTEXITCODE -ne 0) { return $false }
    $inner = Get-ChildItem -Directory $ex | Select-Object -First 1
    if (-not $inner) { return $false }
    Move-Item $inner.FullName "$ToolsDir\nodejs"
    Add-UserPath "$ToolsDir\nodejs"
    if ((Get-NodeState) -eq 'ok') { Write-Ok "  Node.js ok (installed from $(Split-Path -Leaf $found) mirror)"; return $true }
    return $false
}

function Step5-WebView2 {
    Write-Step '[5/8] checking the WebView2 part of Windows...'
    $guid = '{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}'
    $present = $false
    foreach ($root in @("HKLM:\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients", "HKCU:\SOFTWARE\Microsoft\EdgeUpdate\Clients")) {
        if (Test-Path "$root\$guid") {
            $pv = (Get-ItemProperty "$root\$guid" -ErrorAction SilentlyContinue).pv
            if ($pv -and $pv -ne '0.0.0.0') { $present = $true; break }
        }
    }
    if ($present) { Write-Ok '  WebView2 ok'; return }
    Write-Host '  not found - installing...'
    Install-WithWinget 'Microsoft.EdgeWebView2Runtime'
    foreach ($root in @("HKLM:\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients", "HKCU:\SOFTWARE\Microsoft\EdgeUpdate\Clients")) {
        if (Test-Path "$root\$guid") { Write-Ok '  WebView2 ok (installed)'; return }
    }
    Write-Warn2 @'
  WARNING: WebView2 was not found and could not be installed.
  Setup continues - but if the app window turns out blank when it
  opens, install "Microsoft Edge WebView2 Runtime" from Microsoft,
  then run the app again.
'@
}

function Set-GoProxyChain {
    # Order the Go module mirrors by who answers, and remember the chain
    # for every later go command (go env -w persists it for build.bat).
    $alive = @()
    foreach ($m in $GoProxyMirrors) {
        if (Test-Url $m) { $alive += $m }
    }
    if ($alive.Count -eq 0) { $alive = @($GoProxyMirrors) }
    $chain = ($alive -join '|') + '|direct'
    $env:GOPROXY = $chain
    if ((Get-GoState) -eq 'ok') {
        & go env -w "GOPROXY=$chain"
    }
    return $chain
}

function Step6-Wails {
    Write-Step '[6/8] checking the Wails CLI (packs window + engine)...'
    $gobin = Join-Path $env:USERPROFILE 'go\bin'
    if (";$env:Path;" -notlike "*;$gobin;*") { $env:Path += ";$gobin" }
    if (Test-Tool 'wails') { Write-Ok '  Wails CLI ok'; return $true }

    $chain = Set-GoProxyChain
    Write-Host "  using the Go module mirrors: $chain"
    Write-Host '  installing the Wails CLI (it compiles with Go - please wait)...'
    & go install github.com/wailsapp/wails/v2/cmd/wails@latest
    if ($LASTEXITCODE -eq 0 -and (Test-Tool 'wails')) { Write-Ok '  Wails CLI ok'; return $true }
    return $false
}

function Get-GitAuthArgs {
    if (-not $GH_TOKEN) { return $null }
    $b64 = [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes("x-access-token:$GH_TOKEN"))
    return "http.https://github.com/.extraheader=AUTHORIZATION: Basic $b64"
}

function Update-Repo([string]$Repo) {
    if (-not (Test-Path "$Repo\.git")) {
        Write-Host '  standalone copy (downloaded as a zip) - skipping the update check.'
        return
    }
    Write-Host '  updating the code (git pull)...'
    $auth = Get-GitAuthArgs
    if ($auth) {
        & git -c $auth -C $Repo pull --ff-only
    } else {
        & git -C $Repo pull --ff-only
    }
    if ($LASTEXITCODE -eq 0) {
        Write-Ok '  code is up to date.'
    } else {
        Write-Warn2 '  could not auto-update - continuing with the code you have.'
    }
}

function Get-Code([string]$ScriptDir) {
    # 1. This folder is already the repository?
    if (Test-Path "$ScriptDir\wails.json") {
        Update-Repo $ScriptDir
        return $ScriptDir
    }
    # 2. A copy sits next to this script?
    if (Test-Path "$ScriptDir\Younblock\wails.json") {
        Update-Repo "$ScriptDir\Younblock"
        return "$ScriptDir\Younblock"
    }
    # 3. Clone it (with the token if provided, else GitHub's browser login).
    $dest = "$ScriptDir\Younblock"
    if (Test-Tool 'git') {
        Write-Host '  downloading the code from GitHub (git clone)...'
        if (-not $GH_TOKEN) {
            Write-Host '  the repository is private: GitHub will ask you to log in -'
            Write-Host '  a browser window opens; approve it and the download continues.'
        }
        $auth = Get-GitAuthArgs
        if ($auth) {
            & git -c $auth clone https://github.com/assadigit/Younblock.git $dest
        } else {
            & git clone https://github.com/assadigit/Younblock.git $dest
        }
        if ($LASTEXITCODE -eq 0 -and (Test-Path "$dest\wails.json")) {
            Write-Ok '  code downloaded.'
            return $dest
        }
        if (Test-Path $dest) { Remove-Item -Recurse -Force $dest -ErrorAction SilentlyContinue }
    } else {
        Write-Warn2 '  git is not available - trying the direct download instead...'
    }
    # 4. Last path: the source zip, straight from the GitHub API.
    $token = $GH_TOKEN
    if (-not $token) {
        Write-Host ''
        Write-Host '  The direct download needs a GitHub token (the repository is'
        Write-Host '  private). Get one at github.com > Settings > Developer settings'
        Write-Host '  > Tokens (classic), with the "repo" permission.'
        $token = Read-Host '  Paste the token here (or press Enter to stop)'
        if (-not $token) { return $null }
    }
    Write-Host "  downloading the source zip (tag $SourceTag)..."
    $url = "https://api.github.com/repos/assadigit/Younblock/zipball/$SourceTag"
    $zip = Join-Path $env:TEMP 'younblock-src.zip'
    if (-not (Download-File $url $zip "bearer $token")) { return $null }
    $ex = Join-Path $env:TEMP 'younblock-src'
    if (Test-Path $ex) { Remove-Item -Recurse -Force $ex }
    try { New-Item -ItemType Directory -Force -Path $ex -ErrorAction Stop | Out-Null } catch { }
    & tar.exe -xf $zip -C "$ex"
    $inner = Get-ChildItem -Directory $ex | Select-Object -First 1
    if (-not $inner) { return $null }
    if (Test-Path $dest) { Remove-Item -Recurse -Force $dest -ErrorAction SilentlyContinue }
    Move-Item $inner.FullName $dest
    if (Test-Path "$dest\wails.json") {
        Write-Ok '  code downloaded (as a zip).'
        return $dest
    }
    return $null
}

function Step8-FrontendDeps([string]$Repo) {
    Write-Step '[8/8] installing the window building blocks (npm)...'
    $reg = $null
    foreach ($r in $NpmRegistries) {
        if (Test-Url $r) { $reg = $r; break }
    }
    if (-not $reg) { $reg = $NpmRegistries[0] }
    Write-Host "  using the npm registry: $reg"

    Push-Location "$Repo\frontend"
    try {
        $first = -not (Test-Path 'node_modules')
        $verb = 'install'
        if ($first) { $verb = 'ci' }
        if ($first) {
            Write-Host '  first time - downloading everything - please wait...'
        } else {
            Write-Host '  checking for changes (quick)...'
        }
        & npm $verb --no-audit --no-fund --registry=$reg
        if ($LASTEXITCODE -ne 0) {
            $alt = $NpmRegistries | Where-Object { $_ -ne $reg } | Select-Object -First 1
            if ($alt) {
                Write-Warn2 "  the first registry failed - retrying with the mirror: $alt"
                & npm $verb --no-audit --no-fund --registry=$alt
            }
        }
        if ($LASTEXITCODE -ne 0) { return $false }
        Write-Ok '  building blocks ok'
        return $true
    } finally {
        Pop-Location
    }
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

Write-Host ''
Write-Host '=== Younblock one-click setup and run ===' -ForegroundColor White
Write-Host ''
if ($Mode -eq 'dev') {
    Write-Host '  Mode: run the app live (wails dev). Keep this window OPEN'
    Write-Host '  while the app runs; closing it stops the app.'
} else {
    Write-Host '  Mode: build every exe (build.bat). The finished files'
    Write-Host '  land in the dist folder, which opens at the end.'
}
Write-Host '  When Windows asks for permission (UAC), click Yes.'
Write-Host '  First run downloads a lot - expect 5 to 15 minutes.'
Write-Host ''

if (-not (Step1-Winget)) { exit 1 }

# [2/8] Git - helpful but optional (the code can also arrive as a zip).
Write-Step '[2/8] checking Git...'
if (Test-Tool 'git') {
    Write-Ok '  Git ok'
} else {
    Write-Host '  Git is missing - installing (winget)...'
    Install-WithWinget 'Git.Git'
    if (Test-Tool 'git') {
        Write-Ok '  Git ok (winget)'
    } else {
        Write-Warn2 '  Git did not install - the code will be downloaded as a zip'
        Write-Warn2 '  instead (a GitHub token will be asked for).'
    }
}

if (-not (Step3-Go)) {
    Write-Stop 'SETUP STOPPED: Go did not install. Read the messages above.'
    exit 1
}

if (-not (Step4-Node)) {
    Write-Stop 'SETUP STOPPED: Node.js did not install. Read the messages above.'
    exit 1
}

Step5-WebView2

if (-not (Step6-Wails)) {
    Write-Stop @'
SETUP STOPPED: the Wails CLI did not install (every Go module mirror
failed). Check your internet connection and run this file again - or
use the offline pack from the Releases page, which carries the Wails
CLI inside it and needs no internet at all.
'@
    exit 1
}

Write-Step '[7/8] getting the code...'
$ScriptDir = $PSScriptRoot
if (-not $ScriptDir) { $ScriptDir = (Get-Location).Path }
$Repo = Get-Code $ScriptDir
if (-not $Repo) {
    Write-Stop @'
SETUP STOPPED: could not get the code. The repository is private:
either log in when GitHub asks (the browser window), or paste a
GitHub token when prompted, or use the offline pack from the
Releases page.
'@
    exit 1
}
Write-Ok "  code folder: $Repo"

if (-not (Step8-FrontendDeps $Repo)) {
    Write-Stop 'SETUP STOPPED: the window building blocks did not install.'
    exit 1
}

Write-Host ''
if ($Mode -eq 'dev') {
    Write-Host '=== Starting the app: wails dev ===' -ForegroundColor White
    Write-Host ''
    Write-Host '  The first build takes a few minutes - let it work.'
    Write-Host '  The app window (360 x 760) opens by itself.'
    Write-Host '  To stop the app: close this window.'
    Write-Host ''
    Push-Location $Repo
    & wails dev
    Pop-Location
    Write-Host ''
    Write-Host 'The app has stopped.'
    Write-Host ''
    Write-Host 'Tip: to build the everyday exe files instead of dev mode,'
    Write-Host 'run setup-build.bat (same folder as this script).'
} else {
    Write-Host '=== Building every exe file ===' -ForegroundColor White
    Write-Host ''
    Write-Host '  The first build takes a few minutes - let it work.'
    Write-Host ''
    Push-Location $Repo
    & cmd.exe /c build.bat
    $built = ($LASTEXITCODE -eq 0) -and (Test-Path "$Repo\dist\younblock.exe")
    Pop-Location
    if (-not $built) {
        Write-Stop 'BUILD FAILED. Read the messages above - they say what went wrong.'
        Write-Host 'If a missing tool was the problem, fix it and run this file again.'
        exit 1
    }
    Write-Host ''
    Write-Ok 'Build finished. The files are in:'
    Write-Ok "  $Repo\dist"
    Write-Host '  Opening the folder for you now...'
    Start-Process explorer.exe "$Repo\dist"
}
exit 0
