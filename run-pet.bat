@echo off
cd /d "%~dp0"

echo ===================================================
echo           DESKTOP PET LITE - ONE-CLICK RUN
echo ===================================================
echo.

:: 1. Build pet-lite.exe if missing or out-of-date
if not exist "go-lite\pet-lite.exe" (
    echo [*] Binary pet-lite.exe not found. Building with Go...
    pushd go-lite
    go build -ldflags "-s -w -H=windowsgui" -o pet-lite.exe .
    if errorlevel 1 (
        echo [!] ERROR: Failed to build pet-lite.exe. Please ensure Go is installed.
        popd
        pause
        exit /b 1
    )
    popd
    echo [*] Built pet-lite.exe successfully!
)

:: 2. Launch pet with Dictation + Assets enabled
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
