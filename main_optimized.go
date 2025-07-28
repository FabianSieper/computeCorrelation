package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
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

// SITTBlockInfo holds information about a SITT block
type SITTBlockInfo struct {
	Position uint64
	Type     uint32
	Length   uint32
}

func main() {
	// Get channel numbers from user input
	fmt.Print("Enter first channel number: ")
	var channel1 uint16
	fmt.Scanln(&channel1)

	fmt.Print("Enter second channel number: ")
	var channel2 uint16
	fmt.Scanln(&channel2)

	// Get all .ttbin files from data directory
	files, err := getTimeTaFiles("data")
	if err != nil {
		fmt.Printf("Error reading files: %v\n", err)
		return
	}

	if len(files) == 0 {
		fmt.Println("No .ttbin files found in data/ directory.")
		return
	}

	// Process all files and calculate time differences
	timeDiffs, err := processFiles(files, channel1, channel2)
	if err != nil {
		fmt.Printf("Error processing files: %v\n", err)
		return
	}

	// Save results to file
	outputFile := fmt.Sprintf("time_diff_ch%d_ch%d.txt", channel1, channel2)
	err = saveTimeDiffs(timeDiffs, outputFile)
	if err != nil {
		fmt.Printf("Error saving results: %v\n", err)
		return
	}

	fmt.Printf("Successfully calculated %d time differences and saved to '%s'.\n",
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

// processFiles processes all files and calculates time differences using parallel processing
func processFiles(files []string, channel1, channel2 uint16) ([]TimeDiff, error) {
	var allTimeTags []TimeTag
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Channel for collecting results from goroutines
	results := make(chan []TimeTag, len(files))
	errors := make(chan error, len(files))

	// Process files in parallel
	for _, file := range files {
		wg.Add(1)
		go func(filename string) {
			defer wg.Done()
			fmt.Printf("Processing file: %s\n", filename)

			timeTags, err := readTimeTagFile(filename)
			if err != nil {
				errors <- fmt.Errorf("error reading %s: %v", filename, err)
				return
			}
			results <- timeTags
		}(file)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(results)
	close(errors)

	// Check for errors
	select {
	case err := <-errors:
		return nil, err
	default:
	}

	// Collect all results
	for timeTags := range results {
		mu.Lock()
		allTimeTags = append(allTimeTags, timeTags...)
		mu.Unlock()
	}

	fmt.Printf("Total time tags read: %d\n", len(allTimeTags))

	// Sort all events by timestamp to ensure proper chronological order across files
	sort.Slice(allTimeTags, func(i, j int) bool {
		return allTimeTags[i].Timestamp < allTimeTags[j].Timestamp
	})

	// Print summary of found channels
	channelCounts := make(map[uint16]int)
	for _, tag := range allTimeTags {
		channelCounts[tag.Channel]++
	}

	fmt.Println("Found channels:")
	for ch, count := range channelCounts {
		fmt.Printf("  Channel %d: %d Events\n", ch, count)
	}

	// Calculate time differences
	return calculateTimeDiffs(allTimeTags, channel1, channel2), nil
}

// readTimeTagFile reads time tags from a .ttbin file using optimized SITT format parsing
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
			fmt.Printf("  Found: %d blocks of type 0x%x\n", len(blocks), blockType)
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

// findSITTBlocks finds all SITT blocks using memory-efficient streaming approach
func findSITTBlocks(file *os.File, blockType uint32) ([]SITTBlockInfo, error) {
	var blocks []SITTBlockInfo

	// Get file size for better buffer management
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := fileInfo.Size()

	file.Seek(0, io.SeekStart)

	// Use buffered reader for better I/O performance
	const bufferSize = 1024 * 1024 // 1MB buffer for large files
	reader := bufio.NewReaderSize(file, bufferSize)

	var position int64 = 0
	buffer := make([]byte, 64*1024) // 64KB sliding window
	overlap := 16                   // Overlap to catch patterns across buffer boundaries

	for position < fileSize {
		// Read chunk
		n, err := reader.Read(buffer[overlap:])
		if err != nil && err != io.EOF {
			return nil, err
		}
		if n == 0 {
			break
		}

		totalBytes := n + overlap

		// Search for SITT magic bytes in current chunk
		for i := 0; i < totalBytes-16; i += 4 {
			if buffer[i] == 'S' && buffer[i+1] == 'I' && buffer[i+2] == 'T' && buffer[i+3] == 'T' {
				if i+16 <= totalBytes {
					length := binary.LittleEndian.Uint32(buffer[i+4 : i+8])
					btype := binary.LittleEndian.Uint32(buffer[i+8 : i+12])

					if btype == blockType {
						blocks = append(blocks, SITTBlockInfo{
							Position: uint64(position + int64(i) - int64(overlap)),
							Type:     btype,
							Length:   length,
						})
					}
				}
			}
		}

		// Move overlap data to beginning for next iteration
		if totalBytes >= overlap {
			copy(buffer[:overlap], buffer[totalBytes-overlap:totalBytes])
		}
		position += int64(n)
	}

	return blocks, nil
}

// readTimeTagDataAtPosition reads time tag data from a specific position with optimized parsing
func readTimeTagDataAtPosition(file *os.File, position uint64, length uint32) ([]TimeTag, error) {
	// Seek to data position (skip header)
	file.Seek(int64(position+16), io.SeekStart)

	dataSize := length - 4 // Length includes magic but not rest of header
	if dataSize < 12 {
		return nil, nil // No data
	}
	dataSize -= 12 // Rest of header

	// Pre-allocate slice with estimated capacity
	estimatedEvents := int(dataSize / 16) // Each event is typically 16 bytes
	timeTags := make([]TimeTag, 0, estimatedEvents)

	// Use buffered reader for better performance
	reader := bufio.NewReader(file)
	data := make([]byte, dataSize)
	_, err := io.ReadFull(reader, data)
	if err != nil {
		return nil, err
	}

	fmt.Printf("    Debug: Reading %d bytes of time tag data\n", dataSize)

	// Optimized parsing: process data in 16-byte chunks
	for i := 0; i < len(data); i += 16 {
		if i+16 <= len(data) {
			// Try multiple parsing approaches to catch all data
			var validTag *TimeTag

			// Approach 1: Channel at position 4-5, timestamp at 8-11
			channel1 := binary.LittleEndian.Uint16(data[i+4 : i+6])
			timestamp1 := uint64(binary.LittleEndian.Uint32(data[i+8 : i+12]))

			if channel1 > 0 && channel1 < 1000 && timestamp1 > 0 {
				validTag = &TimeTag{Timestamp: timestamp1, Channel: channel1}
			} else {
				// Approach 2: Channel at position 0-1, timestamp at 4-7
				channel2 := binary.LittleEndian.Uint16(data[i : i+2])
				timestamp2 := uint64(binary.LittleEndian.Uint32(data[i+4 : i+8]))

				if channel2 > 0 && channel2 < 1000 && timestamp2 > 0 {
					validTag = &TimeTag{Timestamp: timestamp2, Channel: channel2}
				} else {
					// Approach 3: Channel at position 8-9, timestamp at 12-15
					channel3 := binary.LittleEndian.Uint16(data[i+8 : i+10])
					timestamp3 := uint64(binary.LittleEndian.Uint32(data[i+12 : i+16]))

					if channel3 > 0 && channel3 < 1000 && timestamp3 > 0 {
						validTag = &TimeTag{Timestamp: timestamp3, Channel: channel3}
					}
				}
			}

			if validTag != nil {
				timeTags = append(timeTags, *validTag)
			}
		}
	}

	fmt.Printf("    Debug: Extracted %d time tags\n", len(timeTags))
	return timeTags, nil
}

// calculateTimeDiffs calculates time differences between consecutive events with optimized filtering
func calculateTimeDiffs(timeTags []TimeTag, channel1, channel2 uint16) []TimeDiff {
	// Pre-filter and pre-allocate for better performance
	relevantEvents := make([]TimeTag, 0, len(timeTags)/10) // Estimate 10% relevance

	for _, tag := range timeTags {
		if tag.Channel == channel1 || tag.Channel == channel2 {
			relevantEvents = append(relevantEvents, tag)
		}
	}

	// Events are already sorted by timestamp from processFiles
	fmt.Printf("Relevant events (sorted): %d\n", len(relevantEvents))

	if len(relevantEvents) < 10 {
		// Show all events if few
		for i, event := range relevantEvents {
			fmt.Printf("  Event %d: Channel %d, Timestamp %d\n", i+1, event.Channel, event.Timestamp)
		}
	} else {
		// Show first 10 events
		for i := 0; i < 10; i++ {
			event := relevantEvents[i]
			fmt.Printf("  Event %d: Channel %d, Timestamp %d\n", i+1, event.Channel, event.Timestamp)
		}
	}

	// Pre-allocate timeDiffs slice
	timeDiffs := make([]TimeDiff, 0, len(relevantEvents)-1)

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

// saveTimeDiffs saves time differences to a text file with buffered writing
func saveTimeDiffs(timeDiffs []TimeDiff, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Use large buffer for better write performance
	writer := bufio.NewWriterSize(file, 64*1024)
	defer writer.Flush()

	// Write header
	fmt.Fprintf(writer, "# Time differences between consecutive events\n")
	fmt.Fprintf(writer, "# Format: Timestamp1, Timestamp2, Channel1, Channel2, TimeDiff(ns)\n")
	fmt.Fprintf(writer, "# TimeDiff = Timestamp2 - Timestamp1\n\n")

	// Write data efficiently
	for _, diff := range timeDiffs {
		fmt.Fprintf(writer, "%d,%d,%d,%d,%d\n",
			diff.Timestamp1, diff.Timestamp2, diff.Channel1, diff.Channel2, diff.Diff)
	}

	return nil
}
