@echo off
REM UTP-Core Build Script for Windows
REM This script builds the UTP-Core binary for Windows platforms

setlocal enabledelayedexpansion

REM Configuration
set BINARY_NAME=utp-core
set BUILD_DIR=build
set CMD_DIR=cmd/utp-core

echo [INFO] Building UTP-Core for Windows...

REM Check if Go is installed
where go >nul 2>nul
if %errorlevel% neq 0 (
    echo [ERROR] Go is not installed. Please install Go 1.21 or higher.
    exit /b 1
)

REM Get Go version
for /f "tokens=3" %%i in ('go version') do set GO_VERSION=%%i
echo [INFO] Go version: %GO_VERSION%

REM Get version information
set VERSION=dev
for /f %%i in ('git rev-parse --short HEAD 2^>nul') do set COMMIT=%%i
if "%COMMIT%"=="" set COMMIT=unknown

echo [INFO] Version: %VERSION%
echo [INFO] Commit: %COMMIT%

REM Create build directory
if not exist "%BUILD_DIR%" mkdir "%BUILD_DIR%"

REM Build flags
set LDFLAGS=-s -w -X main.version=%VERSION% -X main.commit=%COMMIT%

REM Build for Windows AMD64
echo [INFO] Building for Windows AMD64...
set GOOS=windows
set GOARCH=amd64
go build -v -ldflags "%LDFLAGS%" -o "%BUILD_DIR%\%BINARY_NAME%-windows-amd64.exe" "./%CMD_DIR%"

if %errorlevel% equ 0 (
    echo [INFO] Build successful!
    echo [INFO] Binary location: %BUILD_DIR%\%BINARY_NAME%-windows-amd64.exe
    dir "%BUILD_DIR%\%BINARY_NAME%-windows-amd64.exe"
) else (
    echo [ERROR] Build failed!
    exit /b 1
)

echo [INFO] Build complete!
endlocal
