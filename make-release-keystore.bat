@echo off
setlocal EnableExtensions
rem ================================================================
rem  make-release-keystore.bat -- Younblock release signing key
rem  Decision D88 (docs/android-port/07 section 8) -- M8-e session
rem
rem  WHAT THIS DOES -- everything local, no network, nothing sent
rem  anywhere and nothing committed to git:
rem    1. finds keytool (PATH, JAVA_HOME, Android Studio's bundled
rem       Java, or a common JDK install)
rem    2. generates a 32-character password with Windows' own crypto
rem       RNG -- the password is created on THIS machine only
rem    3. creates younblock-release.keystore: RSA 4096, valid 10950
rem       days -- 30 years, every keytool question pre-filled, so
rem       there are no prompts at all
rem    4. verifies the new keystore by reading it back with the same
rem       password
rem    5. writes younblock-keystore-backup_DATE.md next to it: the
rem       password, the alias, the certificate SERIAL NUMBER, the
rem       SHA-256 fingerprint, and the full keytool record
rem
rem  HOW TO USE: put this file in a PRIVATE folder -- not inside the
rem  Younblock repo, not inside any synced or cloud folder -- and
rem  double-click it. The keystore and the .md backup are created
rem  next to this file. You may edit the four "set" lines below
rem  before running it.
rem
rem  WHY IT MATTERS: losing the keystore or its password means every
rem  future signed release can never update over an existing signed
rem  install -- users would have to uninstall first. Keep the
rem  .keystore file and the .md backup in TWO different safe places.
rem
rem  Exit codes: 0 ok; 1 keytool not found; 2 keystore already
rem  exists; 3 keytool failed; 4 password generation failed; 5 the
rem  .md backup could not be written.
rem ================================================================
title Younblock release keystore generator

rem ---- settings you may edit before running -----------------------
set "KSFILE=younblock-release.keystore"
set "ALIAS=younblock"
set "VALIDITY=10950"
set "DNAME=CN=Younblock Release, OU=Younblock, O=Younblock, C=IR"
rem ------------------------------------------------------------------

cd /d "%~dp0"

echo.
echo === Younblock release keystore generator ======================
echo Folder   : %CD%
echo Keystore : %KSFILE%
echo Alias    : %ALIAS%
echo Key      : RSA 4096, valid %VALIDITY% days
echo.

if exist ".git" (
    echo NOTE: this folder looks like a git repository. A release
    echo keystore does not belong inside the repo -- it deliberately
    echo carries no release keys. The files will still be created here,
    echo but a private folder outside the repo is the safer place.
    echo.
    pause
)

rem ---- 1) never overwrite an existing keystore --------------------
if exist "%KSFILE%" (
    echo STOPPED: "%KSFILE%" already exists in this folder.
    echo.
    echo A release keystore is created ONCE. A second one would be a NEW
    echo signing identity -- signed releases could never update over
    echo installs signed with the old one.
    echo.
    echo If you lost the password, read the .md backup next to it. Only
    echo if you lost BOTH should you delete the old keystore -- and then
    echo accept that the next signed release needs uninstall-before-update.
    echo.
    pause
    exit /b 2
)

rem ---- 2) locate keytool -------------------------------------------
set "KT="
for /f "delims=" %%F in ('where keytool 2^>nul') do if not defined KT set "KT=%%F"
if not defined KT if exist "%JAVA_HOME%\bin\keytool.exe" set "KT=%JAVA_HOME%\bin\keytool.exe"
if not defined KT if exist "%LOCALAPPDATA%\Programs\Android Studio\jbr\bin\keytool.exe" set "KT=%LOCALAPPDATA%\Programs\Android Studio\jbr\bin\keytool.exe"
if not defined KT if exist "%ProgramFiles%\Android\Android Studio\jbr\bin\keytool.exe" set "KT=%ProgramFiles%\Android\Android Studio\jbr\bin\keytool.exe"
for /d %%D in ("%ProgramFiles%\Java\*") do if not defined KT if exist "%%D\bin\keytool.exe" set "KT=%%D\bin\keytool.exe"
for /d %%D in ("%ProgramFiles%\Eclipse Adoptium\*") do if not defined KT if exist "%%D\bin\keytool.exe" set "KT=%%D\bin\keytool.exe"
for /d %%D in ("%ProgramFiles%\Microsoft\jdk*") do if not defined KT if exist "%%D\bin\keytool.exe" set "KT=%%D\bin\keytool.exe"
for /d %%D in ("%ProgramFiles%\Zulu\*") do if not defined KT if exist "%%D\bin\keytool.exe" set "KT=%%D\bin\keytool.exe"

if not defined KT (
    echo STOPPED: keytool was not found on this machine.
    echo.
    echo keytool ships with every full JDK -- the Java bundled with
    echo Android Studio has one. Install Android Studio or a JDK, then
    echo run this file again. Nothing was created yet.
    echo.
    pause
    exit /b 1
)
echo Using keytool: %KT%
echo.

rem ---- 3) generate the password locally ---------------------------
rem 32 characters drawn from an exactly-64-character alphabet, so
rem every random byte maps to one character with zero bias -- about
rem 192 bits of real entropy. The alphabet skips the characters that
rem are special to cmd.exe and the confusable 0 and 1, so the
rem password is safe to show, store and retype.
set "PW="
set "TS="
for /f "usebackq tokens=1,2 delims=|" %%A in (`powershell -NoProfile -ExecutionPolicy Bypass -Command "$e=[char[]]'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz23456789@#$-';$b=New-Object byte[] 32;$r=New-Object System.Security.Cryptography.RNGCryptoServiceProvider;$r.GetBytes($b);$s='';foreach($x in $b){$s+=[string]$e[$x -band 63]};$t=Get-Date -Format yyyy-MM-dd_HH-mm-ss;Write-Output ($s+'|'+$t)"`) do (
    set "PW=%%A"
    set "TS=%%B"
)
if not defined PW (
    echo STOPPED: Windows PowerShell did not return a password.
    echo PowerShell ships with Windows itself and this step uses no
    echo network. Run this file again; if it keeps failing, note the
    echo message above.
    echo.
    pause
    exit /b 4
)

rem ---- 4) create the keystore, every question pre-filled ----------
echo Generating the key pair -- this can take a minute on slower PCs...
echo.
"%KT%" -genkeypair -keystore "%KSFILE%" -alias "%ALIAS%" -keyalg RSA -keysize 4096 -validity %VALIDITY% -storetype PKCS12 -storepass "%PW%" -keypass "%PW%" -dname "%DNAME%"
if errorlevel 1 (
    echo.
    echo STOPPED: keytool failed to create the keystore. Its own message
    echo is above. Nothing was backed up; fix the cause and run this
    echo file again.
    echo.
    pause
    exit /b 3
)
echo.

rem ---- 5) verify by reading it back with the same password --------
set "DUMP=%TEMP%\younblock_keytool_dump.txt"
"%KT%" -list -v -keystore "%KSFILE%" -storepass "%PW%" > "%DUMP%" 2>&1
if errorlevel 1 (
    echo.
    echo STOPPED: the keystore was created but could not be read back
    echo with the generated password. Treat it as FAILED: delete
    echo "%KSFILE%" and run this file again. Do not sign anything with it.
    echo.
    pause
    exit /b 3
)

rem ---- 6) pull the certificate facts out of the keytool record ----
set "SERIAL="
set "SHA256="
set "VALIDFROM="
set "OWNER="
for /f "usebackq tokens=3" %%A in (`findstr /C:"Serial number:" "%DUMP%"`) do set "SERIAL=%%A"
for /f "usebackq tokens=2" %%A in (`findstr /C:"SHA256:" /C:"SHA-256:" "%DUMP%"`) do if not defined SHA256 set "SHA256=%%A"
for /f "usebackq tokens=2,*" %%A in (`findstr /C:"Valid from:" "%DUMP%"`) do set "VALIDFROM=%%B"
for /f "usebackq tokens=1,*" %%A in (`findstr /B /C:"Owner:" "%DUMP%"`) do set "OWNER=%%B"
if not defined SERIAL set "SERIAL=(not captured -- see the full record below)"
if not defined SHA256 set "SHA256=(not captured -- see the full record below)"
if not defined VALIDFROM set "VALIDFROM=(not captured -- see the full record below)"
if not defined OWNER set "OWNER=(not captured -- see the full record below)"

rem ---- 7) write the .md backup next to the keystore ---------------
set "MDFILE=younblock-keystore-backup_%TS%.md"
set "KSPATH=%CD%\%KSFILE%"

> "%MDFILE%" echo # Younblock release keystore -- password backup
>> "%MDFILE%" echo.
>> "%MDFILE%" echo **This file contains the keystore password in plain text.**
>> "%MDFILE%" echo Move it to safe storage now -- a password manager, an encrypted
>> "%MDFILE%" echo USB drive, or printed paper -- and delete this copy from any
>> "%MDFILE%" echo folder that syncs, backs up automatically, or gets committed to
>> "%MDFILE%" echo git. Never commit this file anywhere.
>> "%MDFILE%" echo.
>> "%MDFILE%" echo - Created:              %DATE% %TIME%
>> "%MDFILE%" echo - Machine / user:       %COMPUTERNAME% / %USERNAME%
>> "%MDFILE%" echo - Keystore file:        %KSPATH%
>> "%MDFILE%" echo - Keystore type:        PKCS12 -- RSA 4096, validity %VALIDITY% days
>> "%MDFILE%" echo - Key alias:            %ALIAS%
>> "%MDFILE%" echo - Certificate owner:    %OWNER%
>> "%MDFILE%" echo - Certificate serial:   %SERIAL%
>> "%MDFILE%" echo - Valid from:           %VALIDFROM%
>> "%MDFILE%" echo - SHA-256 fingerprint:  %SHA256%
>> "%MDFILE%" echo - Generated with:       make-release-keystore.bat -- the password
>> "%MDFILE%" echo                        was generated locally on the machine above
>> "%MDFILE%" echo                        and never sent anywhere.
>> "%MDFILE%" echo.
>> "%MDFILE%" echo ## Password
>> "%MDFILE%" echo.
>> "%MDFILE%" echo ```
>> "%MDFILE%" echo %PW%
>> "%MDFILE%" echo ```
>> "%MDFILE%" echo.
>> "%MDFILE%" echo ## If you lose this file
>> "%MDFILE%" echo.
>> "%MDFILE%" echo Losing the .keystore file or its password means every future
>> "%MDFILE%" echo signed release can never update over an existing signed
>> "%MDFILE%" echo install -- users would have to uninstall first. Keep the
>> "%MDFILE%" echo .keystore file and this backup in TWO different safe places,
>> "%MDFILE%" echo and keep both out of any synced folder and out of the
>> "%MDFILE%" echo Younblock repo -- the repo deliberately carries no release keys.
>> "%MDFILE%" echo.
>> "%MDFILE%" echo ## Full keytool record
>> "%MDFILE%" echo.
>> "%MDFILE%" echo ```
type "%DUMP%" >> "%MDFILE%"
>> "%MDFILE%" echo ```
>> "%MDFILE%" echo.

set "MDFAILED="
if not exist "%MDFILE%" set "MDFAILED=1"
del "%DUMP%" >nul 2>&1

rem ---- 8) summary ---------------------------------------------------
echo.
echo === DONE ========================================================
echo.
echo Keystore : %KSPATH%
echo Backup   : %CD%\%MDFILE%
echo Alias    : %ALIAS%
echo Serial   : %SERIAL%
echo SHA-256  : %SHA256%
echo.
echo The password -- it is also inside the .md backup:
echo.
echo     %PW%
echo.
echo NEXT STEPS
echo   1. Move the .md backup somewhere safe, then delete it from this
echo      folder if this folder syncs anywhere.
echo   2. Copy the .keystore file to a second safe place.
echo   3. Keep BOTH out of the Younblock repo and out of any cloud or
echo      synced folder.
echo   4. When the signed-release lane opens, this keystore becomes the
echo      signingConfig key store file and this password its store
echo      password -- see docs/android-port/07 section 8.
echo.
if defined MDFAILED (
    echo WARNING: the .md backup file was NOT written. The keystore is
    echo created -- the password above is the ONLY copy while this
    echo window is open. Write it down now, then create your own backup
    echo note.
    echo.
)
pause
if defined MDFAILED exit /b 5
exit /b 0
