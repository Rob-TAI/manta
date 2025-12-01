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

	foundHero := false

	p.OnEntity(func(e *manta.Entity, op manta.EntityOp) error {
		if foundHero || p.Tick > 10000 {
			return fmt.Errorf("done")
		}

		className := e.GetClassName()
		if !strings.HasPrefix(className, "CDOTA_Unit_Hero_") {
			return nil
		}

		// Found a hero, let's inspect the CBodyComponent fields
		fmt.Printf("Hero: %s at tick %d\n", className, p.Tick)
		fmt.Println("\nAll fields in entity:")

		allFields := e.Map()
		for key, value := range allFields {
			if strings.Contains(key, "CBodyComponent") {
				fmt.Printf("  %s = %v (type: %T)\n", key, value, value)
			}
		}

		foundHero = true
		return fmt.Errorf("done")
	})

	if err := p.Start(); err != nil && !strings.Contains(err.Error(), "done") {
		log.Fatalf("parser error: %s", err)
	}
}
