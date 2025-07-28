@echo off
REM Build script for Timetag Correlation Analyzer (Windows)
REM Builds binaries for Windows, macOS (Intel & Apple Silicon), and Linux
REM All binaries are placed in the 'bin\' directory

echo === Building Timetag Correlation Analyzer ===
echo Building binaries for all platforms...
echo.

REM Create bin directory if it doesn't exist
echo Creating bin directory...
if not exist bin mkdir bin
echo.

REM Clean previous builds
echo Cleaning previous builds...
if exist bin\computeCorrelation.exe del bin\computeCorrelation.exe
if exist bin\computeCorrelation-* del bin\computeCorrelation-*
if exist bin\computeCorrelation del bin\computeCorrelation
echo.

REM Build for Windows (64-bit)
echo Building for Windows (amd64)...
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-s -w" -o bin\computeCorrelation.exe main.go
if errorlevel 1 (
    echo ERROR: Failed to build Windows binary
    pause
    exit /b 1
)
echo ✓ Windows binary: bin\computeCorrelation.exe

REM Build for macOS Intel (64-bit)
echo Building for macOS Intel (amd64)...
set GOOS=darwin
set GOARCH=amd64
go build -ldflags="-s -w" -o bin\computeCorrelation-darwin-amd64 main.go
if errorlevel 1 (
    echo ERROR: Failed to build macOS Intel binary
    pause
    exit /b 1
)
echo ✓ macOS Intel binary: bin\computeCorrelation-darwin-amd64

REM Build for macOS Apple Silicon (ARM64)
echo Building for macOS Apple Silicon (arm64)...
set GOOS=darwin
set GOARCH=arm64
go build -ldflags="-s -w" -o bin\computeCorrelation-darwin-arm64 main.go
if errorlevel 1 (
    echo ERROR: Failed to build macOS Apple Silicon binary
    pause
    exit /b 1
)
echo ✓ macOS Apple Silicon binary: bin\computeCorrelation-darwin-arm64

REM Build for Linux (64-bit)
echo Building for Linux (amd64)...
set GOOS=linux
set GOARCH=amd64
go build -ldflags="-s -w" -o bin\computeCorrelation-linux-amd64 main.go
if errorlevel 1 (
    echo ERROR: Failed to build Linux binary
    pause
    exit /b 1
)
echo ✓ Linux binary: bin\computeCorrelation-linux-amd64

REM Reset environment variables
set GOOS=
set GOARCH=

echo.
echo === Build Summary ===
echo All binaries built successfully in bin\ directory:
dir bin\computeCorrelation*
echo.
echo Build flags used: -ldflags='-s -w' (strips debug info and reduces binary size)
echo ✓ All builds completed successfully!
echo.
pause
