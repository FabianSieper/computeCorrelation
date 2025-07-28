@echo off
REM Timetag Correlation Analyzer - Windows Starter Script
REM ====================================================

echo.
echo Timetag Correlation Analyzer
echo ============================
echo.

REM Check if computeCorrelation.exe exists
if not exist "computeCorrelation.exe" (
    echo FEHLER: computeCorrelation.exe nicht gefunden!
    echo Stellen Sie sicher, dass Sie sich im richtigen Verzeichnis befinden.
    echo.
    pause
    exit /b 1
)

REM Check if data directory exists
if not exist "data\" (
    echo FEHLER: data\ Verzeichnis nicht gefunden!
    echo Erstellen Sie einen 'data' Ordner und legen Sie Ihre .ttbin Dateien hinein.
    echo.
    pause
    exit /b 1
)

REM Check if .ttbin files exist
dir /b "data\*.ttbin" >nul 2>&1
if errorlevel 1 (
    echo WARNUNG: Keine .ttbin Dateien im data\ Verzeichnis gefunden!
    echo Legen Sie Ihre .ttbin Dateien in den data\ Ordner.
    echo.
    pause
)

echo Starte Timetag Correlation Analyzer...
echo.

REM Run the program
computeCorrelation.exe

echo.
echo Programm beendet.
echo Die Ergebnisse wurden in eine .txt Datei gespeichert.
echo.
pause
