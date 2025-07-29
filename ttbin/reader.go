// Package ttbin provides functionality for reading and processing SITT format timetag files (.ttbin)
package ttbin

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

// Constants for ttbin file processing
const (
	TTBinExtension    = "*.ttbin"
	MaxConcurrency    = 4
	DefaultBufferSize = 64 * 1024
	LargeBufferSize   = 1024 * 1024
	MaxSampleEvents   = 1000
	RecordSize        = 10
	SITTHeaderSize    = 16
)

// TimeTag represents a single time tag entry
type TimeTag struct {
	Timestamp uint64
	Channel   uint16
}

// SITTBlockInfo holds information about a SITT block
type SITTBlockInfo struct {
	Position uint64
	Type     uint32
	Length   uint32
}

// Reader provides functionality for reading ttbin files
type Reader struct {
	DataDirectory string
}

// NewReader creates a new ttbin Reader
func NewReader(dataDirectory string) *Reader {
	return &Reader{
		DataDirectory: dataDirectory,
	}
}

// GetTimeTaFiles returns all .ttbin files sorted lexicographically
func (r *Reader) GetTimeTaFiles() ([]string, error) {
	// Check if directory exists
	if _, err := os.Stat(r.DataDirectory); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory '%s' does not exist", r.DataDirectory)
	}

	pattern := filepath.Join(r.DataDirectory, TTBinExtension)
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search for .ttbin files: %v", err)
	}

	sort.Strings(files)
	return files, nil
}

// ScanAvailableChannels scans all files to discover which channels contain events using optimized parallel processing
func (r *Reader) ScanAvailableChannels(files []string) (map[uint16]int, error) {
	channelCounts := make(map[uint16]int)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Process files in parallel for scanning
	maxConcurrency := MaxConcurrency
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

			localCounts, err := r.scanSingleFile(filename)
			if err != nil {
				return // Skip files with errors
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

// ProcessFileOptimized processes a single file with streaming and filtering for better performance
func (r *Reader) ProcessFileOptimized(filename string, channelFilter []uint16, resultChan chan<- TimeTag) (int, error) {
	file, err := r.openAndValidateFile(filename)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	allBlocks, err := r.findAllSITTBlocks(file)
	if err != nil {
		return 0, err
	}

	return r.processBlocksInParallel(file, allBlocks, channelFilter, resultChan)
}

// ProcessFiles processes multiple files in parallel with channel filtering
func (r *Reader) ProcessFiles(files []string, channelFilter []uint16, resultChan chan<- TimeTag) error {
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

			count, err := r.ProcessFileOptimized(filename, channelFilter, resultChan)
			if err != nil {
				errorMu.Lock()
				processingErrors = append(processingErrors, fmt.Errorf("error reading %s: %v", filename, err))
				errorMu.Unlock()
				return
			}

			if count == 0 {
				// This is normal - not all files may contain events for target channels
			}
		}(file)
	}

	// Close channel when all workers are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Wait for all goroutines to complete
	wg.Wait()

	// Check for processing errors
	if len(processingErrors) > 0 {
		return fmt.Errorf("processing errors occurred: %d files had errors", len(processingErrors))
	}

	return nil
}

// scanSingleFile scans a single file for channel information
func (r *Reader) scanSingleFile(filepath string) (map[uint16]int, error) {
	localCounts := make(map[uint16]int)

	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	timeTagBlocks, err := r.findTimeTagBlocks(file)
	if err != nil {
		return localCounts, nil // No TIME_TAG blocks in this file
	}

	if len(timeTagBlocks) == 0 {
		return localCounts, nil
	}

	return r.processBlocksForScanning(filepath, timeTagBlocks, localCounts)
}

func (r *Reader) openAndValidateFile(filename string) (*os.File, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
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

func (r *Reader) findAllSITTBlocks(file *os.File) ([]SITTBlockInfo, error) {
	blockTypes := []uint32{0x01, 0x02, 0x03}
	var allBlocks []SITTBlockInfo
	foundAnyBlock := false

	for _, blockType := range blockTypes {
		blocks, err := r.findSITTBlocks(file, blockType)
		if err != nil {
			return nil, fmt.Errorf("failed to find SITT blocks: %v", err)
		}

		if len(blocks) > 0 {
			foundAnyBlock = true
			allBlocks = append(allBlocks, blocks...)
		}
	}

	if !foundAnyBlock {
		return nil, fmt.Errorf("no SITT blocks found - file may not be in SITT format")
	}

	return allBlocks, nil
}

func (r *Reader) processBlocksInParallel(file *os.File, allBlocks []SITTBlockInfo, channelFilter []uint16, resultChan chan<- TimeTag) (int, error) {
	const maxBlockWorkers = 8
	blockChan := make(chan SITTBlockInfo, len(allBlocks))
	var blockWg sync.WaitGroup
	var totalCount int64
	var countMu sync.Mutex

	numWorkers := maxBlockWorkers
	if len(allBlocks) < numWorkers {
		numWorkers = len(allBlocks)
	}

	// Create channel filter map for efficient lookup
	filterMap := make(map[uint16]bool)
	for _, ch := range channelFilter {
		filterMap[ch] = true
	}

	for i := 0; i < numWorkers; i++ {
		blockWg.Add(1)
		go func() {
			defer blockWg.Done()
			localCount := 0

			for block := range blockChan {
				count, err := r.processBlockOptimized(file, block, filterMap, resultChan)
				if err != nil {
					continue // Skip blocks with errors
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
func (r *Reader) processBlockOptimized(file *os.File, block SITTBlockInfo, channelFilter map[uint16]bool, resultChan chan<- TimeTag) (int, error) {
	// Create a separate file handle for this worker to avoid race conditions
	workerFile, err := os.Open(file.Name())
	if err != nil {
		return 0, err
	}
	defer workerFile.Close()

	if _, err := workerFile.Seek(int64(block.Position+SITTHeaderSize), io.SeekStart); err != nil {
		return 0, err
	}

	dataSize := r.calculateDataSize(block.Length)
	if dataSize == 0 {
		return 0, nil
	}

	data, err := r.readDataBlock(workerFile, dataSize)
	if err != nil {
		return 0, err
	}

	// Parse and filter in one pass
	count := 0
	// Each record is 10 bytes: timestamp(8 bytes) + channel(2 bytes)
	for i := 0; i < len(data); i += RecordSize {
		if i+RecordSize <= len(data) {
			if tag := r.tryParseTimeTag(data[i : i+RecordSize]); tag != nil {
				// Filter for target channels only
				if channelFilter[tag.Channel] {
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

// findSITTBlocks finds SITT blocks of a specific type in the file
func (r *Reader) findSITTBlocks(file *os.File, blockType uint32) ([]SITTBlockInfo, error) {
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
		newBlocks := r.searchSITTInBuffer(buffer, totalBytes, blockType, position, overlap)
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
func (r *Reader) searchSITTInBuffer(buffer []byte, totalBytes int, blockType uint32, position int64, overlap int) []SITTBlockInfo {
	var blocks []SITTBlockInfo

	for i := 0; i < totalBytes-SITTHeaderSize; i += 4 {
		if r.isSITTMagic(buffer, i) && i+SITTHeaderSize <= totalBytes {
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
func (r *Reader) isSITTMagic(buffer []byte, index int) bool {
	return buffer[index] == 'S' &&
		buffer[index+1] == 'I' &&
		buffer[index+2] == 'T' &&
		buffer[index+3] == 'T'
}

// Helper functions to reduce complexity
func (r *Reader) calculateDataSize(length uint32) uint32 {
	dataSize := length - 4 // Length includes magic but not rest of header
	if dataSize < 12 {
		return 0
	}
	return dataSize - 12 // Rest of header
}

func (r *Reader) readDataBlock(file *os.File, dataSize uint32) ([]byte, error) {
	reader := bufio.NewReader(file)
	data := make([]byte, dataSize)
	_, err := io.ReadFull(reader, data)
	return data, err
}

func (r *Reader) tryParseTimeTag(chunk []byte) *TimeTag {
	// Use the same format as the scanner: timestamp(8 bytes) + channel(2 bytes)
	// This matches the format used in scanSingleFile
	if len(chunk) >= RecordSize {
		// Read timestamp (8 bytes, little endian)
		timestamp := binary.LittleEndian.Uint64(chunk[0:8])
		// Read channel (2 bytes, little endian)
		channel := binary.LittleEndian.Uint16(chunk[8:10])

		if r.isValidTimeTag(channel, timestamp) {
			return &TimeTag{Timestamp: timestamp, Channel: channel}
		}
	}

	return nil
}

func (r *Reader) isValidTimeTag(channel uint16, timestamp uint64) bool {
	// Removed arbitrary channel limit - allow all valid uint16 channels
	return timestamp > 0
}

func (r *Reader) findTimeTagBlocks(file *os.File) ([]SITTBlockInfo, error) {
	blockTypes := []uint32{0x01, 0x02, 0x03}
	var timeTagBlocks []SITTBlockInfo

	for _, blockType := range blockTypes {
		blocks, err := r.findSITTBlocks(file, blockType)
		if err == nil {
			timeTagBlocks = append(timeTagBlocks, blocks...)
		}
	}

	return timeTagBlocks, nil
}

func (r *Reader) processBlocksForScanning(filepath string, timeTagBlocks []SITTBlockInfo, localCounts map[uint16]int) (map[uint16]int, error) {
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
			blockLocalCounts := r.scanBlocksWorker(filepath, blockChan)

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

func (r *Reader) scanBlocksWorker(filepath string, blockChan <-chan SITTBlockInfo) map[uint16]int {
	blockLocalCounts := make(map[uint16]int)

	for block := range blockChan {
		// Create separate file handle for this worker
		workerFile, err := os.Open(filepath)
		if err != nil {
			continue
		}

		counts := r.scanSingleBlock(workerFile, block)
		for channel, count := range counts {
			blockLocalCounts[channel] += count
		}

		workerFile.Close()
	}

	return blockLocalCounts
}

func (r *Reader) scanSingleBlock(workerFile *os.File, block SITTBlockInfo) map[uint16]int {
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
