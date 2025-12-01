package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/dotabuff/manta"
)

func main() {
	f, err := os.Open("replay.dem")
	if err != nil {
		log.Fatalf("unable to open file: %s", err)
	}
	defer f.Close()

	p, err := manta.NewStreamParser(f)
	if err != nil {
		log.Fatalf("unable to create parser: %s", err)
	}

	// Track if we've already printed
	hasShown := false

	p.OnEntity(func(e *manta.Entity, op manta.EntityOp) error {
		if hasShown {
			return nil
		}

		className := e.GetClassName()

		// Only process hero entities
		if !strings.Contains(className, "_Hero_") {
			return nil
		}

		allFields := e.Map()

		// Check if this entity has m_flStartSequenceCycle
		if _, exists := allFields["m_flStartSequenceCycle"]; exists {
			hasShown = true

			fmt.Printf("\n=== Found hero with m_flStartSequenceCycle ===\n")
			fmt.Printf("Class: %s\n", className)
			fmt.Printf("Tick: %d\n", p.Tick)
			fmt.Printf("\nAll fields available:\n")

			// Print all fields sorted for easier reading
			for key, value := range allFields {
				fmt.Printf("  %s = %v (type: %T)\n", key, value, value)
			}

			fmt.Println("\n=== Looking for time-related fields ===")
			for key, value := range allFields {
				lowerKey := strings.ToLower(key)
				if strings.Contains(lowerKey, "time") || strings.Contains(lowerKey, "duration") ||
				   strings.Contains(lowerKey, "elapsed") || strings.Contains(lowerKey, "tick") {
					fmt.Printf("  %s = %v (type: %T)\n", key, value, value)
				}
			}
		}

		return nil
	})

	p.Start()

	if !hasShown {
		log.Printf("No hero entities with m_flStartSequenceCycle found\n")
	}
}
