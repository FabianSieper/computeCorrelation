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

	// Get all .ttbin files from data directory first to analyze available channels
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

	fmt.Printf("Found %d .ttbin file(s) to analyze:\n", len(files))
	for i, file := range files {
		fmt.Printf("  %d. %s\n", i+1, file)
	}
	fmt.Println()

	// Scan all files to find available channels
	fmt.Println("Scanning files to discover available channels...")
	availableChannels, err := scanAvailableChannels(files)
	if err != nil {
		fmt.Printf("ERROR: Failed to scan files for channels: %v\n", err)
		fmt.Println("\nPress Enter to exit...")
		fmt.Scanln()
		return
	}

	if len(availableChannels) == 0 {
		fmt.Println("ERROR: No valid channels found in any file.")
		fmt.Println("The files may be corrupted or not in SITT format.")
		fmt.Println("\nPress Enter to exit...")
		fmt.Scanln()
		return
	}

	// Display available channels
	fmt.Printf("\nAvailable channels found in files:\n")
	var sortedChannels []uint16
	for ch := range availableChannels {
		sortedChannels = append(sortedChannels, ch)
	}
	sort.Slice(sortedChannels, func(i, j int) bool { return sortedChannels[i] < sortedChannels[j] })

	for _, ch := range sortedChannels {
		count := availableChannels[ch]
		fmt.Printf("  Channel %d: %d events\n", ch, count)
	}
	fmt.Println()

	// Get channel numbers from user input with validation against available channels
	var channel1, channel2 uint16

	for {
		fmt.Print("Enter first channel number: ")
		_, err := fmt.Scanln(&channel1)
		if err != nil {
			fmt.Println("Please enter a valid number.")
			clearInputBuffer()
			continue
		}
		if _, exists := availableChannels[channel1]; !exists {
			fmt.Printf("Channel %d not found. Please choose from available channels above.\n", channel1)
			continue
		}
		break
	}

	for {
		fmt.Print("Enter second channel number: ")
		_, err := fmt.Scanln(&channel2)
		if err != nil {
			fmt.Println("Please enter a valid number.")
			clearInputBuffer()
			continue
		}
		if _, exists := availableChannels[channel2]; !exists {
			fmt.Printf("Channel %d not found. Please choose from available channels above.\n", channel2)
			continue
		}
		if channel2 == channel1 {
			fmt.Println("Second channel must be different from the first channel.")
			continue
		}
		break
	}

	fmt.Printf("\nAnalyzing channels %d (%d events) and %d (%d events)...\n\n",
		channel1, availableChannels[channel1], channel2, availableChannels[channel2])

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

	// Process all files and calculate time differences using streaming architecture
	outputFile := fmt.Sprintf("time_diff_ch%d_ch%d.txt", channel1, channel2)
	totalDiffs, err := processFilesStreaming(files, channel1, channel2, outputFile)
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

	if totalDiffs == 0 {
		fmt.Printf("WARNING: No time differences calculated!\n")
		fmt.Printf("This could mean:\n")
		fmt.Printf("- No events found for channels %d or %d\n", channel1, channel2)
		fmt.Printf("- Only one channel has events (need both for differences)\n")
		fmt.Printf("- File format not recognized or corrupted\n")
		fmt.Println("\nPress Enter to exit...")
		fmt.Scanln()
		return
	}

	fmt.Printf("✓ SUCCESS: Calculated %d time differences and saved to '%s'\n", totalDiffs, outputFile)
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

// processFilesStreaming processes files with streaming architecture to prevent memory issues
func processFilesStreaming(files []string, channel1, channel2 uint16, outputFile string) (int, error) {
	// Create output file first
	outFile, err := os.Create(outputFile)
	if err != nil {
		return 0, fmt.Errorf("failed to create output file: %v", err)
	}
	defer outFile.Close()

	writer := bufio.NewWriterSize(outFile, 64*1024)
	defer writer.Flush()

	// Write header
	fmt.Fprintf(writer, "# Time differences between consecutive events\n")
	fmt.Fprintf(writer, "# Generated by Timetag Correlation Analyzer\n")
	fmt.Fprintf(writer, "# Format: Timestamp1, Timestamp2, Channel1, Channel2, TimeDiff(ns)\n")
	fmt.Fprintf(writer, "# TimeDiff = Timestamp2 - Timestamp1\n")
	fmt.Fprintf(writer, "# Processing channels %d and %d\n\n", channel1, channel2)

	// Create channel for sorted time tags with small buffer (memory efficient)
	const bufferSize = 1000 // Small buffer to prevent memory issues
	timeTagsChan := make(chan TimeTag, bufferSize)
	var wg sync.WaitGroup
	var processingErrors []error
	var errorMu sync.Mutex

	// Process files in parallel with limited concurrency
	maxConcurrency := 4
	if len(files) < maxConcurrency {
		maxConcurrency = len(files)
	}
	semaphore := make(chan struct{}, maxConcurrency)

	for _, file := range files {
		wg.Add(1)
		go func(filename string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			semaphore <- struct{}{}
			fmt.Printf("Processing file: %s\n", filename)

			count, err := processFileOptimized(filename, channel1, channel2, timeTagsChan)
			if err != nil {
				errorMu.Lock()
				processingErrors = append(processingErrors, fmt.Errorf("error reading %s: %v", filename, err))
				errorMu.Unlock()
				return
			}

			if count == 0 {
				fmt.Printf("  Warning: No relevant time tags found in %s\n", filename)
			} else {
				fmt.Printf("  Extracted %d relevant time tags from %s\n", count, filename)
			}
		}(file)
	}

	// Close channel when all workers are done
	go func() {
		wg.Wait()
		close(timeTagsChan)
	}()

	// Stream processing: collect tags in small batches, sort, calculate diffs, write, then discard
	return streamCalculateAndWrite(timeTagsChan, channel1, channel2, writer, processingErrors)
}

// streamCalculateAndWrite processes time tags in streaming fashion
func streamCalculateAndWrite(timeTagsChan <-chan TimeTag, channel1, channel2 uint16, writer *bufio.Writer, processingErrors []error) (int, error) {
	const batchSize = 5000 // Process in small batches to control memory
	var batch []TimeTag
	var lastEvent *TimeTag
	totalDiffs := 0

	// Check for processing errors first
	if len(processingErrors) > 0 {
		fmt.Printf("Warning: %d files had errors during processing\n", len(processingErrors))
	}

	fmt.Println("Streaming calculation and writing results...")

	for tag := range timeTagsChan {
		batch = append(batch, tag)

		// Process batch when it reaches size limit
		if len(batch) >= batchSize {
			count, newLastEvent := processBatch(batch, channel1, channel2, writer, lastEvent)
			totalDiffs += count
			lastEvent = newLastEvent

			// Clear batch to free memory
			batch = batch[:0]

			// Print progress
			if totalDiffs%1000 == 0 && totalDiffs > 0 {
				fmt.Printf("  Processed %d time differences...\n", totalDiffs)
				writer.Flush() // Ensure data is written regularly
			}
		}
	}

	// Process remaining batch
	if len(batch) > 0 {
		count, _ := processBatch(batch, channel1, channel2, writer, lastEvent)
		totalDiffs += count
	}

	if totalDiffs == 0 {
		return 0, fmt.Errorf("no events found for channels %d and %d", channel1, channel2)
	}

	writer.Flush()
	fmt.Printf("✓ Streaming processing complete: %d time differences calculated\n", totalDiffs)
	return totalDiffs, nil
}

// processBatch processes a small batch of time tags and calculates differences
func processBatch(batch []TimeTag, channel1, channel2 uint16, writer *bufio.Writer, lastEvent *TimeTag) (int, *TimeTag) {
	if len(batch) == 0 {
		return 0, lastEvent
	}

	// Sort this batch
	sort.Slice(batch, func(i, j int) bool {
		return batch[i].Timestamp < batch[j].Timestamp
	})

	count := 0
	var prevEvent *TimeTag

	// Use lastEvent from previous batch as starting point
	if lastEvent != nil {
		prevEvent = lastEvent
	}

	for i := 0; i < len(batch); i++ {
		currentEvent := &batch[i]

		// Calculate difference if we have a previous event and channels are different
		if prevEvent != nil && prevEvent.Channel != currentEvent.Channel {
			// Check if this involves our target channels
			if (prevEvent.Channel == channel1 || prevEvent.Channel == channel2) &&
				(currentEvent.Channel == channel1 || currentEvent.Channel == channel2) {

				diff := int64(currentEvent.Timestamp) - int64(prevEvent.Timestamp)

				// Write directly to file
				fmt.Fprintf(writer, "%d,%d,%d,%d,%d\n",
					prevEvent.Timestamp, currentEvent.Timestamp,
					prevEvent.Channel, currentEvent.Channel, diff)
				count++
			}
		}
		prevEvent = currentEvent
	}

	// Return the last event from this batch for next batch
	return count, prevEvent
}

// processFiles processes all files and calculates time differences with improved performance using workers
func processFiles(files []string, channel1, channel2 uint16) ([]TimeDiff, error) {
	// Use buffered channel to collect filtered time tags directly
	const bufferSize = 10000
	allTimeTagsChan := make(chan TimeTag, bufferSize)
	var wg sync.WaitGroup
	var processingErrors []error
	var errorMu sync.Mutex

	// Process files in parallel with optimized worker pool
	maxConcurrency := 4
	if len(files) < maxConcurrency {
		maxConcurrency = len(files)
	}

	semaphore := make(chan struct{}, maxConcurrency)

	for _, file := range files {
		wg.Add(1)
		go func(filename string) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore

			// Acquire semaphore
			semaphore <- struct{}{}

			fmt.Printf("Processing file: %s\n", filename)

			// Process file and stream filtered results directly to channel
			count, err := processFileOptimized(filename, channel1, channel2, allTimeTagsChan)
			if err != nil {
				errorMu.Lock()
				processingErrors = append(processingErrors, fmt.Errorf("error reading %s: %v", filename, err))
				errorMu.Unlock()
				return
			}

			if count == 0 {
				fmt.Printf("  Warning: No relevant time tags found in %s\n", filename)
			} else {
				fmt.Printf("  Extracted %d relevant time tags from %s\n", count, filename)
			}
		}(file)
	}

	// Close channel when all workers are done
	go func() {
		wg.Wait()
		close(allTimeTagsChan)
	}()

	// Collect results from channel into slice
	var allTimeTags []TimeTag
	for tag := range allTimeTagsChan {
		allTimeTags = append(allTimeTags, tag)
	}

	// Check for errors
	if len(processingErrors) > 0 {
		if len(allTimeTags) == 0 {
			return nil, processingErrors[0] // Return first error if no data
		}
		// Log warnings but continue if we have some data
		fmt.Printf("Warning: %d files had errors but continuing with available data\n", len(processingErrors))
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

// processFileOptimized processes a single file with streaming and filtering for better performance
func processFileOptimized(filename string, channel1, channel2 uint16, resultChan chan<- TimeTag) (int, error) {
	file, err := os.Open(filename)
	if err != nil {
		return 0, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	// Check file size
	fileInfo, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %v", err)
	}

	if fileInfo.Size() == 0 {
		return 0, fmt.Errorf("file is empty")
	}

	if fileInfo.Size() < 32 {
		return 0, fmt.Errorf("file too small to contain valid SITT data (size: %d bytes)", fileInfo.Size())
	}

	// Find all SITT blocks that might contain time tag data
	blockTypes := []uint32{0x01, 0x02, 0x03}
	var allBlocks []SITTBlockInfo
	foundAnyBlock := false

	for _, blockType := range blockTypes {
		blocks, err := findSITTBlocks(file, blockType)
		if err != nil {
			return 0, fmt.Errorf("failed to find SITT blocks: %v", err)
		}

		if len(blocks) > 0 {
			foundAnyBlock = true
			fmt.Printf("  Found: %d blocks of type 0x%x\n", len(blocks), blockType)
			allBlocks = append(allBlocks, blocks...)
		}
	}

	if !foundAnyBlock {
		return 0, fmt.Errorf("no SITT blocks found - file may not be in SITT format")
	}

	// Process blocks in parallel using worker pool
	const maxBlockWorkers = 8
	blockChan := make(chan SITTBlockInfo, len(allBlocks))
	var blockWg sync.WaitGroup
	var totalCount int64
	var countMu sync.Mutex

	// Start block processing workers
	numWorkers := maxBlockWorkers
	if len(allBlocks) < numWorkers {
		numWorkers = len(allBlocks)
	}

	for i := 0; i < numWorkers; i++ {
		blockWg.Add(1)
		go func() {
			defer blockWg.Done()
			localCount := 0

			for block := range blockChan {
				count, err := processBlockOptimized(file, block, channel1, channel2, resultChan)
				if err != nil {
					// Log error but continue processing other blocks
					fmt.Printf("    Warning: Error processing block at %d: %v\n", block.Position, err)
					continue
				}
				localCount += count
			}

			countMu.Lock()
			totalCount += int64(localCount)
			countMu.Unlock()
		}()
	}

	// Send blocks to workers
	for _, block := range allBlocks {
		blockChan <- block
	}
	close(blockChan)

	// Wait for all block workers to complete
	blockWg.Wait()

	return int(totalCount), nil
}

// processBlockOptimized processes a single SITT block with filtering
func processBlockOptimized(file *os.File, block SITTBlockInfo, channel1, channel2 uint16, resultChan chan<- TimeTag) (int, error) {
	// Create a separate file handle for this worker to avoid race conditions
	workerFile, err := os.Open(file.Name())
	if err != nil {
		return 0, err
	}
	defer workerFile.Close()

	if _, err := workerFile.Seek(int64(block.Position+16), io.SeekStart); err != nil {
		return 0, err
	}

	dataSize := calculateDataSize(block.Length)
	if dataSize == 0 {
		return 0, nil
	}

	data, err := readDataBlock(workerFile, dataSize)
	if err != nil {
		return 0, err
	}

	// Parse and filter in one pass
	count := 0
	// Each record is 10 bytes: timestamp(8 bytes) + channel(2 bytes)
	for i := 0; i < len(data); i += 10 {
		if i+10 <= len(data) {
			if tag := tryParseTimeTag(data[i : i+10]); tag != nil {
				// Filter for target channels only
				if tag.Channel == channel1 || tag.Channel == channel2 {
					select {
					case resultChan <- *tag:
						count++
					default:
						// Channel is full, this shouldn't happen with proper buffering
						// but we handle it gracefully
						resultChan <- *tag
						count++
					}
				}
			}
		}
	}

	return count, nil
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
	// Each record is 10 bytes: timestamp(8 bytes) + channel(2 bytes)
	estimatedEvents := len(data) / 10
	timeTags := make([]TimeTag, 0, estimatedEvents)

	for i := 0; i < len(data); i += 10 {
		if i+10 <= len(data) {
			if tag := tryParseTimeTag(data[i : i+10]); tag != nil {
				timeTags = append(timeTags, *tag)
			}
		}
	}

	return timeTags
}

func tryParseTimeTag(chunk []byte) *TimeTag {
	// Use the same format as the scanner: timestamp(8 bytes) + channel(2 bytes)
	// This matches the format used in scanSingleFile
	if len(chunk) >= 10 {
		// Read timestamp (8 bytes, little endian)
		timestamp := binary.LittleEndian.Uint64(chunk[0:8])
		// Read channel (2 bytes, little endian)
		channel := binary.LittleEndian.Uint16(chunk[8:10])

		if isValidTimeTag(channel, timestamp) {
			return &TimeTag{Timestamp: timestamp, Channel: channel}
		}
	}

	return nil
}

func isValidTimeTag(channel uint16, timestamp uint64) bool {
	// Removed arbitrary channel limit - allow all valid uint16 channels
	return timestamp > 0
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

// scanAvailableChannels scans all files to discover which channels contain events using optimized parallel processing
func scanAvailableChannels(files []string) (map[uint16]int, error) {
	channelCounts := make(map[uint16]int)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Process files in parallel for scanning
	maxConcurrency := 4
	if len(files) < maxConcurrency {
		maxConcurrency = len(files)
	}
	semaphore := make(chan struct{}, maxConcurrency)

	for _, filepath := range files {
		wg.Add(1)
		go func(filename string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			semaphore <- struct{}{} // Acquire semaphore

			fmt.Printf("  Scanning %s...\n", filename)

			localCounts, err := scanSingleFile(filename)
			if err != nil {
				fmt.Printf("    Warning: Failed to scan %s: %v\n", filename, err)
				return
			}

			// Merge results thread-safely
			mu.Lock()
			for channel, count := range localCounts {
				channelCounts[channel] += count
			}
			mu.Unlock()
		}(filepath)
	}

	wg.Wait()
	return channelCounts, nil
}

// scanSingleFile scans a single file for channel information
func scanSingleFile(filepath string) (map[uint16]int, error) {
	localCounts := make(map[uint16]int)

	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	// Find TIME_TAG blocks (try different block types like in readTimeTagFile)
	blockTypes := []uint32{0x01, 0x02, 0x03}
	var timeTagBlocks []SITTBlockInfo
	for _, blockType := range blockTypes {
		blocks, err := findSITTBlocks(file, blockType)
		if err == nil {
			timeTagBlocks = append(timeTagBlocks, blocks...)
		}
	}

	if len(timeTagBlocks) == 0 {
		return localCounts, nil // No TIME_TAG blocks in this file
	}

	// Process blocks in parallel within the file
	const maxSampleEvents = 1000
	blockChan := make(chan SITTBlockInfo, len(timeTagBlocks))
	var blockWg sync.WaitGroup
	var blockMu sync.Mutex

	// Limit concurrent block processing
	maxBlockWorkers := 4
	if len(timeTagBlocks) < maxBlockWorkers {
		maxBlockWorkers = len(timeTagBlocks)
	}

	for i := 0; i < maxBlockWorkers; i++ {
		blockWg.Add(1)
		go func() {
			defer blockWg.Done()
			blockLocalCounts := make(map[uint16]int)

			for block := range blockChan {
				// Create separate file handle for this worker
				workerFile, err := os.Open(filepath)
				if err != nil {
					continue
				}

				dataStart := block.Position + 16
				_, err = workerFile.Seek(int64(dataStart), io.SeekStart)
				if err != nil {
					workerFile.Close()
					continue
				}

				reader := bufio.NewReader(workerFile)
				eventCount := 0
				dataSize := block.Length - 16

				for eventCount < maxSampleEvents && uint32(eventCount*10) < dataSize {
					var timetag uint64
					var channel uint16

					// Read timetag (8 bytes, little endian)
					err := binary.Read(reader, binary.LittleEndian, &timetag)
					if err != nil {
						break
					}

					// Read channel (2 bytes, little endian)
					err = binary.Read(reader, binary.LittleEndian, &channel)
					if err != nil {
						break
					}

					blockLocalCounts[channel]++
					eventCount++
				}
				workerFile.Close()
			}

			// Merge block results
			blockMu.Lock()
			for channel, count := range blockLocalCounts {
				localCounts[channel] += count
			}
			blockMu.Unlock()
		}()
	}

	// Send blocks to workers
	for _, block := range timeTagBlocks {
		blockChan <- block
	}
	close(blockChan)

	blockWg.Wait()
	return localCounts, nil
}

// clearInputBuffer clears the input buffer to handle invalid input
func clearInputBuffer() {
	var dummy string
	fmt.Scanln(&dummy)
}
