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

	count := 0
	maxSamples := 10

	p.OnEntity(func(e *manta.Entity, op manta.EntityOp) error {
		if count >= maxSamples || p.Tick > 10000 {
			return fmt.Errorf("done")
		}

		className := e.GetClassName()
		if !strings.HasPrefix(className, "CDOTA_Unit_Hero_") {
			return nil
		}

		// Read cell and vec coordinates
		cellXRaw, _ := e.GetUint64("CBodyComponent.m_cellX")
		cellYRaw, _ := e.GetUint64("CBodyComponent.m_cellY")
		cellX := int32(cellXRaw)
		cellY := int32(cellYRaw)

		vecX, _ := e.GetFloat32("CBodyComponent.m_vecX")
		vecY, _ := e.GetFloat32("CBodyComponent.m_vecY")

		// Calculate true position
		const cellSize float64 = 250.0
		posX := float64(cellX)*cellSize + float64(vecX)
		posY := float64(cellY)*cellSize + float64(vecY)

		fmt.Printf("%s: cell=(%d,%d) vec=(%.1f,%.1f) → position=(%.1f,%.1f)\n",
			className, cellX, cellY, vecX, vecY, posX, posY)

		count++
		return nil
	})

	if err := p.Start(); err != nil && !strings.Contains(err.Error(), "done") {
		log.Fatalf("parser error: %s", err)
	}

	fmt.Println("\nExpected range: 7500 to 25000 for playable map area")
}
