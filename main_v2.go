package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// TimeTag represents a single time tag entry
type TimeTag struct {
	Timestamp uint64
	Channel   uint16
}

// TimeDiff represents a time difference between two channels
type TimeDiff struct {
	Timestamp1 uint64
	Timestamp2 uint64
	Channel1   uint16
	Channel2   uint16
	Diff       int64
}

func main() {
	// Get channel numbers from user input
	fmt.Print("Geben Sie die erste Kanalnummer ein: ")
	var channel1 uint16
	fmt.Scanln(&channel1)

	fmt.Print("Geben Sie die zweite Kanalnummer ein: ")
	var channel2 uint16
	fmt.Scanln(&channel2)

	// Get all .ttbin files from data directory
	files, err := getTimeTaFiles("data")
	if err != nil {
		fmt.Printf("Fehler beim Lesen der Dateien: %v\n", err)
		return
	}

	// Process all files and calculate time differences
	timeDiffs, err := processFiles(files, channel1, channel2)
	if err != nil {
		fmt.Printf("Fehler beim Verarbeiten der Dateien: %v\n", err)
		return
	}

	// Save results to file
	outputFile := fmt.Sprintf("time_diff_ch%d_ch%d.txt", channel1, channel2)
	err = saveTimeDiffs(timeDiffs, outputFile)
	if err != nil {
		fmt.Printf("Fehler beim Speichern der Ergebnisse: %v\n", err)
		return
	}

	fmt.Printf("Erfolgreich %d Zeitdifferenzen berechnet und in '%s' gespeichert.\n",
		len(timeDiffs), outputFile)
}

// getTimeTaFiles returns all .ttbin files sorted lexicographically
func getTimeTaFiles(dataDir string) ([]string, error) {
	pattern := filepath.Join(dataDir, "*.ttbin")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}

// processFiles processes all files and calculates time differences
func processFiles(files []string, channel1, channel2 uint16) ([]TimeDiff, error) {
	var allTimeTags []TimeTag

	// Read all time tags from all files
	for _, file := range files {
		fmt.Printf("Verarbeite Datei: %s\n", file)
		timeTags, err := readTimeTagFile(file)
		if err != nil {
			return nil, fmt.Errorf("fehler beim Lesen von %s: %v", file, err)
		}
		allTimeTags = append(allTimeTags, timeTags...)
	}

	fmt.Printf("Insgesamt %d Time Tags gelesen\n", len(allTimeTags))

	// Calculate time differences
	return calculateTimeDiffs(allTimeTags, channel1, channel2), nil
}

// readTimeTagFile reads time tags from a .ttbin file using a simpler approach
func readTimeTagFile(filename string) ([]TimeTag, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var timeTags []TimeTag

	// Find all SITT blocks with type 0x03 (time tag data)
	blocks, err := findSITTBlocks(file, 0x03)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Debug: Found %d time tag blocks\n", len(blocks))

	for _, block := range blocks {
		tags, err := readTimeTagDataAtPosition(file, block.Position, block.Length)
		if err != nil {
			return nil, err
		}
		timeTags = append(timeTags, tags...)
	}

	return timeTags, nil
}

// SITTBlockInfo holds information about a SITT block
type SITTBlockInfo struct {
	Position uint64
	Type     uint32
	Length   uint32
}

// findSITTBlocks finds all SITT blocks of a specific type in the file
func findSITTBlocks(file *os.File, blockType uint32) ([]SITTBlockInfo, error) {
	var blocks []SITTBlockInfo

	// Read the entire file to search for SITT patterns
	file.Seek(0, io.SeekStart)
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	// Search for SITT magic bytes
	for i := 0; i < len(data)-16; i += 4 {
		if data[i] == 'S' && data[i+1] == 'I' && data[i+2] == 'T' && data[i+3] == 'T' {
			if i+16 <= len(data) {
				length := binary.LittleEndian.Uint32(data[i+4 : i+8])
				btype := binary.LittleEndian.Uint32(data[i+8 : i+12])

				fmt.Printf("Debug: Found SITT block at 0x%x, type: 0x%x, length: %d\n", i, btype, length)

				if btype == blockType {
					blocks = append(blocks, SITTBlockInfo{
						Position: uint64(i),
						Type:     btype,
						Length:   length,
					})
				}
			}
		}
	}

	return blocks, nil
}

// readTimeTagDataAtPosition reads time tag data from a specific position
func readTimeTagDataAtPosition(file *os.File, position uint64, length uint32) ([]TimeTag, error) {
	// Seek to data position (skip header)
	file.Seek(int64(position+16), io.SeekStart)

	dataSize := length - 4 // Length seems to include magic but not rest of header
	if dataSize < 12 {
		return nil, nil // No data
	}
	dataSize -= 12 // Rest of header

	data := make([]byte, dataSize)
	_, err := file.Read(data)
	if err != nil {
		return nil, err
	}

	var timeTags []TimeTag

	fmt.Printf("Debug: Reading %d bytes of time tag data from position 0x%x\n", dataSize, position)
	if len(data) >= 32 {
		fmt.Printf("Debug: First 32 bytes: %x\n", data[:32])
	}

	// Look for the actual data start - skip initial zeros
	dataStart := 16 // Skip first 16 bytes which seem to be metadata

	fmt.Printf("Debug: Starting data parsing at offset %d\n", dataStart)

	// Parse time tags starting from the actual data
	for i := dataStart; i < len(data); i += 16 {
		if i+16 <= len(data) {
			// Based on hex analysis: 00000000 06010000 fafeffff 00000000
			// Bytes 4-5: Channel (little endian)
			// Bytes 8-11: Timestamp (little endian)

			channel := binary.LittleEndian.Uint16(data[i+4 : i+6])
			timestamp := uint64(binary.LittleEndian.Uint32(data[i+8 : i+12]))

			if channel > 0 && channel < 1000 && timestamp > 0 {
				timeTags = append(timeTags, TimeTag{
					Timestamp: timestamp,
					Channel:   channel,
				})

				fmt.Printf("Debug: Tag %d - Channel: %d, Timestamp: %d (hex: %x)\n",
					len(timeTags), channel, timestamp, timestamp)
			} else {
				// Debug failed parse
				fmt.Printf("Debug: Skipped entry at %d - Channel: %d, Timestamp: %d\n", i, channel, timestamp)
			}
		}
	}

	fmt.Printf("Debug: Extracted %d time tags from this block\n", len(timeTags))
	return timeTags, nil
}

// calculateTimeDiffs calculates time differences between two channels
func calculateTimeDiffs(timeTags []TimeTag, channel1, channel2 uint16) []TimeDiff {
	var timeDiffs []TimeDiff
	var lastCh1Timestamp, lastCh2Timestamp uint64
	var hasCh1, hasCh2 bool

	for _, tag := range timeTags {
		if tag.Channel == channel1 {
			lastCh1Timestamp = tag.Timestamp
			hasCh1 = true

			// If we have a previous timestamp from channel2, calculate difference
			if hasCh2 {
				diff := int64(tag.Timestamp) - int64(lastCh2Timestamp)
				timeDiffs = append(timeDiffs, TimeDiff{
					Timestamp1: lastCh2Timestamp,
					Timestamp2: tag.Timestamp,
					Channel1:   channel2,
					Channel2:   channel1,
					Diff:       diff,
				})
			}
		} else if tag.Channel == channel2 {
			lastCh2Timestamp = tag.Timestamp
			hasCh2 = true

			// If we have a previous timestamp from channel1, calculate difference
			if hasCh1 {
				diff := int64(tag.Timestamp) - int64(lastCh1Timestamp)
				timeDiffs = append(timeDiffs, TimeDiff{
					Timestamp1: lastCh1Timestamp,
					Timestamp2: tag.Timestamp,
					Channel1:   channel1,
					Channel2:   channel2,
					Diff:       diff,
				})
			}
		}
	}

	return timeDiffs
}

// saveTimeDiffs saves time differences to a text file
func saveTimeDiffs(timeDiffs []TimeDiff, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// Write header
	fmt.Fprintf(writer, "# Zeitdifferenzen zwischen Kanälen\n")
	fmt.Fprintf(writer, "# Format: Timestamp1, Timestamp2, Channel1, Channel2, TimeDiff(ns)\n")
	fmt.Fprintf(writer, "# TimeDiff = Timestamp2 - Timestamp1\n\n")

	// Write data
	for _, diff := range timeDiffs {
		fmt.Fprintf(writer, "%d,%d,%d,%d,%d\n",
			diff.Timestamp1, diff.Timestamp2, diff.Channel1, diff.Channel2, diff.Diff)
	}

	return nil
}
