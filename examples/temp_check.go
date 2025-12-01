package main

import (
	"fmt"
	"log"
	"os"

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

	// Look for fields containing "Body" or "CBody"
	seenFields := make(map[string]bool)

	p.OnEntity(func(e *manta.Entity, op manta.EntityOp) error {
		allFields := e.Map()
		
		for key := range allFields {
			if !seenFields[key] {
				seenFields[key] = true
			}
		}
		
		return nil
	})

	p.Start()

	fmt.Println("\nAll unique field names:")
	for field := range seenFields {
		fmt.Println(field)
	}
}
