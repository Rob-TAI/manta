package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/dotabuff/manta"
)

func main() {
	// Create output file
	outFile, err := os.Create("spectator_data.txt")
	if err != nil {
		log.Fatalf("unable to create output file: %s", err)
	}
	defer outFile.Close()

	// Create buffered writer for better performance
	writer := bufio.NewWriter(outFile)
	defer writer.Flush()

	// Create a new parser instance from a file
	f, err := os.Open("replay.dem")
	if err != nil {
		log.Fatalf("unable to open file: %s", err)
	}
	defer f.Close()

	p, err := manta.NewStreamParser(f)
	if err != nil {
		log.Fatalf("unable to create parser: %s", err)
	}

	// Track what we've seen
	seenClasses := make(map[string]bool)
	entityCount := 0
	matchCount := 0
	lastProgressUpdate := time.Now()
	lastFlush := time.Now()

	p.OnEntity(func(e *manta.Entity, op manta.EntityOp) error {
		entityCount++

		// Update progress every 100ms
		if time.Since(lastProgressUpdate) > 100*time.Millisecond {
			fmt.Printf("\rProcessing... Entities: %d, Spectator matches: %d", entityCount, matchCount)
			lastProgressUpdate = time.Now()
		}

		className := e.GetClassName()
		allFields := e.Map()

		// Check if class name contains "spectator" (case insensitive)
		classNameLower := strings.ToLower(className)
		hasSpectatorInClass := strings.Contains(classNameLower, "spectator")

		// Check if any field name contains "spectator"
		hasSpectatorInField := false
		spectatorFields := []string{}
		for key := range allFields {
			if strings.Contains(strings.ToLower(key), "spectator") {
				hasSpectatorInField = true
				spectatorFields = append(spectatorFields, key)
			}
		}

		// If either condition is true, print details
		if hasSpectatorInClass || hasSpectatorInField {
			// Only print full details once per class
			if !seenClasses[className] {
				seenClasses[className] = true
				matchCount++

				fmt.Fprintf(writer, "\n=== %s (Tick: %d) ===\n", className, p.Tick)

				if hasSpectatorInClass {
					fmt.Fprintf(writer, "  [Class name contains 'spectator']\n")
				}

				if hasSpectatorInField {
					fmt.Fprintf(writer, "  [Has spectator-related fields: %v]\n", spectatorFields)
				}

				fmt.Fprintf(writer, "\n  All fields:\n")
				for key, value := range allFields {
					fmt.Fprintf(writer, "    %s = %v (type: %T)\n", key, value, value)
				}
				fmt.Fprintf(writer, "\n")
			}
		}

		// Flush to disk every second
		if time.Since(lastFlush) > 1*time.Second {
			writer.Flush()
			lastFlush = time.Now()
		}

		return nil
	})

	// Start parsing the replay!
	p.Start()

	// Final flush
	writer.Flush()

	fmt.Printf("\rProcessing... Entities: %d, Spectator matches: %d\n", entityCount, matchCount)

	log.Printf("Parse Complete! Found %d unique classes/fields with 'spectator' from %d entities\n",
		matchCount, entityCount)
	log.Printf("Results written to 'spectator_data.txt'\n")
}
