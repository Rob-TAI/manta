package main

import (
	"fmt"
	"log"
	"os"

	"github.com/dotabuff/manta"
)

type ViewerInstance struct {
	Tick      uint32
	Op        manta.EntityOp
	AllFields map[string]interface{}
}

func main() {
	// Specify which class to inspect (change this to inspect different classes)
	targetClass := "CDOTAFogOfWarTempViewers"

	// Create a new parser instance from a file. Alternatively see NewParser([]byte)
	f, err := os.Open("replay.dem")
	if err != nil {
		log.Fatalf("unable to open file: %s", err)
	}
	defer f.Close()

	p, err := manta.NewStreamParser(f)
	if err != nil {
		log.Fatalf("unable to create parser: %s", err)
	}

	// Track all instances
	var instances []ViewerInstance
	entityCount := 0

	p.OnEntity(func(e *manta.Entity, op manta.EntityOp) error {
		entityCount++
		if entityCount%10000 == 0 {
			fmt.Printf("\rProcessed %d entities, found %d viewers...", entityCount, len(instances))
		}

		className := e.GetClassName()

		// If this is the target class
		if className == targetClass {
			allFields := e.Map()

			instance := ViewerInstance{
				Tick:      p.Tick,
				Op:        op,
				AllFields: allFields,
			}

			instances = append(instances, instance)
		}

		return nil
	})

	// Start parsing the replay!
	p.Start()

	fmt.Printf("\rProcessed %d entities, found %d viewers\n", entityCount, len(instances))

	// Write results to file
	outFile, err := os.Create("example.txt")
	if err != nil {
		log.Fatalf("unable to create output file: %s", err)
	}
	defer outFile.Close()

	fmt.Fprintf(outFile, "CDOTAFogOfWarTempViewers Instances - All Fields\n")
	fmt.Fprintf(outFile, "================================================\n\n")
	fmt.Fprintf(outFile, "Total instances found: %d\n\n", len(instances))

	for i, inst := range instances {
		fmt.Fprintf(outFile, "[%d] Tick: %d, Time: %.2fs, Op: %v\n", i+1, inst.Tick, float32(inst.Tick)/30.0, inst.Op)
		fmt.Fprintf(outFile, "    All Fields:\n")

		// Print all fields
		for key, value := range inst.AllFields {
			fmt.Fprintf(outFile, "      %s = %v (type: %T)\n", key, value, value)
		}

		fmt.Fprintf(outFile, "\n")
	}

	fmt.Fprintf(outFile, "Parse Complete!\n")

	log.Printf("Output written to 'example.txt' - found %d instances\n", len(instances))
}
