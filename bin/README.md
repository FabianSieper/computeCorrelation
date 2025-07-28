# Binaries Directory

This directory contains pre-compiled executables for different operating systems and architectures.

## Available Binaries

- **`computeCorrelation.exe`** - Windows 64-bit executable
- **`computeCorrelation-darwin-amd64`** - macOS Intel (x86_64) executable
- **`computeCorrelation-darwin-arm64`** - macOS Apple Silicon (ARM64) executable  
- **`computeCorrelation-linux-amd64`** - Linux 64-bit executable

## Usage

### Windows

```cmd
bin\computeCorrelation.exe
```

### macOS

```bash
# For Intel Macs
chmod +x bin/computeCorrelation-darwin-amd64
./bin/computeCorrelation-darwin-amd64

# For Apple Silicon Macs
chmod +x bin/computeCorrelation-darwin-arm64
./bin/computeCorrelation-darwin-arm64

### Linux

```bash
chmod +x bin/computeCorrelation-linux-amd64
./bin/computeCorrelation-linux-amd64

## Building Binaries

To rebuild all binaries, run the build script from the project root:

### Unix/Linux/macOS

```bash
./build.sh
```

### Windows Build

```cmd
build.bat
```

## Binary Optimization

All binaries are built with the following optimization flags:

- `-ldflags="-s -w"` - Strips debug information and symbol tables to reduce file size
- Cross-compilation for multiple platforms using Go's built-in GOOS/GOARCH support

## File Sizes

The binaries are optimized for size while maintaining full functionality. Typical sizes:

- Windows: ~1.9 MB
- macOS: ~1.8 MB  
- Linux: ~1.8 MB
