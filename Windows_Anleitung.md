# Windows Anleitung - Timetag Correlation Analyzer

## Systemanforderungen
- Windows 10/11 (64-bit)
- Keine zusätzliche Software erforderlich

## Installation

1. Laden Sie die folgenden Dateien herunter:
   - `computeCorrelation.exe` - Das Hauptprogramm
   - `data/` Ordner mit Ihren .ttbin Dateien

2. Erstellen Sie einen Ordner auf Ihrem Computer (z.B. `C:\TimetagAnalyzer\`)

3. Kopieren Sie `computeCorrelation.exe` in diesen Ordner

4. Erstellen Sie einen Unterordner namens `data\` im gleichen Verzeichnis

5. Kopieren Sie Ihre .ttbin Dateien in den `data\` Ordner

## Nutzung

### Über Kommandozeile (empfohlen):

1. Öffnen Sie die Eingabeaufforderung (cmd) oder PowerShell:
   - Drücken Sie `Windows + R`
   - Geben Sie `cmd` ein und drücken Sie Enter

2. Navigieren Sie zum Programmordner:
   ```cmd
   cd C:\TimetagAnalyzer
   ```

3. Führen Sie das Programm aus:
   ```cmd
   computeCorrelation.exe
   ```

4. Folgen Sie den Anweisungen:
   ```
   Geben Sie die erste Kanalnummer ein: 262
   Geben Sie die zweite Kanalnummer ein: 261
   ```

### Über Doppelklick:

Sie können auch direkt auf `computeCorrelation.exe` doppelklicken. Das Programm öffnet sich in einem Kommandozeilenfenster.

## Ausgabe

Das Programm erstellt eine Datei namens `time_diff_ch<X>_ch<Y>.txt` mit den berechneten Zeitdifferenzen.

## Beispiel Verzeichnisstruktur

```
C:\TimetagAnalyzer\
├── computeCorrelation.exe
├── data\
│   ├── TimeTags-test_2025-07-08_161026.ttbin
│   └── TimeTags-test_2025-07-08_161026-1.ttbin
└── time_diff_ch1_ch262.txt (wird erstellt)
```

## Fehlerbehebung

- **"Das System kann die angegebene Datei nicht finden"**: Stellen Sie sicher, dass Sie sich im richtigen Verzeichnis befinden
- **Keine .ttbin Dateien gefunden**: Überprüfen Sie, ob der `data\` Ordner existiert und .ttbin Dateien enthält
- **Programm schließt sich sofort**: Führen Sie das Programm über die Kommandozeile aus, um Fehlermeldungen zu sehen

## Windows Defender

Falls Windows Defender eine Warnung anzeigt, können Sie die Datei als sicher markieren. Dies ist normal bei unbekannten .exe Dateien.
