# Timetag Correlation Analyzer - PowerShell Starter Script
# ========================================================

Write-Host ""
Write-Host "Timetag Correlation Analyzer" -ForegroundColor Green
Write-Host "============================" -ForegroundColor Green
Write-Host ""

# Check if computeCorrelation.exe exists
if (-not (Test-Path "computeCorrelation.exe")) {
    Write-Host "FEHLER: computeCorrelation.exe nicht gefunden!" -ForegroundColor Red
    Write-Host "Stellen Sie sicher, dass Sie sich im richtigen Verzeichnis befinden." -ForegroundColor Yellow
    Write-Host ""
    Read-Host "Drücken Sie Enter zum Beenden"
    exit 1
}

# Check if data directory exists
if (-not (Test-Path "data\")) {
    Write-Host "FEHLER: data\ Verzeichnis nicht gefunden!" -ForegroundColor Red
    Write-Host "Erstellen Sie einen 'data' Ordner und legen Sie Ihre .ttbin Dateien hinein." -ForegroundColor Yellow
    Write-Host ""
    Read-Host "Drücken Sie Enter zum Beenden"
    exit 1
}

# Check if .ttbin files exist
$ttbinFiles = Get-ChildItem -Path "data\" -Filter "*.ttbin" -ErrorAction SilentlyContinue
if ($ttbinFiles.Count -eq 0) {
    Write-Host "WARNUNG: Keine .ttbin Dateien im data\ Verzeichnis gefunden!" -ForegroundColor Yellow
    Write-Host "Legen Sie Ihre .ttbin Dateien in den data\ Ordner." -ForegroundColor Yellow
    Write-Host ""
    Read-Host "Drücken Sie Enter um trotzdem fortzufahren"
}

Write-Host "Starte Timetag Correlation Analyzer..." -ForegroundColor Cyan
Write-Host ""

# Run the program
& ".\computeCorrelation.exe"

Write-Host ""
Write-Host "Programm beendet." -ForegroundColor Green
Write-Host "Die Ergebnisse wurden in eine .txt Datei gespeichert." -ForegroundColor Green
Write-Host ""
Read-Host "Drücken Sie Enter zum Beenden"
