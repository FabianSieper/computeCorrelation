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
	fmt.Println("=== Timetag Correlation Analyzer ===")
	fmt.Println("This program calculates time differences between consecutive events from two channels.")
	fmt.Println()

	// Validate data directory first
	if _, err := os.Stat("data"); os.IsNotExist(err) {
		fmt.Println("ERROR: 'data' directory not found!")
		fmt.Println("Please create a 'data' directory and place your .ttbin files in it.")
		fmt.Println("Example structure:")
		fmt.Println("  your-project/")
		fmt.Println("  ├── computeCorrelation.exe")
		fmt.Println("  └── data/")
		fmt.Println("      ├── file1.ttbin")
		fmt.Println("      └── file2.ttbin")
		fmt.Println()
		fmt.Println("Press Enter to exit...")
		fmt.Scanln()
		return
	}

	// Get channel numbers from user input with validation
	var channel1, channel2 uint16
	
	for {
		fmt.Print("Enter first channel number (1-999): ")
		_, err := fmt.Scanln(&channel1)
		if err != nil || channel1 == 0 || channel1 >= 1000 {
			fmt.Println("Please enter a valid channel number between 1 and 999.")
			// Clear input buffer
			var dummy string
			fmt.Scanln(&dummy)
			continue
		}
		break
	}

	for {
		fmt.Print("Enter second channel number (1-999): ")
		_, err := fmt.Scanln(&channel2)
		if err != nil || channel2 == 0 || channel2 >= 1000 {
			fmt.Println("Please enter a valid channel number between 1 and 999.")
			// Clear input buffer
			var dummy string
			fmt.Scanln(&dummy)
			continue
		}
		if channel2 == channel1 {
			fmt.Println("Second channel must be different from the first channel.")
			continue
		}
		break
	}

	fmt.Printf("\nAnalyzing channels %d and %d...\n\n", channel1, channel2)

	// Get all .ttbin files from data directory
	files, err := getTimeTaFiles("data")
	if err != nil {
		fmt.Printf("ERROR: Failed to read files from data directory: %v\n", err)
		fmt.Println("Please check that the data directory exists and is accessible.")
		fmt.Println("\nPress Enter to exit...")
		fmt.Scanln()
		return
	}

	if len(files) == 0 {
		fmt.Println("ERROR: No .ttbin files found in data/ directory.")
		fmt.Println("Please place your .ttbin files in the data folder.")
		fmt.Println("Supported file extension: .ttbin")
		fmt.Println("\nPress Enter to exit...")
		fmt.Scanln()
		return
	}

	fmt.Printf("Found %d .ttbin file(s) to process:\n", len(files))
	for i, file := range files {
		fmt.Printf("  %d. %s\n", i+1, file)
	}
	fmt.Println()

	// Process all files and calculate time differences
	timeDiffs, err := processFiles(files, channel1, channel2)
	if err != nil {
		fmt.Printf("ERROR: Failed to process files: %v\n", err)
		fmt.Println("This could be due to:")
		fmt.Println("- Corrupted or invalid .ttbin files")
		fmt.Println("- Insufficient memory for very large files")
		fmt.Println("- File permission issues")
		fmt.Println("\nPress Enter to exit...")
		fmt.Scanln()
		return
	}

	if len(timeDiffs) == 0 {
		fmt.Printf("WARNING: No time differences calculated!\n")
		fmt.Printf("This could mean:\n")
		fmt.Printf("- No events found for channels %d or %d\n", channel1, channel2)
		fmt.Printf("- Only one channel has events (need both for differences)\n")
		fmt.Printf("- File format not recognized or corrupted\n")
		fmt.Println("\nPress Enter to exit...")
		fmt.Scanln()
		return
	}

	// Save results to file
	outputFile := fmt.Sprintf("time_diff_ch%d_ch%d.txt", channel1, channel2)
	err = saveTimeDiffs(timeDiffs, outputFile)
	if err != nil {
		fmt.Printf("ERROR: Failed to save results: %v\n", err)
		fmt.Println("Please check that you have write permissions in this directory.")
		fmt.Println("\nPress Enter to exit...")
		fmt.Scanln()
		return
	}

	fmt.Printf("✓ SUCCESS: Calculated %d time differences and saved to '%s'\n", len(timeDiffs), outputFile)
	fmt.Println("\nPress Enter to exit...")
	fmt.Scanln()
}

// getTimeTaFiles returns all .ttbin files sorted lexicographically
func getTimeTaFiles(dataDir string) ([]string, error) {
	// Check if directory exists
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory '%s' does not exist", dataDir)
	}

	pattern := filepath.Join(dataDir, "*.ttbin")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search for .ttbin files: %v", err)
	}

	sort.Strings(files)
	return files, nil
}

// processFiles processes all files and calculates time differences with improved error handling
func processFiles(files []string, channel1, channel2 uint16) ([]TimeDiff, error) {
	var allTimeTags []TimeTag
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Channels for collecting results from goroutines
	results := make(chan []TimeTag, len(files))
	errors := make(chan error, len(files))

	// Process files in parallel (but limit concurrency for very large files)
	maxConcurrency := 4
	if len(files) < maxConcurrency {
		maxConcurrency = len(files)
	}
	
	semaphore := make(chan struct{}, maxConcurrency)

	for _, file := range files {
		wg.Add(1)
		go func(filename string) {
			defer wg.Done()
			
			// Limit concurrency
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			fmt.Printf("Processing file: %s\n", filename)
			
			timeTags, err := readTimeTagFile(filename)
			if err != nil {
				errors <- fmt.Errorf("error reading %s: %v", filename, err)
				return
			}
			
			if len(timeTags) == 0 {
				fmt.Printf("  Warning: No time tags found in %s\n", filename)
			} else {
				fmt.Printf("  Extracted %d time tags from %s\n", len(timeTags), filename)
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

	if len(allTimeTags) == 0 {
		return nil, fmt.Errorf("no time tags found in any file")
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

	fmt.Printf("Found channels and event counts:\n")
	var sortedChannels []uint16
	for ch := range channelCounts {
		sortedChannels = append(sortedChannels, ch)
	}
	sort.Slice(sortedChannels, func(i, j int) bool { return sortedChannels[i] < sortedChannels[j] })
	
	for _, ch := range sortedChannels {
		count := channelCounts[ch]
		if ch == channel1 || ch == channel2 {
			fmt.Printf("  Channel %d: %d Events ← Target channel\n", ch, count)
		} else {
			fmt.Printf("  Channel %d: %d Events\n", ch, count)
		}
	}
	fmt.Println()

	// Check if target channels exist
	if channelCounts[channel1] == 0 {
		return nil, fmt.Errorf("no events found for channel %d", channel1)
	}
	if channelCounts[channel2] == 0 {
		return nil, fmt.Errorf("no events found for channel %d", channel2)
	}

	// Calculate time differences
	return calculateTimeDiffs(allTimeTags, channel1, channel2), nil
}

// readTimeTagFile reads time tags from a .ttbin file with improved error handling
func readTimeTagFile(filename string) ([]TimeTag, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	// Check file size
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %v", err)
	}
	
	if fileInfo.Size() == 0 {
		return nil, fmt.Errorf("file is empty")
	}
	
	if fileInfo.Size() < 32 {
		return nil, fmt.Errorf("file too small to contain valid SITT data (size: %d bytes)", fileInfo.Size())
	}

	var timeTags []TimeTag

	// Find all SITT blocks that might contain time tag data
	blockTypes := []uint32{0x01, 0x02, 0x03}
	foundAnyBlock := false
	
	for _, blockType := range blockTypes {
		blocks, err := findSITTBlocks(file, blockType)
		if err != nil {
			return nil, fmt.Errorf("failed to find SITT blocks: %v", err)
		}

		if len(blocks) > 0 {
			foundAnyBlock = true
			fmt.Printf("  Found: %d blocks of type 0x%x\n", len(blocks), blockType)
		}

		for _, block := range blocks {
			tags, err := readTimeTagDataAtPosition(file, block.Position, block.Length)
			if err != nil {
				return nil, fmt.Errorf("failed to read time tag data: %v", err)
			}
			timeTags = append(timeTags, tags...)
		}
	}

	if !foundAnyBlock {
		return nil, fmt.Errorf("no SITT blocks found - file may not be in SITT format")
	}

	return timeTags, nil
}

// Refactored findSITTBlocks with reduced complexity
func findSITTBlocks(file *os.File, blockType uint32) ([]SITTBlockInfo, error) {
	var blocks []SITTBlockInfo

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := fileInfo.Size()

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	const bufferSize = 1024 * 1024 // 1MB buffer
	reader := bufio.NewReaderSize(file, bufferSize)

	var position int64 = 0
	buffer := make([]byte, 64*1024) // 64KB sliding window
	overlap := 16

	for position < fileSize {
		n, err := reader.Read(buffer[overlap:])
		if err != nil && err != io.EOF {
			return nil, err
		}
		if n == 0 {
			break
		}

		totalBytes := n + overlap
		newBlocks := searchSITTInBuffer(buffer, totalBytes, blockType, position, overlap)
		blocks = append(blocks, newBlocks...)

		// Move overlap data to beginning
		if totalBytes >= overlap {
			copy(buffer[:overlap], buffer[totalBytes-overlap:totalBytes])
		}
		position += int64(n)
	}

	return blocks, nil
}

// Helper function to search for SITT patterns in buffer
func searchSITTInBuffer(buffer []byte, totalBytes int, blockType uint32, position int64, overlap int) []SITTBlockInfo {
	var blocks []SITTBlockInfo
	
	for i := 0; i < totalBytes-16; i += 4 {
		if isSITTMagic(buffer, i) && i+16 <= totalBytes {
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
	
	return blocks
}

// Helper function to check SITT magic bytes
func isSITTMagic(buffer []byte, index int) bool {
	return buffer[index] == 'S' && 
		   buffer[index+1] == 'I' && 
		   buffer[index+2] == 'T' && 
		   buffer[index+3] == 'T'
}

// Refactored readTimeTagDataAtPosition with reduced complexity
func readTimeTagDataAtPosition(file *os.File, position uint64, length uint32) ([]TimeTag, error) {
	if _, err := file.Seek(int64(position+16), io.SeekStart); err != nil {
		return nil, err
	}

	dataSize := calculateDataSize(length)
	if dataSize == 0 {
		return nil, nil
	}

	data, err := readDataBlock(file, dataSize)
	if err != nil {
		return nil, err
	}

	timeTags := parseTimeTagData(data)
	return timeTags, nil
}

// Helper functions to reduce complexity
func calculateDataSize(length uint32) uint32 {
	dataSize := length - 4 // Length includes magic but not rest of header
	if dataSize < 12 {
		return 0
	}
	return dataSize - 12 // Rest of header
}

func readDataBlock(file *os.File, dataSize uint32) ([]byte, error) {
	reader := bufio.NewReader(file)
	data := make([]byte, dataSize)
	_, err := io.ReadFull(reader, data)
	return data, err
}

func parseTimeTagData(data []byte) []TimeTag {
	estimatedEvents := len(data) / 16
	timeTags := make([]TimeTag, 0, estimatedEvents)

	for i := 0; i < len(data); i += 16 {
		if i+16 <= len(data) {
			if tag := tryParseTimeTag(data[i:i+16]); tag != nil {
				timeTags = append(timeTags, *tag)
			}
		}
	}

	return timeTags
}

func tryParseTimeTag(chunk []byte) *TimeTag {
	// Try different parsing approaches
	approaches := []struct {
		channelOffset   int
		timestampOffset int
	}{
		{4, 8},  // Approach 1: Channel at 4-5, timestamp at 8-11
		{0, 4},  // Approach 2: Channel at 0-1, timestamp at 4-7
		{8, 12}, // Approach 3: Channel at 8-9, timestamp at 12-15
	}

	for _, approach := range approaches {
		if approach.timestampOffset+4 <= len(chunk) && approach.channelOffset+2 <= len(chunk) {
			channel := binary.LittleEndian.Uint16(chunk[approach.channelOffset : approach.channelOffset+2])
			timestamp := uint64(binary.LittleEndian.Uint32(chunk[approach.timestampOffset : approach.timestampOffset+4]))

			if isValidTimeTag(channel, timestamp) {
				return &TimeTag{Timestamp: timestamp, Channel: channel}
			}
		}
	}

	return nil
}

func isValidTimeTag(channel uint16, timestamp uint64) bool {
	return channel > 0 && channel < 1000 && timestamp > 0
}

// calculateTimeDiffs with improved error handling and user feedback
func calculateTimeDiffs(timeTags []TimeTag, channel1, channel2 uint16) []TimeDiff {
	// Filter relevant events
	relevantEvents := make([]TimeTag, 0, len(timeTags)/10)
	for _, tag := range timeTags {
		if tag.Channel == channel1 || tag.Channel == channel2 {
			relevantEvents = append(relevantEvents, tag)
		}
	}

	fmt.Printf("Relevant events for analysis: %d\n", len(relevantEvents))

	// Show sample events
	showSampleEvents(relevantEvents)

	// Calculate differences
	timeDiffs := make([]TimeDiff, 0, len(relevantEvents)-1)
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

func showSampleEvents(events []TimeTag) {
	if len(events) == 0 {
		return
	}

	fmt.Println("Sample events (chronologically sorted):")
	maxShow := 10
	if len(events) < maxShow {
		maxShow = len(events)
	}

	for i := 0; i < maxShow; i++ {
		event := events[i]
		fmt.Printf("  Event %d: Channel %d, Timestamp %d\n", i+1, event.Channel, event.Timestamp)
	}

	if len(events) > maxShow {
		fmt.Printf("  ... and %d more events\n", len(events)-maxShow)
	}
	fmt.Println()
}

// saveTimeDiffs with improved error handling
func saveTimeDiffs(timeDiffs []TimeDiff, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create output file '%s': %v", filename, err)
	}
	defer file.Close()

	writer := bufio.NewWriterSize(file, 64*1024)
	defer func() {
		if flushErr := writer.Flush(); flushErr != nil {
			fmt.Printf("Warning: Error flushing data to file: %v\n", flushErr)
		}
	}()

	// Write header with metadata
	fmt.Fprintf(writer, "# Time differences between consecutive events\n")
	fmt.Fprintf(writer, "# Generated by Timetag Correlation Analyzer\n")
	fmt.Fprintf(writer, "# Format: Timestamp1, Timestamp2, Channel1, Channel2, TimeDiff(ns)\n")
	fmt.Fprintf(writer, "# TimeDiff = Timestamp2 - Timestamp1\n")
	fmt.Fprintf(writer, "# Total events: %d\n\n", len(timeDiffs))

	// Write data
	for _, diff := range timeDiffs {
		_, err := fmt.Fprintf(writer, "%d,%d,%d,%d,%d\n",
			diff.Timestamp1, diff.Timestamp2, diff.Channel1, diff.Channel2, diff.Diff)
		if err != nil {
			return fmt.Errorf("failed to write data to file: %v", err)
		}
	}

	return nil
}
