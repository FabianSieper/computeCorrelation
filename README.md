# Timetag Correlation Analyzer

This Go program calculates time differences between two channels from timetag files (.ttbin format).

## Features

The program:

1. Reads all .ttbin files from the `data/` directory (sorted lexicographically)
2. Extracts timetag data for two user-specified channels
3. Calculates time differences between consecutive events of the specified channels
4. Saves results to a CSV file

Key features:

- **Multiple files**: Automatically processes all .ttbin files in chronological order
- **Streaming**: Efficient processing of large files through block-wise reading
- **Correlation analysis**: Calculates time differences between all consecutive events of selected channels
- **Channel overview**: Shows a summary of all found channels and event counts
- **Cross-platform**: Works on Windows, macOS, and Linux

## Installation & Usage

### Windows (Recommended - No Go installation required)

#### Option A: Using setup script (first-time users)

If you're setting up the program for the first time:

1. Run `setup_windows.bat` to automatically:
   - Check if all required files are present
   - Create the `data/` directory if needed
   - Verify your .ttbin files are in place
   - Guide you through the setup process

2. After setup, use `start_windows.bat` to run the program

#### Option B: Using pre-compiled executable (experienced users)

1. Download or copy the `computeCorrelation.exe` file
2. Create a folder for your data and place your .ttbin files in a subfolder called `data/`
3. Double-click `start_windows.bat` or run:

   ```cmd
   computeCorrelation.exe
   ```

#### Option C: Using batch script

1. Use the provided `start_windows.bat` script which includes error checking:

   ```cmd
   start_windows.bat
   ```

### macOS / Linux

#### Option A: Using pre-compiled binary (if available)

```bash
chmod +x computeCorrelation
./computeCorrelation
```

#### Option B: Compile from source

1. Install Go from <https://golang.org/dl/>
2. Compile:

   ```bash
   go build -o computeCorrelation main.go
   ```

3. Run:

   ```bash
   ./computeCorrelation
   ```

### Building for Different Platforms

If you need to create executables for different operating systems:

```bash
# For Windows (from any OS)
GOOS=windows GOARCH=amd64 go build -o computeCorrelation.exe main.go

# For macOS (from any OS)
GOOS=darwin GOARCH=amd64 go build -o computeCorrelation main.go

# For Linux (from any OS)
GOOS=linux GOARCH=amd64 go build -o computeCorrelation main.go
```

## Usage Instructions

1. Place your .ttbin files in a `data/` subdirectory
2. Run the program using one of the methods above
3. Enter the two channel numbers when prompted:

   ```text
   Enter first channel number: 258
   Enter second channel number: 259
   ```

## File Structure

Your project directory should look like this:

```text
your-project/
├── computeCorrelation.exe      (Windows executable)
├── computeCorrelation          (macOS/Linux executable)
├── setup_windows.bat           (Windows setup script - first-time users)
├── start_windows.bat           (Windows launcher script)
├── main.go                     (source code)
└── data/                       (your .ttbin files)
    ├── file1.ttbin
    ├── file2.ttbin
    └── ...
```

### Windows Script Description

- **`setup_windows.bat`**: Run this first if you're new to the program. It will:
  - Check if all required files are present
  - Create the `data/` directory automatically
  - Verify your .ttbin files are in the right place
  - Provide helpful guidance for first-time setup

- **`start_windows.bat`**: Use this to run the program with error checking and user-friendly messages

## Output Format

The program creates a file named `time_diff_ch<X>_ch<Y>.txt` with the following structure:

```text
# Time differences between consecutive events
# Format: Timestamp1, Timestamp2, Channel1, Channel2, TimeDiff(ns)
# TimeDiff = Timestamp2 - Timestamp1

<timestamp1>,<timestamp2>,<channel1>,<channel2>,<time_diff>
...
```

Where:

- `timestamp1`: Timestamp of the first event
- `timestamp2`: Timestamp of the second event  
- `channel1`: Channel number of the first event
- `channel2`: Channel number of the second event
- `time_diff`: Time difference in nanoseconds (timestamp2 - timestamp1)

## File Format

The program expects .ttbin files in SITT (SwissInstruments Time Tag) format in the `data/` directory.

## Example Output

```text
Enter first channel number: 262
Enter second channel number: 261
Processing file: data/TimeTags-test_2025-07-08_161026-1.ttbin
  Found: 1 blocks of type 0x3
Processing file: data/TimeTags-test_2025-07-08_161026.ttbin
  Found: 1 blocks of type 0x3
Total time tags read: 4
Found channels:
  Channel 262: 2 Events
  Channel 261: 2 Events
Relevant events (sorted): 4
Successfully calculated 3 time differences and saved to 'time_diff_ch262_ch261.txt'.
```
