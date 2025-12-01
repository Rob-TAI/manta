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

	// Track unique time-related fields from heroes
	timeFields := make(map[string]interface{})
	heroCount := 0
	maxHeroes := 5 // Just check first few heroes

	p.OnEntity(func(e *manta.Entity, op manta.EntityOp) error {
		className := e.GetClassName()

		// Only check heroes
		if !strings.HasPrefix(className, "CDOTA_Unit_Hero_") {
			return nil
		}

		heroCount++
		if heroCount > maxHeroes {
			return fmt.Errorf("checked enough heroes")
		}

		allFields := e.Map()

		fmt.Printf("\n=== Hero: %s (Tick: %d) ===\n", className, p.Tick)
		fmt.Printf("Tick: %d\n", p.Tick)
		fmt.Printf("Time (seconds): %.2f\n", float64(p.Tick)/30.0)
		fmt.Printf("Time (minutes): %.2f\n", float64(p.Tick)/1800.0)

		// Look for any time-related fields
		fmt.Println("\nAll fields containing 'time', 'Time', or 'clock':")
		for key, value := range allFields {
			lowerKey := strings.ToLower(key)
			if strings.Contains(lowerKey, "time") || strings.Contains(lowerKey, "clock") {
				fmt.Printf("  %s = %v (type: %T)\n", key, value, value)
				timeFields[key] = value
			}
		}

		return nil
	})

	if err := p.Start(); err != nil {
		if !strings.Contains(err.Error(), "checked enough heroes") {
			log.Fatalf("parser error: %s", err)
		}
	}

	fmt.Println("\n\n=== SUMMARY ===")
	fmt.Println("Unique time-related fields found across heroes:")
	for field := range timeFields {
		fmt.Printf("  - %s\n", field)
	}

	fmt.Println("\n=== RECOMMENDATION ===")
	fmt.Println("The most reliable way to get time is to calculate from tick:")
	fmt.Println("  - Game time (seconds) = tick / 30.0")
	fmt.Println("  - Game time (minutes) = tick / 1800.0")
	fmt.Println("\nThis gives you the actual in-game time at each tick.")
}
