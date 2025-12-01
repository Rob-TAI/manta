package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/dotabuff/manta"
)

type SequenceData struct {
	Tick        uint32  `json:"tick"`
	Time        float32 `json:"time"`
	Value       float32 `json:"value"`
	RespawnTime float32 `json:"respawn_time"`
}

func main() {
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

	// Map to store hero data: hero_name -> []SequenceData
	heroData := make(map[string][]SequenceData)

	// Progress tracking
	entityCount := 0
	occurrenceCount := 0
	lastProgressUpdate := time.Now()

	p.OnEntity(func(e *manta.Entity, op manta.EntityOp) error {
		entityCount++

		// Update progress every 100ms
		if time.Since(lastProgressUpdate) > 100*time.Millisecond {
			fmt.Printf("\rProcessing... Entities: %d, Hero m_flStartSequenceCycle occurrences: %d", entityCount, occurrenceCount)
			lastProgressUpdate = time.Now()
		}

		className := e.GetClassName()

		// Only process hero entities
		if !strings.Contains(className, "_Hero_") {
			return nil
		}

		allFields := e.Map()

		// Check if this entity has m_flStartSequenceCycle
		if value, exists := allFields["m_flStartSequenceCycle"]; exists {
			if floatVal, ok := value.(float32); ok {
				occurrenceCount++

				// Calculate game time from tick (assuming 30 ticks per second)
				gameTime := float32(p.Tick) / 30.0

				// Get respawn time if available
				respawnTime := float32(0)
				if rt, exists := allFields["m_flRespawnTime"]; exists {
					if rtFloat, ok := rt.(float32); ok {
						respawnTime = rtFloat
					}
				}

				// Add to hero data
				heroData[className] = append(heroData[className], SequenceData{
					Tick:        p.Tick,
					Time:        gameTime,
					Value:       floatVal,
					RespawnTime: respawnTime,
				})
			}
		}

		return nil
	})

	// Start parsing the replay!
	p.Start()

	fmt.Printf("\rProcessing... Entities: %d, Hero m_flStartSequenceCycle occurrences: %d\n", entityCount, occurrenceCount)

	// Write JSON output
	outFile, err := os.Create("hero_sequences.json")
	if err != nil {
		log.Fatalf("unable to create output file: %s", err)
	}
	defer outFile.Close()

	encoder := json.NewEncoder(outFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(heroData); err != nil {
		log.Fatalf("unable to encode JSON: %s", err)
	}

	log.Printf("Parse Complete! Found %d occurrences of m_flStartSequenceCycle from %d hero entities\n",
		occurrenceCount, len(heroData))
	log.Printf("Results written to 'hero_sequences.json'\n")
}
