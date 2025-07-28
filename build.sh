#!/bin/bash

# Build script for Timetag Correlation Analyzer
# Builds binaries for Windows, macOS (Intel & Apple Silicon), and Linux
# All binaries are placed in the 'bin/' directory

set -e  # Exit on any error

echo "=== Building Timetag Correlation Analyzer ==="
echo "Building binaries for all platforms..."
echo

# Create bin directory if it doesn't exist
echo "Creating bin directory..."
mkdir -p bin
echo

# Clean previous builds
echo "Cleaning previous builds..."
rm -f bin/computeCorrelation bin/computeCorrelation.exe bin/computeCorrelation-*
echo

# Build for Windows (64-bit)
echo "Building for Windows (amd64)..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o bin/computeCorrelation.exe main.go
echo "✓ Windows binary: bin/computeCorrelation.exe"

# Build for macOS Intel (64-bit)
echo "Building for macOS Intel (amd64)..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o bin/computeCorrelation-darwin-amd64 main.go
echo "✓ macOS Intel binary: bin/computeCorrelation-darwin-amd64"

# Build for macOS Apple Silicon (ARM64)
echo "Building for macOS Apple Silicon (arm64)..."
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o bin/computeCorrelation-darwin-arm64 main.go
echo "✓ macOS Apple Silicon binary: bin/computeCorrelation-darwin-arm64"

# Build for Linux (64-bit)
echo "Building for Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/computeCorrelation-linux-amd64 main.go
echo "✓ Linux binary: bin/computeCorrelation-linux-amd64"

# Build for current platform (for local testing)
echo "Building for current platform..."
go build -ldflags="-s -w" -o bin/computeCorrelation main.go
echo "✓ Local binary: bin/computeCorrelation"

echo
echo "=== Build Summary ==="
echo "All binaries built successfully in bin/ directory:"
ls -la bin/computeCorrelation*
echo
echo "Build flags used: -ldflags='-s -w' (strips debug info and reduces binary size)"
echo "✓ All builds completed successfully!"
