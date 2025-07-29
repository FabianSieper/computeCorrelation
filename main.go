package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"

	"github.com/FabianSieper/computeCorrelation/ttbin"
)

// Constants for the main application
const (
	PressEnterToExit = "\nPress Enter to exit..."
	DataDirectory    = "data"
	BatchSize        = 5000
)

// TimeDiff represents a time difference between two channels
type TimeDiff struct {
	Timestamp1 uint64
	Timestamp2 uint64
	Channel1   uint16
	Channel2   uint16
	Diff       int64
}

func main() {
	printHeader()

	// Create ttbin processor
	processor := ttbin.NewProcessor(DataDirectory)

	if err := processor.ValidateDataDirectory(); err != nil {
		handleError(err)
		return
	}

	files, err := processor.GetFiles()
	if err != nil {
		handleError(err)
		return
	}

	// Get user's choice of operation
	operation, err := getOperationChoice()
	if err != nil {
		handleError(err)
		return
	}

	switch operation {
	case 1: // Time difference analysis
		availableChannels, err := processor.ScanChannels(files)
		if err != nil {
			handleError(err)
			return
		}

		channel1, channel2, err := getUserChannelInput(processor, availableChannels)
		if err != nil {
			handleError(err)
			return
		}

		if err := processAndSaveResults(processor, files, channel1, channel2, availableChannels); err != nil {
			handleError(err)
			return
		}

		fmt.Printf("✓ SUCCESS: Results saved successfully")

	case 2: // CSV export
		// For CSV export, we only need a quick scan of one file to discover channels for display
		// Since we export ALL channels anyway, we don't need full channel counts
		availableChannels, err := processor.QuickScanChannels(files)
		if err != nil {
			handleError(err)
			return
		}

		if len(availableChannels) > 0 {
			fmt.Printf("Found channels in files - will export all data from all channels\n\n")
		}

		if err := exportToCSV(processor, files); err != nil {
			handleError(err)
			return
		}

		fmt.Printf("✓ SUCCESS: CSV export completed successfully")
	}

	fmt.Print(PressEnterToExit)
	fmt.Scanln()
}

func printHeader() {
	fmt.Println("=== Timetag Correlation Analyzer ===")
	fmt.Println("This program can analyze timetag data in two ways:")
	fmt.Println("1. Calculate time differences between consecutive events from two channels")
	fmt.Println("2. Export all time tag data to a human-readable CSV file")
	fmt.Println()
}

func getOperationChoice() (int, error) {
	fmt.Println("Choose operation:")
	fmt.Println("1. Time difference analysis")
	fmt.Println("2. Export to CSV")
	fmt.Print("Enter your choice (1 or 2): ")

	var choice int
	_, err := fmt.Scanln(&choice)
	if err != nil {
		return 0, fmt.Errorf("please enter a valid number")
	}

	if choice != 1 && choice != 2 {
		return 0, fmt.Errorf("please choose 1 or 2")
	}

	fmt.Println()
	return choice, nil
}

func getUserChannelInput(processor *ttbin.Processor, availableChannels map[uint16]int) (uint16, uint16, error) {
	fmt.Printf("Found %d .ttbin file(s) to analyze:\n", len(availableChannels))
	processor.DisplayChannels(availableChannels)

	// Use MaxUint16 as a special value to indicate "no exclusion" for the first channel
	channel1, err := getValidChannelInput("Enter first channel number: ", processor, availableChannels, ^uint16(0))
	if err != nil {
		return 0, 0, err
	}

	channel2, err := getValidChannelInput("Enter second channel number: ", processor, availableChannels, channel1)
	if err != nil {
		return 0, 0, err
	}

	fmt.Printf("\nAnalyzing channels %d (%d events) and %d (%d events)...\n\n",
		channel1, processor.GetChannelCount(channel1, availableChannels),
		channel2, processor.GetChannelCount(channel2, availableChannels))

	return channel1, channel2, nil
}

func getValidChannelInput(prompt string, processor *ttbin.Processor, availableChannels map[uint16]int, excludeChannel uint16) (uint16, error) {
	for {
		fmt.Print(prompt)
		var channel uint16
		_, err := fmt.Scanln(&channel)
		if err != nil {
			fmt.Println("Please enter a valid number.")
			clearInputBuffer()
			continue
		}
		if err := processor.ValidateChannel(channel, availableChannels); err != nil {
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

func processAndSaveResults(processor *ttbin.Processor, files []string, channel1, channel2 uint16, availableChannels map[uint16]int) error {
	outputFile := fmt.Sprintf("time_diff_ch%d_ch%d.txt", channel1, channel2)
	totalDiffs, err := processFilesStreaming(processor, files, channel1, channel2, outputFile)
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

// processFilesStreaming processes files with streaming architecture to prevent memory issues
func processFilesStreaming(processor *ttbin.Processor, files []string, channel1, channel2 uint16, outputFile string) (int, error) {
	outFile, err := createOutputFile(outputFile, channel1, channel2)
	if err != nil {
		return 0, err
	}
	defer outFile.Close()

	writer := bufio.NewWriterSize(outFile, 64*1024) // DefaultBufferSize
	defer writer.Flush()

	timeTagsChan := make(chan ttbin.TimeTag, 1000)

	// Process files using the ttbin processor
	go func() {
		err := processor.ProcessFiles(files, []uint16{channel1, channel2}, timeTagsChan)
		if err != nil {
			fmt.Printf("Warning: Error processing files: %v\n", err)
		}
	}()

	return streamCalculateAndWrite(timeTagsChan, channel1, channel2, writer)
}

func createOutputFile(outputFile string, channel1, channel2 uint16) (*os.File, error) {
	outFile, err := os.Create(outputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %v", err)
	}

	writer := bufio.NewWriterSize(outFile, 64*1024) // DefaultBufferSize
	fmt.Fprintf(writer, "# Time differences between consecutive events\n")
	fmt.Fprintf(writer, "# Generated by Timetag Correlation Analyzer\n")
	fmt.Fprintf(writer, "# Format: Timestamp1, Timestamp2, Channel1, Channel2, TimeDiff(ns)\n")
	fmt.Fprintf(writer, "# TimeDiff = Timestamp2 - Timestamp1\n")
	fmt.Fprintf(writer, "# Processing channels %d and %d\n\n", channel1, channel2)
	writer.Flush()

	return outFile, nil
}

// streamCalculateAndWrite processes time tags in streaming fashion
func streamCalculateAndWrite(timeTagsChan <-chan ttbin.TimeTag, channel1, channel2 uint16, writer *bufio.Writer) (int, error) {
	var batch []ttbin.TimeTag
	var lastEvent *ttbin.TimeTag
	totalDiffs := 0
	totalTags := 0

	fmt.Println("Streaming calculation and writing results...")
	fmt.Printf("Progress: [Time Tags Processed: %d, Time Differences: %d]\n", totalTags, totalDiffs)

	for tag := range timeTagsChan {
		batch = append(batch, tag)
		totalTags++

		// Show progress for every 1000 tags processed
		if totalTags%1000 == 0 {
			fmt.Printf("\rProgress: [Time Tags Processed: %d, Time Differences: %d]", totalTags, totalDiffs)
		}

		// Process batch when it reaches size limit
		if len(batch) >= BatchSize {
			count, newLastEvent := processBatch(batch, channel1, channel2, writer, lastEvent)
			totalDiffs += count
			lastEvent = newLastEvent

			// Clear batch to free memory
			batch = batch[:0]

			// Update progress display with both metrics
			fmt.Printf("\rProgress: [Time Tags Processed: %d, Time Differences: %d]", totalTags, totalDiffs)
			writer.Flush() // Ensure data is written regularly
		}
	}

	// Process remaining batch
	if len(batch) > 0 {
		count, _ := processBatch(batch, channel1, channel2, writer, lastEvent)
		totalDiffs += count
	}

	// Final progress update
	fmt.Printf("\rProgress: [Time Tags Processed: %d, Time Differences: %d]\n", totalTags, totalDiffs)

	if totalDiffs == 0 {
		return 0, fmt.Errorf("no events found for channels %d and %d", channel1, channel2)
	}

	writer.Flush()
	fmt.Printf("✓ Streaming processing complete: %d time tags processed, %d time differences calculated\n", totalTags, totalDiffs)
	return totalDiffs, nil
}

// processBatch processes a small batch of time tags and calculates differences
func processBatch(batch []ttbin.TimeTag, channel1, channel2 uint16, writer *bufio.Writer, lastEvent *ttbin.TimeTag) (int, *ttbin.TimeTag) {
	if len(batch) == 0 {
		return 0, lastEvent
	}

	// Sort this batch
	sort.Slice(batch, func(i, j int) bool {
		return batch[i].Timestamp < batch[j].Timestamp
	})

	count := 0
	var prevEvent *ttbin.TimeTag

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

// clearInputBuffer clears the input buffer to handle invalid input
func clearInputBuffer() {
	var dummy string
	fmt.Scanln(&dummy)
}

// exportToCSV exports all time tag data to a human-readable CSV file
func exportToCSV(processor *ttbin.Processor, files []string) error {
	fmt.Printf("Found %d .ttbin file(s) to export:\n", len(files))

	// Generate output filename with timestamp
	outputFile := "timetag_export.csv"
	fmt.Printf("\nExporting all time tag data to '%s'...\n\n", outputFile)

	// Create output file
	outFile, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer outFile.Close()

	writer := bufio.NewWriterSize(outFile, 64*1024)
	defer writer.Flush()

	// Write CSV header
	fmt.Fprintf(writer, "Timestamp,Channel\n")

	timeTagsChan := make(chan ttbin.TimeTag, 1000)

	// Process all files without channel filtering - extract ALL time tags
	go func() {
		err := processor.ProcessAllFiles(files, timeTagsChan)
		if err != nil {
			fmt.Printf("Warning: Error processing files: %v\n", err)
		}
	}()

	return streamToCSV(timeTagsChan, writer)
}

// streamToCSV processes time tags and writes them to CSV format
func streamToCSV(timeTagsChan <-chan ttbin.TimeTag, writer *bufio.Writer) error {
	var batch []ttbin.TimeTag
	totalTags := 0

	fmt.Println("Processing and writing to CSV...")
	fmt.Printf("Progress: [Time Tags Processed: %d]\n", totalTags)

	for tag := range timeTagsChan {
		batch = append(batch, tag)
		totalTags++

		// Show progress for every 1000 tags processed
		if totalTags%1000 == 0 {
			fmt.Printf("\rProgress: [Time Tags Processed: %d]", totalTags)
		}

		// Process batch when it reaches size limit
		if len(batch) >= BatchSize {
			writeCSVBatch(batch, writer)
			batch = batch[:0] // Clear batch to free memory
			writer.Flush()    // Ensure data is written regularly
		}
	}

	// Process remaining batch
	if len(batch) > 0 {
		writeCSVBatch(batch, writer)
	}

	// Final progress update
	fmt.Printf("\rProgress: [Time Tags Processed: %d]\n", totalTags)

	if totalTags == 0 {
		return fmt.Errorf("no time tags found in the files")
	}

	writer.Flush()
	fmt.Printf("✓ CSV export complete: %d time tags exported\n", totalTags)
	return nil
}

// writeCSVBatch writes a batch of time tags to the CSV file
func writeCSVBatch(batch []ttbin.TimeTag, writer *bufio.Writer) {
	// Sort batch by timestamp for chronological order
	sort.Slice(batch, func(i, j int) bool {
		return batch[i].Timestamp < batch[j].Timestamp
	})

	for _, tag := range batch {
		fmt.Fprintf(writer, "%d,%d\n", tag.Timestamp, tag.Channel)
	}
}
