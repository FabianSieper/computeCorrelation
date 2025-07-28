@echo off
echo ============================================
echo Timetag Correlation Analyzer - Setup
echo ============================================
echo.

REM Check if computeCorrelation.exe exists in bin directory
if not exist "bin\computeCorrelation.exe" (
    echo ERROR: bin\computeCorrelation.exe not found!
    echo Please make sure the executable file is in the bin directory.
    echo Run build.bat to create the binaries if needed.
    echo.
    pause
    exit /b 1
)

REM Create data directory if it doesn't exist
if not exist "data" (
    echo Creating data directory...
    mkdir data
    echo Data directory created successfully.
    echo.
) else (
    echo Data directory already exists.
    echo.
)

REM Check if data directory has .ttbin files
set "found_ttbin=0"
for %%f in (data\*.ttbin) do (
    set "found_ttbin=1"
    goto :found
)

:found
if %found_ttbin%==0 (
    echo WARNING: No .ttbin files found in data directory!
    echo Please copy your .ttbin files to the 'data' folder before running the program.
    echo.
    echo Example file structure:
    echo   your-project\
    echo   ├── bin\
    echo   │   └── computeCorrelation.exe
    echo   ├── start_windows.bat
    echo   └── data\
    echo       ├── file1.ttbin
    echo       ├── file2.ttbin
    echo       └── ...
    echo.
    echo Press any key to continue anyway...
    pause >nul
) else (
    echo Found .ttbin files in data directory:
    for %%f in (data\*.ttbin) do echo   - %%f
    echo.
)

echo Setup complete! You can now run the program by:
echo 1. Double-clicking 'start_windows.bat'
echo 2. Or running 'bin\computeCorrelation.exe' directly
echo.
echo The program will ask you for two channel numbers to analyze.
echo Results will be saved as 'time_diff_chX_chY.txt' files.
echo.
pause
