@echo off
setlocal enabledelayedexpansion
cd /d "%~dp0"

echo ===================================================
echo           DESKTOP PET LITE - ONE-CLICK RUN
echo ===================================================
echo.

:: 1. Check or Auto-Install uv if needed
where uv >nul 2>nul
if %errorlevel% neq 0 (
    echo [*] 'uv' is not found in PATH. Checking standalone installer...
    powershell -NoProfile -ExecutionPolicy Bypass -Command "irm https://astral.sh/uv/install.ps1 | iex" >nul 2>&1
    set "PATH=%USERPROFILE%\.cargo\bin;%USERPROFILE%\.local\bin;%PATH%"
)

:: 2. Prepare Python Virtual Environment for Voice Sidecar
if not exist "voice-sidecar\.venv\Scripts\python.exe" (
    echo [*] Preparing voice-sidecar Python environment...
    where uv >nul 2>nul
    if %errorlevel% equ 0 (
        pushd voice-sidecar
        echo [*] Installing dependencies with uv...
        uv sync --no-install-project
        popd
    ) else (
        echo [!] Warning: Could not configure uv automatically.
        echo [!] Dictation requires Python. Please install uv or Python 3.10+ if dictation fails to start.
    )
)

:: 3. Check for prebuilt pet-lite.exe or build with Go
if not exist "go-lite\pet-lite.exe" (
    echo [*] pet-lite.exe not found. Looking for Go compiler...
    where go >nul 2>nul
    if %errorlevel% equ 0 (
        echo [*] Building pet-lite.exe with Go...
        pushd go-lite
        go build -ldflags "-s -w -H=windowsgui" -o pet-lite.exe .
        popd
    ) else (
        echo [!] ERROR: pet-lite.exe is not present and Go is not installed on this machine.
        echo [!] Please install Go (https://go.dev) or build pet-lite.exe once before transferring.
        pause
        exit /b 1
    )
)

:: 4. Launch Desktop Pet with Dictation
echo [*] Launching Desktop Pet (pet5, F5 Toggle Dictation, F6 Hold-to-Talk)...
start "" "go-lite\pet-lite.exe" -assets "assets" -pet pet5 -dictation -dictation-toggle F5 -dictation-hold F6

echo.
echo Desktop Pet is running!
echo - F5: Toggle Dictation
echo - F6: Hold-to-Talk
echo - Click pet: Open chat
echo ===================================================
timeout /t 3 >nul
exit /b 0
