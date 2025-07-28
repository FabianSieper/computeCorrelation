@echo off
REM Build script for Timetag Correlation Analyzer (Windows)
REM Builds binaries for Windows, macOS (Intel & Apple Silicon), and Linux

echo === Building Timetag Correlation Analyzer ===
echo Building binaries for all platforms...
echo.

REM Clean previous builds
echo Cleaning previous builds...
if exist computeCorrelation.exe del computeCorrelation.exe
if exist computeCorrelation-* del computeCorrelation-*
if exist computeCorrelation del computeCorrelation
echo.

REM Build for Windows (64-bit)
echo Building for Windows (amd64)...
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-s -w" -o computeCorrelation.exe main.go
if errorlevel 1 (
    echo ERROR: Failed to build Windows binary
    pause
    exit /b 1
)
echo ✓ Windows binary: computeCorrelation.exe

REM Build for macOS Intel (64-bit)
echo Building for macOS Intel (amd64)...
set GOOS=darwin
set GOARCH=amd64
go build -ldflags="-s -w" -o computeCorrelation-darwin-amd64 main.go
if errorlevel 1 (
    echo ERROR: Failed to build macOS Intel binary
    pause
    exit /b 1
)
echo ✓ macOS Intel binary: computeCorrelation-darwin-amd64

REM Build for macOS Apple Silicon (ARM64)
echo Building for macOS Apple Silicon (arm64)...
set GOOS=darwin
set GOARCH=arm64
go build -ldflags="-s -w" -o computeCorrelation-darwin-arm64 main.go
if errorlevel 1 (
    echo ERROR: Failed to build macOS Apple Silicon binary
    pause
    exit /b 1
)
echo ✓ macOS Apple Silicon binary: computeCorrelation-darwin-arm64

REM Build for Linux (64-bit)
echo Building for Linux (amd64)...
set GOOS=linux
set GOARCH=amd64
go build -ldflags="-s -w" -o computeCorrelation-linux-amd64 main.go
if errorlevel 1 (
    echo ERROR: Failed to build Linux binary
    pause
    exit /b 1
)
echo ✓ Linux binary: computeCorrelation-linux-amd64

REM Reset environment variables
set GOOS=
set GOARCH=

echo.
echo === Build Summary ===
echo All binaries built successfully:
dir computeCorrelation*
echo.
echo Build flags used: -ldflags='-s -w' (strips debug info and reduces binary size)
echo ✓ All builds completed successfully!
echo.
pause
