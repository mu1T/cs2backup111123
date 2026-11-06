@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion

echo ========================================================
echo   CS2 Config Backup ^& Sync - Portable Production Build
echo ========================================================

cd /d "%~dp0"

:: Check Go environment
where go >nul 2>&1
if errorlevel 1 (
    if exist "C:\Program Files\Go\bin\go.exe" (
        set "PATH=%PATH%;C:\Program Files\Go\bin"
    ) else (
        echo [ERROR] Go compiler not found! Please ensure Go is installed and in PATH.
        exit /b 1
    )
)

:: Check pnpm / npm environment
set "PKG_MGR=pnpm"
where pnpm >nul 2>&1
if errorlevel 1 (
    where npm >nul 2>&1
    if errorlevel 1 (
        if exist "C:\Program Files\nodejs\npm.cmd" (
            set "PKG_MGR=C:\Program Files\nodejs\npm.cmd"
        ) else (
            echo [ERROR] Neither pnpm nor npm found! Please ensure Node.js is installed.
            exit /b 1
        )
    ) else (
        set "PKG_MGR=npm"
    )
)

:: 1. Build frontend bundle
echo [1/3] Building production frontend bundle via Vite...
cd frontend
call !PKG_MGR! run build
if errorlevel 1 (
    echo [ERROR] Frontend build failed!
    cd ..
    exit /b 1
)
cd ..

:: 2. Ensure Windows resource syso with embedded icon and manifest
echo [2/3] Checking Windows resources and embedded application icon...
if not exist "rsrc_windows_amd64.syso" (
    where go-winres >nul 2>&1
    if errorlevel 1 (
        if exist "%USERPROFILE%\go\bin\go-winres.exe" (
            set "PATH=%PATH%;%USERPROFILE%\go\bin"
        ) else (
            echo Installing go-winres to compile Windows application resources...
            go install github.com/tc-hib/go-winres@latest
            set "PATH=%PATH%;%USERPROFILE%\go\bin"
        )
    )
    go-winres simply --arch amd64 --icon img/app_icon.ico --manifest gui --product-version 2.0.0 --file-version 2.0.0 --product-name "CS2 Config Backup & Sync" --file-description "CS2 Config Backup & Sync Desktop Application" --copyright "Copyright 2026" --original-filename cs2backup.exe
)

:: 3. Compile portable Windows executable
echo [3/3] Compiling portable single-file executable cs2backup.exe...
go build -tags "desktop,production" -ldflags "-s -w -H windowsgui" -o cs2backup.exe .
if errorlevel 1 (
    echo [ERROR] Go compilation failed!
    exit /b 1
)

if not exist "cs2backup.exe" goto :notfound

for %%F in (cs2backup.exe) do set size=%%~zF
set /a sizeMB=!size! / 1048576
echo ========================================================
echo   BUILD SUCCESSFUL!
echo   Output: %~dp0cs2backup.exe
echo   Size:   !size! bytes [~!sizeMB! MB]
echo ========================================================
exit /b 0

:notfound
echo [ERROR] cs2backup.exe not found after build.
exit /b 1
