package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/dotabuff/manta"
)

type Position struct {
	CellX uint64 `json:"CBodyComponent.m_cellX"`
	CellY uint64 `json:"CBodyComponent.m_cellY"`
	CellZ uint64 `json:"CBodyComponent.m_cellZ"`
}

type VisionRange struct {
	Day   int32 `json:"day"`
	Night int32 `json:"night"`
}

type EntityData struct {
	Position
	Team uint64 `json:"m_iTeamNum,omitempty"`
}

type TickData struct {
	Team2 map[string]EntityData `json:"team2"`
	Team3 map[string]EntityData `json:"team3"`
}

type Output struct {
	VisionRanges map[string]VisionRange `json:"vision_ranges"`
	Ticks        map[uint32]TickData    `json:"ticks"`
}

func main() {
	// Command-line flags
	maxTick := flag.Int("maxtick", 1000, "Maximum tick to parse (0 = unlimited)")
	flag.Parse()

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

	// Track vision ranges (static data from first occurrence)
	visionRanges := make(map[string]VisionRange)

	// Track ticks data
	ticks := make(map[uint32]TickData)

	entityCount := 0
	lastTick := uint32(0)

	p.OnEntity(func(e *manta.Entity, op manta.EntityOp) error {
		entityCount++
		if entityCount%10000 == 0 {
			fmt.Printf("\rProcessed %d entities at tick %d...", entityCount, p.Tick)
		}

		lastTick = p.Tick

		// Check if we've reached the tick limit
		if *maxTick > 0 && p.Tick > uint32(*maxTick) {
			return fmt.Errorf("reached max tick %d", *maxTick)
		}

		className := e.GetClassName()

		// Check if this is one of our target classes
		isHero := strings.HasPrefix(className, "CDOTA_Unit_Hero_")
		isTower := className == "CDOTA_BaseNPC_Tower"
		isFort := className == "CDOTA_BaseNPC_Fort"

		if !isHero && !isTower && !isFort {
			return nil
		}

		// Extract required fields
		cellX, _ := e.GetUint64("CBodyComponent.m_cellX")
		cellY, _ := e.GetUint64("CBodyComponent.m_cellY")
		cellZ, _ := e.GetUint64("CBodyComponent.m_cellZ")
		teamNum, _ := e.GetUint64("m_iTeamNum")

		// Store vision ranges for heroes (static data)
		if isHero {
			if _, exists := visionRanges[className]; !exists {
				dayVision, _ := e.GetInt32("m_iDayTimeVisionRange")
				nightVision, _ := e.GetInt32("m_iNightTimeVisionRange")
				visionRanges[className] = VisionRange{
					Day:   dayVision,
					Night: nightVision,
				}
			}
		}

		// Initialize tick data if needed
		if _, exists := ticks[p.Tick]; !exists {
			ticks[p.Tick] = TickData{
				Team2: make(map[string]EntityData),
				Team3: make(map[string]EntityData),
			}
		}

		tickData := ticks[p.Tick]

		entityData := EntityData{
			Position: Position{
				CellX: cellX,
				CellY: cellY,
				CellZ: cellZ,
			},
			Team: teamNum,
		}

		// Store in appropriate team bucket
		entityKey := fmt.Sprintf("%s_%d", className, e.GetIndex())
		if teamNum == 2 {
			tickData.Team2[entityKey] = entityData
		} else if teamNum == 3 {
			tickData.Team3[entityKey] = entityData
		}

		ticks[p.Tick] = tickData

		return nil
	})

	// Start parsing
	if *maxTick > 0 {
		fmt.Printf("Parsing up to tick %d...\n", *maxTick)
	}

	if err := p.Start(); err != nil {
		// Check if it's our max tick error
		if !strings.Contains(err.Error(), "reached max tick") {
			log.Fatalf("parser error: %s", err)
		}
	}

	fmt.Printf("\rProcessed %d entities up to tick %d\n", entityCount, lastTick)

	// Prepare output
	output := Output{
		VisionRanges: visionRanges,
		Ticks:        ticks,
	}

	// Write JSON output
	jsonFile, err := os.Create("vision.json")
	if err != nil {
		log.Fatalf("unable to create JSON output file: %s", err)
	}
	defer jsonFile.Close()

	encoder := json.NewEncoder(jsonFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		log.Fatalf("unable to encode JSON: %s", err)
	}

	// Write human-readable summary
	summaryFile, err := os.Create("vision.txt")
	if err != nil {
		log.Fatalf("unable to create summary file: %s", err)
	}
	defer summaryFile.Close()

	fmt.Fprintf(summaryFile, "DOTA 2 VISION TRACKING - SUMMARY\n")
	fmt.Fprintf(summaryFile, "=================================\n\n")

	// Vision ranges summary
	fmt.Fprintf(summaryFile, "VISION RANGES\n")
	fmt.Fprintf(summaryFile, "-------------\n")

	var heroNames []string
	for hero := range visionRanges {
		heroNames = append(heroNames, hero)
	}
	sort.Strings(heroNames)

	for _, hero := range heroNames {
		vr := visionRanges[hero]
		fmt.Fprintf(summaryFile, "%s: Day=%d, Night=%d\n", hero, vr.Day, vr.Night)
	}

	// Ticks summary
	fmt.Fprintf(summaryFile, "\n\nTICKS DATA\n")
	fmt.Fprintf(summaryFile, "----------\n")
	fmt.Fprintf(summaryFile, "Total ticks captured: %d\n", len(ticks))

	var tickNumbers []uint32
	for tick := range ticks {
		tickNumbers = append(tickNumbers, tick)
	}
	sort.Slice(tickNumbers, func(i, j int) bool { return tickNumbers[i] < tickNumbers[j] })

	if len(tickNumbers) > 0 {
		fmt.Fprintf(summaryFile, "First tick: %d (%.2fs)\n", tickNumbers[0], float32(tickNumbers[0])/30.0)
		fmt.Fprintf(summaryFile, "Last tick: %d (%.2fs)\n", tickNumbers[len(tickNumbers)-1], float32(tickNumbers[len(tickNumbers)-1])/30.0)
	}

	// Sample tick data
	if len(tickNumbers) > 0 {
		sampleTick := tickNumbers[0]
		tickData := ticks[sampleTick]
		fmt.Fprintf(summaryFile, "\nSample data from tick %d:\n", sampleTick)
		fmt.Fprintf(summaryFile, "  Team 2 entities: %d\n", len(tickData.Team2))
		fmt.Fprintf(summaryFile, "  Team 3 entities: %d\n", len(tickData.Team3))
	}

	fmt.Fprintf(summaryFile, "\n\nFull data available in vision.json\n")

	log.Printf("Output written to vision.json and vision.txt")
	log.Printf("Captured %d hero vision ranges and %d ticks of data\n", len(visionRanges), len(ticks))
}
