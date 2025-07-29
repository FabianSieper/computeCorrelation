package main

import (
	"encoding/binary"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run debug_deeper.go <ttbin_file>")
		os.Exit(1)
	}

	filename := os.Args[1]
	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		fmt.Printf("Error getting file info: %v\n", err)
		os.Exit(1)
	}
	fileSize := fileInfo.Size()
	fmt.Printf("File: %s (size: %d bytes)\n", filename, fileSize)

	// Scan for all SITT blocks
	data := make([]byte, 4096)
	var offset int64 = 0

	fmt.Println("\nScanning for SITT blocks:")
	
	for offset < fileSize {
		n, err := file.ReadAt(data, offset)
		if err != nil && n == 0 {
			break
		}

		// Look for SITT magic in this chunk
		for i := 0; i < n-12; i++ {
			if offset+int64(i)+12 > fileSize {
				break
			}

			magic := binary.LittleEndian.Uint32(data[i:i+4])
			if magic == 0x54544953 { // "SITT" in little endian
				length := binary.LittleEndian.Uint32(data[i+4:i+8])
				btype := binary.LittleEndian.Uint32(data[i+8:i+12])
				actualOffset := offset + int64(i)
				
				fmt.Printf("SITT block at offset %d:\n", actualOffset)
				fmt.Printf("  Length: %d bytes\n", length)
				fmt.Printf("  Type: 0x%x (%d)\n", btype, btype)
				
				// If this is a time tag data block (not config), analyze it
				if btype == 1 || btype == 3 {
					fmt.Printf("  -> This might be time tag data\n")
					analyzeTimeTagBlock(file, actualOffset, length)
				}
				
				// Skip to after this block
				if length > 0 && length < 1000000 { // Sanity check
					offset = actualOffset + int64(length)
					goto nextBlock
				}
			}
		}
		
		offset += int64(n - 12) // Overlap to catch blocks spanning chunks
		nextBlock:
	}
}

func analyzeTimeTagBlock(file *os.File, blockOffset int64, blockLength uint32) {
	// Read the block header (16 bytes)
	headerData := make([]byte, 16)
	_, err := file.ReadAt(headerData, blockOffset)
	if err != nil {
		return
	}

	dataStart := blockOffset + 16
	dataLength := int64(blockLength) - 16
	
	if dataLength <= 0 {
		return
	}

	// Read some data from the block (up to 200 bytes)
	readSize := int64(200)
	if dataLength < readSize {
		readSize = dataLength
	}
	
	blockData := make([]byte, readSize)
	n, err := file.ReadAt(blockData, dataStart)
	if err != nil && n == 0 {
		return
	}

	fmt.Printf("  Data starts at offset %d, length %d\n", dataStart, dataLength)
	fmt.Printf("  First %d bytes of data:\n", n)
	
	// Try to parse as 10-byte records (8 bytes timestamp + 2 bytes channel)
	channels := make(map[uint16]int)
	validRecords := 0
	
	for i := 0; i < n-9; i += 10 {
		timestamp := binary.LittleEndian.Uint64(blockData[i:i+8])
		channel := binary.LittleEndian.Uint16(blockData[i+8:i+10])
		
		// Check if this looks like valid time tag data
		if timestamp > 0 && timestamp < 0x7FFFFFFFFFFFFFFF && channel < 32768 {
			channels[channel]++
			validRecords++
			if validRecords <= 5 {
				fmt.Printf("    Record %d: timestamp=%d, channel=%d\n", validRecords, timestamp, channel)
			}
		}
	}
	
	if validRecords > 0 {
		fmt.Printf("  Found %d valid 10-byte records\n", validRecords)
		fmt.Printf("  Channels found: ")
		for channel := range channels {
			fmt.Printf("%d ", channel)
		}
		fmt.Println()
	} else {
		fmt.Printf("  No valid 10-byte records found\n")
		// Show hex dump
		fmt.Printf("  Hex dump of first 80 bytes:\n")
		for i := 0; i < 80 && i < n; i += 16 {
			fmt.Printf("    %04x: ", i)
			for j := 0; j < 16 && i+j < n && i+j < 80; j++ {
				fmt.Printf("%02x ", blockData[i+j])
			}
			fmt.Println()
		}
	}
}
