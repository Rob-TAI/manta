package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/dotabuff/manta"
)

func main() {
	// Create output file
	outFile, err := os.Create("all_fields.txt")
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

	// Track seen fields
	seen_fields := make(map[string]bool)

	// Progress tracking
	entityCount := 0
	lastProgressUpdate := time.Now()
	lastFlush := time.Now()

	p.OnEntity(func(e *manta.Entity, op manta.EntityOp) error {
		entityCount++

		// Update progress every 100ms
		if time.Since(lastProgressUpdate) > 100*time.Millisecond {
			fmt.Printf("\rProcessing... Entities: %d, Unique fields: %d", entityCount, len(seen_fields))
			lastProgressUpdate = time.Now()
		}

		className := e.GetClassName()
		allFields := e.Map()

		// Write each new field immediately to file
		for key, value := range allFields {
			if !seen_fields[key] {
				seen_fields[key] = true
				fmt.Fprintf(writer, "[%s] %s = %v (type: %T)\n",
					className, key, value, value)
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

	fmt.Printf("\rProcessing... Entities: %d, Unique fields: %d\n", entityCount, len(seen_fields))

	log.Printf("Parse Complete! Wrote %d unique fields from %d entities to 'all_fields.txt'\n",
		len(seen_fields), entityCount)
}
