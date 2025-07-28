# Timetag Correlation Analyzer

Dieses Go-Programm berechnet Zeitdifferenzen zwischen zwei Kanälen aus Timetag-Dateien (.ttbin Format).

## Funktionalität

Das Programm:

1. Liest alle .ttbin Dateien aus dem `data/` Ordner (lexikographisch sortiert)
2. Extrahiert Timetag-Daten für zwei benutzerdefinierte Kanäle
3. Berechnet die zeitlichen Differenzen zwischen aufeinanderfolgenden Events der beiden Kanäle
4. Speichert die Ergebnisse in einer CSV-Datei

## Nutzung

1. Kompilieren:

   ```bash
   go build -o computeCorrelation main.go
   ```

2. Ausführen:

   ```bash
   ./computeCorrelation
   ```

3. Das Programm fragt nach zwei Kanalnummern:

   ```text
   Geben Sie die erste Kanalnummer ein: 258
   Geben Sie die zweite Kanalnummer ein: 259
   ```

## Ausgabe

Das Programm erstellt eine Datei namens `time_diff_ch<X>_ch<Y>.txt` mit folgender Struktur:

```text
# Zeitdifferenzen zwischen Kanälen
# Format: Timestamp1, Timestamp2, Channel1, Channel2, TimeDiff(ns)
# TimeDiff = Timestamp2 - Timestamp1

<timestamp1>,<timestamp2>,<channel1>,<channel2>,<time_diff>
...
```

Wobei:

- `timestamp1`: Zeitstempel des ersten Events
- `timestamp2`: Zeitstempel des zweiten Events  
- `channel1`: Kanalnummer des ersten Events
- `channel2`: Kanalnummer des zweiten Events
- `time_diff`: Zeitdifferenz in Nanosekunden (timestamp2 - timestamp1)

## Dateiformat

Das Programm erwartet .ttbin Dateien im SITT (SwissInstruments Time Tag) Format im `data/` Verzeichnis.

## Features

- **Mehrere Dateien**: Verarbeitet automatisch alle .ttbin Dateien in chronologischer Reihenfolge
- **Streaming**: Effiziente Verarbeitung großer Dateien durch blockweise Einlesen
- **Korrelationsanalyse**: Berechnet Zeitdifferenzen zwischen allen aufeinanderfolgenden Events der gewählten Kanäle
- **Kanalübersicht**: Zeigt eine Zusammenfassung aller gefundenen Kanäle und Event-Anzahlen

## Beispielausgabe

```
Geben Sie die erste Kanalnummer ein: 262
Geben Sie die zweite Kanalnummer ein: 261
Verarbeite Datei: data/TimeTags-test_2025-07-08_161026-1.ttbin
  Gefunden: 1 Time Tag Blöcke
Verarbeite Datei: data/TimeTags-test_2025-07-08_161026.ttbin
  Gefunden: 1 Time Tag Blöcke
Insgesamt 2 Time Tags gelesen
Gefundene Kanäle:
  Kanal 262: 2 Events
Erfolgreich 0 Zeitdifferenzen berechnet und in 'time_diff_ch262_ch261.txt' gespeichert.
```
