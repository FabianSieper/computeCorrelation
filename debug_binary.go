package main

import (
	"encoding/binary"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run debug_binary.go <ttbin_file>")
		os.Exit(1)
	}

	filename := os.Args[1]
	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// Read first 1KB to examine file structure
	data := make([]byte, 1024)
	n, err := file.Read(data)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("File: %s\n", filename)
	fmt.Printf("Read %d bytes\n\n", n)

	// Print hex dump of first 256 bytes
	fmt.Println("Hex dump of first 256 bytes:")
	for i := 0; i < 256 && i < n; i += 16 {
		fmt.Printf("%04x: ", i)
		for j := 0; j < 16 && i+j < n && i+j < 256; j++ {
			fmt.Printf("%02x ", data[i+j])
		}
		fmt.Println()
	}

	fmt.Println("\nLooking for potential channel values (as uint16 little endian):")
	channels := make(map[uint16]int)
	
	// Check every 2-byte position as potential channel
	for i := 0; i < n-1; i += 2 {
		channel := binary.LittleEndian.Uint16(data[i:i+2])
		if channel > 0 && channel < 32768 { // Valid channel range
			channels[channel]++
		}
	}

	fmt.Println("Most frequent 2-byte values (potential channels):")
	count := 0
	for channel, freq := range channels {
		if freq > 1 && count < 20 { // Show top 20 most frequent
			fmt.Printf("  %d: appears %d times\n", channel, freq)
			count++
		}
	}

	fmt.Println("\nLooking for SITT blocks:")
	// Search for SITT magic numbers
	sittFound := false
	for i := 0; i < n-12; i++ {
		// Check for potential SITT block header
		magic := binary.LittleEndian.Uint32(data[i:i+4])
		if magic == 0x54544953 { // "SITT" in little endian
			fmt.Printf("Found SITT magic at offset %d\n", i)
			length := binary.LittleEndian.Uint32(data[i+4:i+8])
			btype := binary.LittleEndian.Uint32(data[i+8:i+12])
			fmt.Printf("  Length: %d, Type: 0x%x\n", length, btype)
			sittFound = true
		}
	}
	
	if !sittFound {
		fmt.Println("No SITT magic found in first 1KB")
	}
}
