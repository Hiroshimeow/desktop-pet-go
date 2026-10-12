@echo off
setlocal enabledelayedexpansion
cd /d "%~dp0"

echo ===================================================
echo           DESKTOP PET LITE - ONE-CLICK RUN
echo ===================================================
echo.

:: 1. Check Voice Sidecar Python Environment
if not exist "voice-sidecar\.venv\Scripts\python.exe" (
    echo [*] Python virtual environment for voice-sidecar not found.
    where uv >nul 2>nul
    if %errorlevel% equ 0 (
        echo [*] Running uv sync for voice-sidecar...
        pushd voice-sidecar
        uv sync --no-install-project
        popd
    ) else (
        echo [!] Warning: uv is not installed. Voice sidecar may need manual setup if not already prepared.
    )
)

:: 2. Build pet-lite.exe if missing
if not exist "go-lite\pet-lite.exe" (
    echo [*] Binary pet-lite.exe not found. Building with Go...
    where go >nul 2>nul
    if %errorlevel% neq 0 (
        echo [!] ERROR: Go is not installed on this machine.
        pause
        exit /b 1
    )
    pushd go-lite
    go build -ldflags "-s -w -H=windowsgui" -o pet-lite.exe .
    if errorlevel 1 (
        echo [!] ERROR: Failed to build pet-lite.exe.
        popd
        pause
        exit /b 1
    )
    popd
    echo [*] Built pet-lite.exe successfully!
)

:: 3. Launch pet with Dictation + Assets enabled
echo [*] Launching Desktop Pet with Dictation enabled (F5 Toggle, F6 Hold)...
start "" "go-lite\pet-lite.exe" -assets "assets" -pet pet5 -dictation -dictation-toggle F5 -dictation-hold F6

echo.
echo Desktop Pet is now running!
echo - F5: Toggle Dictation
echo - F6: Hold-to-Talk
echo - Click pet: Open chat
echo ===================================================
timeout /t 3 >nul
exit /b 0
