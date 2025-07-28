@echo off
REM Timetag Correlation Analyzer - Windows Starter Script
REM ====================================================

echo.
echo Timetag Correlation Analyzer
echo ============================
echo.

REM Check if computeCorrelation.exe exists in bin directory
if not exist "bin\computeCorrelation.exe" (
    echo ERROR: bin\computeCorrelation.exe not found!
    echo Make sure you are in the correct directory and that binaries are built.
    echo Run build.bat to create the binaries if needed.
    echo.
    pause
    exit /b 1
)

REM Check if data directory exists
if not exist "data\" (
    echo ERROR: data\ directory not found!
    echo Create a 'data' folder and place your .ttbin files in it.
    echo.
    pause
    exit /b 1
)

REM Check if .ttbin files exist
dir /b "data\*.ttbin" >nul 2>&1
if errorlevel 1 (
    echo WARNING: No .ttbin files found in data\ directory!
    echo Please place your .ttbin files in the data folder first.
    echo.
    pause
    exit /b 1
)

echo Starting Timetag Correlation Analyzer...
echo.

REM Run the program
bin\computeCorrelation.exe

REM Keep window open if there was an error
if errorlevel 1 (
    echo.
    echo Program exited with an error.
    pause
)

echo.
echo Program completed successfully.
echo Results have been saved to time_diff_chX_chY.txt files.
echo.
pauseecho Starte Timetag Correlation Analyzer...
echo.

REM Run the program
computeCorrelation.exe

echo.
echo Programm beendet.
echo Die Ergebnisse wurden in eine .txt Datei gespeichert.
echo.
pause
