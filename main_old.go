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

	if len(files) == 0 {
		fmt.Println("Keine .ttbin Dateien im data/ Verzeichnis gefunden.")
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

	// Print summary of found channels
	channelCounts := make(map[uint16]int)
	for _, tag := range allTimeTags {
		channelCounts[tag.Channel]++
	}
	
	fmt.Println("Gefundene Kanäle:")
	for ch, count := range channelCounts {
		fmt.Printf("  Kanal %d: %d Events\n", ch, count)
	}

	// Print first few events for debugging
	fmt.Println("Erste Events (chronologisch sortiert):")
	for i, tag := range allTimeTags {
		if i < 10 { // Show first 10 events
			fmt.Printf("  Event %d: Kanal %d, Timestamp %d\n", i+1, tag.Channel, tag.Timestamp)
		}
	}

	// Calculate time differences
	return calculateTimeDiffs(allTimeTags, channel1, channel2), nil
}

// readTimeTagFile reads time tags from a .ttbin file using SITT format parsing
func readTimeTagFile(filename string) ([]TimeTag, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var timeTags []TimeTag
	
	// Find all SITT blocks that might contain time tag data (types 0x01, 0x02, 0x03)
	for _, blockType := range []uint32{0x01, 0x02, 0x03} {
		blocks, err := findSITTBlocks(file, blockType)
		if err != nil {
			return nil, err
		}
		
		if len(blocks) > 0 {
			fmt.Printf("  Gefunden: %d Blöcke vom Typ 0x%x\n", len(blocks), blockType)
		}
		
		for _, block := range blocks {
			tags, err := readTimeTagDataAtPosition(file, block.Position, block.Length)
			if err != nil {
				return nil, err
			}
			timeTags = append(timeTags, tags...)
		}
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
	
	dataSize := length - 4 // Length includes magic but not rest of header  
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
	
	fmt.Printf("  Debug: Reading %d bytes of time tag data\n", dataSize)
	
	// Skip first 16 bytes which seem to be metadata, then parse all 16-byte entries
	for i := 16; i < len(data); i += 16 {
		if i+16 <= len(data) {
			// Try multiple parsing approaches to catch all data
			
			// Approach 1: Channel at position 4-5, timestamp at 8-11
			channel1 := binary.LittleEndian.Uint16(data[i+4 : i+6])
			timestamp1 := uint64(binary.LittleEndian.Uint32(data[i+8 : i+12]))
			
			// Approach 2: Channel at position 0-1, timestamp at 4-7  
			channel2 := binary.LittleEndian.Uint16(data[i : i+2])
			timestamp2 := uint64(binary.LittleEndian.Uint32(data[i+4 : i+8]))
			
			// Approach 3: Channel at position 8-9, timestamp at 12-15
			channel3 := binary.LittleEndian.Uint16(data[i+8 : i+10])
			timestamp3 := uint64(binary.LittleEndian.Uint32(data[i+12 : i+16]))
			
			// Use whichever approach gives reasonable values
			if channel1 > 0 && channel1 < 1000 && timestamp1 > 0 {
				timeTags = append(timeTags, TimeTag{
					Timestamp: timestamp1,
					Channel:   channel1,
				})
			} else if channel2 > 0 && channel2 < 1000 && timestamp2 > 0 {
				timeTags = append(timeTags, TimeTag{
					Timestamp: timestamp2,
					Channel:   channel2,
				})
			} else if channel3 > 0 && channel3 < 1000 && timestamp3 > 0 {
				timeTags = append(timeTags, TimeTag{
					Timestamp: timestamp3,
					Channel:   channel3,
				})
			}
		}
	}
	
	fmt.Printf("  Debug: Extracted %d time tags\n", len(timeTags))
	return timeTags, nil
}

// calculateTimeDiffs calculates time differences between consecutive events of specified channels
func calculateTimeDiffs(timeTags []TimeTag, channel1, channel2 uint16) []TimeDiff {
	var timeDiffs []TimeDiff
	
	// Filter events for the specified channels and sort by timestamp
	var relevantEvents []TimeTag
	for _, tag := range timeTags {
		if tag.Channel == channel1 || tag.Channel == channel2 {
			relevantEvents = append(relevantEvents, tag)
		}
	}
	
	// Sort by timestamp to ensure chronological order
	sort.Slice(relevantEvents, func(i, j int) bool {
		return relevantEvents[i].Timestamp < relevantEvents[j].Timestamp
	})
	
	fmt.Printf("Relevante Events (sortiert): %d\n", len(relevantEvents))
	for i, event := range relevantEvents {
		if i < 10 { // Show first 10
			fmt.Printf("  Event %d: Kanal %d, Timestamp %d\n", i+1, event.Channel, event.Timestamp)
		}
	}
	
	// Calculate differences between consecutive events
	for i := 1; i < len(relevantEvents); i++ {
		prevEvent := relevantEvents[i-1]
		currentEvent := relevantEvents[i]
		
		diff := int64(currentEvent.Timestamp) - int64(prevEvent.Timestamp)
		
		timeDiffs = append(timeDiffs, TimeDiff{
			Timestamp1: prevEvent.Timestamp,
			Timestamp2: currentEvent.Timestamp,
			Channel1:   prevEvent.Channel,
			Channel2:   currentEvent.Channel,
			Diff:       diff,
		})
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
