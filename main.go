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

// Constants to avoid duplication
const (
	PressEnterToExit  = "\nPress Enter to exit..."
	FailedToOpenFile  = "failed to open file: %v"
	DataDirectory     = "data"
	TTBinExtension    = "*.ttbin"
	MaxConcurrency    = 4
	DefaultBufferSize = 64 * 1024
	LargeBufferSize   = 1024 * 1024
	BatchSize         = 5000
	MaxSampleEvents   = 1000
	RecordSize        = 10
	SITTHeaderSize    = 16
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
	printHeader()

	if err := validateDataDirectory(); err != nil {
		handleError(err)
		return
	}

	files, err := getAndValidateFiles()
	if err != nil {
		handleError(err)
		return
	}

	availableChannels, err := discoverChannels(files)
	if err != nil {
		handleError(err)
		return
	}

	channel1, channel2, err := getUserChannelInput(availableChannels)
	if err != nil {
		handleError(err)
		return
	}

	if err := processAndSaveResults(files, channel1, channel2, availableChannels); err != nil {
		handleError(err)
		return
	}

	fmt.Printf("✓ SUCCESS: Results saved successfully")
	fmt.Print(PressEnterToExit)
	fmt.Scanln()
}

func printHeader() {
	fmt.Println("=== Timetag Correlation Analyzer ===")
	fmt.Println("This program calculates time differences between consecutive events from two channels.")
	fmt.Println()
}

func validateDataDirectory() error {
	if _, err := os.Stat(DataDirectory); os.IsNotExist(err) {
		return fmt.Errorf("'%s' directory not found! Please create a '%s' directory and place your .ttbin files in it", DataDirectory, DataDirectory)
	}
	return nil
}

func getAndValidateFiles() ([]string, error) {
	files, err := getTimeTaFiles(DataDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to read files from data directory: %v", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no .ttbin files found in %s/ directory. Please place your .ttbin files in the data folder", DataDirectory)
	}

	fmt.Printf("Found %d .ttbin file(s) to analyze:\n", len(files))
	for i, file := range files {
		fmt.Printf("  %d. %s\n", i+1, file)
	}
	fmt.Println()

	return files, nil
}

func discoverChannels(files []string) (map[uint16]int, error) {
	fmt.Println("Scanning files to discover available channels...")
	availableChannels, err := scanAvailableChannels(files)
	if err != nil {
		return nil, fmt.Errorf("failed to scan files for channels: %v", err)
	}

	if len(availableChannels) == 0 {
		return nil, fmt.Errorf("no valid channels found in any file. The files may be corrupted or not in SITT format")
	}

	displayAvailableChannels(availableChannels)
	return availableChannels, nil
}

func displayAvailableChannels(availableChannels map[uint16]int) {
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
}

func getUserChannelInput(availableChannels map[uint16]int) (uint16, uint16, error) {
	// Use MaxUint16 as a special value to indicate "no exclusion" for the first channel
	channel1, err := getValidChannelInput("Enter first channel number: ", availableChannels, ^uint16(0))
	if err != nil {
		return 0, 0, err
	}

	channel2, err := getValidChannelInput("Enter second channel number: ", availableChannels, channel1)
	if err != nil {
		return 0, 0, err
	}

	fmt.Printf("\nAnalyzing channels %d (%d events) and %d (%d events)...\n\n",
		channel1, availableChannels[channel1], channel2, availableChannels[channel2])

	return channel1, channel2, nil
}

func getValidChannelInput(prompt string, availableChannels map[uint16]int, excludeChannel uint16) (uint16, error) {
	for {
		fmt.Print(prompt)
		var channel uint16
		_, err := fmt.Scanln(&channel)
		if err != nil {
			fmt.Println("Please enter a valid number.")
			clearInputBuffer()
			continue
		}
		if _, exists := availableChannels[channel]; !exists {
			fmt.Printf("Channel %d not found. Please choose from available channels above.\n", channel)
			continue
		}
		// Only check for exclusion if excludeChannel is not the special "no exclusion" value
		if excludeChannel != ^uint16(0) && channel == excludeChannel {
			fmt.Println("Second channel must be different from the first channel.")
			continue
		}
		return channel, nil
	}
}

func processAndSaveResults(files []string, channel1, channel2 uint16, availableChannels map[uint16]int) error {
	outputFile := fmt.Sprintf("time_diff_ch%d_ch%d.txt", channel1, channel2)
	totalDiffs, err := processFilesStreaming(files, channel1, channel2, outputFile)
	if err != nil {
		return fmt.Errorf("failed to process files: %v", err)
	}

	if totalDiffs == 0 {
		return fmt.Errorf("no time differences calculated! No events found for channels %d or %d", channel1, channel2)
	}

	fmt.Printf("✓ SUCCESS: Calculated %d time differences and saved to '%s'\n", totalDiffs, outputFile)
	return nil
}

func handleError(err error) {
	fmt.Printf("ERROR: %v\n", err)
	fmt.Print(PressEnterToExit)
	fmt.Scanln()
}

// getTimeTaFiles returns all .ttbin files sorted lexicographically
func getTimeTaFiles(dataDir string) ([]string, error) {
	// Check if directory exists
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory '%s' does not exist", dataDir)
	}

	pattern := filepath.Join(dataDir, TTBinExtension)
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search for .ttbin files: %v", err)
	}

	sort.Strings(files)
	return files, nil
}

// processFilesStreaming processes files with streaming architecture to prevent memory issues
func processFilesStreaming(files []string, channel1, channel2 uint16, outputFile string) (int, error) {
	outFile, err := createOutputFile(outputFile, channel1, channel2)
	if err != nil {
		return 0, err
	}
	defer outFile.Close()

	writer := bufio.NewWriterSize(outFile, DefaultBufferSize)
	defer writer.Flush()

	timeTagsChan := make(chan TimeTag, 1000)
	processingErrors := processFilesInParallel(files, channel1, channel2, timeTagsChan)

	return streamCalculateAndWrite(timeTagsChan, channel1, channel2, writer, processingErrors)
}

func createOutputFile(outputFile string, channel1, channel2 uint16) (*os.File, error) {
	outFile, err := os.Create(outputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %v", err)
	}

	writer := bufio.NewWriterSize(outFile, DefaultBufferSize)
	fmt.Fprintf(writer, "# Time differences between consecutive events\n")
	fmt.Fprintf(writer, "# Generated by Timetag Correlation Analyzer\n")
	fmt.Fprintf(writer, "# Format: Timestamp1, Timestamp2, Channel1, Channel2, TimeDiff(ns)\n")
	fmt.Fprintf(writer, "# TimeDiff = Timestamp2 - Timestamp1\n")
	fmt.Fprintf(writer, "# Processing channels %d and %d\n\n", channel1, channel2)
	writer.Flush()

	return outFile, nil
}

func processFilesInParallel(files []string, channel1, channel2 uint16, timeTagsChan chan TimeTag) []error {
	var wg sync.WaitGroup
	var processingErrors []error
	var errorMu sync.Mutex

	maxConcurrency := MaxConcurrency
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

	return processingErrors
}

// streamCalculateAndWrite processes time tags in streaming fashion
func streamCalculateAndWrite(timeTagsChan <-chan TimeTag, channel1, channel2 uint16, writer *bufio.Writer, processingErrors []error) (int, error) {
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
		if len(batch) >= BatchSize {
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

// processFileOptimized processes a single file with streaming and filtering for better performance
func processFileOptimized(filename string, channel1, channel2 uint16, resultChan chan<- TimeTag) (int, error) {
	file, err := openAndValidateFile(filename)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	allBlocks, err := findAllSITTBlocks(file)
	if err != nil {
		return 0, err
	}

	return processBlocksInParallel(file, allBlocks, channel1, channel2, resultChan)
}

func openAndValidateFile(filename string) (*os.File, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf(FailedToOpenFile, err)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to get file info: %v", err)
	}

	if fileInfo.Size() == 0 {
		file.Close()
		return nil, fmt.Errorf("file is empty")
	}

	if fileInfo.Size() < 32 {
		file.Close()
		return nil, fmt.Errorf("file too small to contain valid SITT data (size: %d bytes)", fileInfo.Size())
	}

	return file, nil
}

func findAllSITTBlocks(file *os.File) ([]SITTBlockInfo, error) {
	blockTypes := []uint32{0x01, 0x02, 0x03}
	var allBlocks []SITTBlockInfo
	foundAnyBlock := false

	for _, blockType := range blockTypes {
		blocks, err := findSITTBlocks(file, blockType)
		if err != nil {
			return nil, fmt.Errorf("failed to find SITT blocks: %v", err)
		}

		if len(blocks) > 0 {
			foundAnyBlock = true
			fmt.Printf("  Found: %d blocks of type 0x%x\n", len(blocks), blockType)
			allBlocks = append(allBlocks, blocks...)
		}
	}

	if !foundAnyBlock {
		return nil, fmt.Errorf("no SITT blocks found - file may not be in SITT format")
	}

	return allBlocks, nil
}

func processBlocksInParallel(file *os.File, allBlocks []SITTBlockInfo, channel1, channel2 uint16, resultChan chan<- TimeTag) (int, error) {
	const maxBlockWorkers = 8
	blockChan := make(chan SITTBlockInfo, len(allBlocks))
	var blockWg sync.WaitGroup
	var totalCount int64
	var countMu sync.Mutex

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

	for _, block := range allBlocks {
		blockChan <- block
	}
	close(blockChan)

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

	if _, err := workerFile.Seek(int64(block.Position+SITTHeaderSize), io.SeekStart); err != nil {
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
	for i := 0; i < len(data); i += RecordSize {
		if i+RecordSize <= len(data) {
			if tag := tryParseTimeTag(data[i : i+RecordSize]); tag != nil {
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

	reader := bufio.NewReaderSize(file, LargeBufferSize)

	var position int64 = 0
	buffer := make([]byte, DefaultBufferSize)
	overlap := SITTHeaderSize

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

	for i := 0; i < totalBytes-SITTHeaderSize; i += 4 {
		if isSITTMagic(buffer, i) && i+SITTHeaderSize <= totalBytes {
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

func tryParseTimeTag(chunk []byte) *TimeTag {
	// Use the same format as the scanner: timestamp(8 bytes) + channel(2 bytes)
	// This matches the format used in scanSingleFile
	if len(chunk) >= RecordSize {
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
		return nil, fmt.Errorf(FailedToOpenFile, err)
	}
	defer file.Close()

	timeTagBlocks, err := findTimeTagBlocks(file)
	if err != nil {
		return localCounts, nil // No TIME_TAG blocks in this file
	}

	if len(timeTagBlocks) == 0 {
		return localCounts, nil
	}

	return processBlocksForScanning(filepath, timeTagBlocks, localCounts)
}

func findTimeTagBlocks(file *os.File) ([]SITTBlockInfo, error) {
	blockTypes := []uint32{0x01, 0x02, 0x03}
	var timeTagBlocks []SITTBlockInfo

	for _, blockType := range blockTypes {
		blocks, err := findSITTBlocks(file, blockType)
		if err == nil {
			timeTagBlocks = append(timeTagBlocks, blocks...)
		}
	}

	return timeTagBlocks, nil
}

func processBlocksForScanning(filepath string, timeTagBlocks []SITTBlockInfo, localCounts map[uint16]int) (map[uint16]int, error) {
	blockChan := make(chan SITTBlockInfo, len(timeTagBlocks))
	var blockWg sync.WaitGroup
	var blockMu sync.Mutex

	// Limit concurrent block processing
	maxBlockWorkers := MaxConcurrency
	if len(timeTagBlocks) < maxBlockWorkers {
		maxBlockWorkers = len(timeTagBlocks)
	}

	for i := 0; i < maxBlockWorkers; i++ {
		blockWg.Add(1)
		go func() {
			defer blockWg.Done()
			blockLocalCounts := scanBlocksWorker(filepath, blockChan)

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

func scanBlocksWorker(filepath string, blockChan <-chan SITTBlockInfo) map[uint16]int {
	blockLocalCounts := make(map[uint16]int)

	for block := range blockChan {
		// Create separate file handle for this worker
		workerFile, err := os.Open(filepath)
		if err != nil {
			continue
		}

		counts := scanSingleBlock(workerFile, block)
		for channel, count := range counts {
			blockLocalCounts[channel] += count
		}

		workerFile.Close()
	}

	return blockLocalCounts
}

func scanSingleBlock(workerFile *os.File, block SITTBlockInfo) map[uint16]int {
	blockCounts := make(map[uint16]int)

	dataStart := block.Position + SITTHeaderSize
	_, err := workerFile.Seek(int64(dataStart), io.SeekStart)
	if err != nil {
		return blockCounts
	}

	reader := bufio.NewReader(workerFile)
	eventCount := 0
	dataSize := block.Length - SITTHeaderSize

	for eventCount < MaxSampleEvents && uint32(eventCount*RecordSize) < dataSize {
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

		blockCounts[channel]++
		eventCount++
	}

	return blockCounts
}

// clearInputBuffer clears the input buffer to handle invalid input
func clearInputBuffer() {
	var dummy string
	fmt.Scanln(&dummy)
}
