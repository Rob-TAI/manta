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
	Team        uint64
	VisionRange int32 // Day vision range for calculations
	ClassName   string
}

type TickData struct {
	Team2 map[string]EntityData
	Team3 map[string]EntityData
}

type HeroVisibility struct {
	Visible bool     `json:"visible"`
	SeenBy  []string `json:"seen_by"`
}

type TeamVisibility map[string]HeroVisibility

type TickVisibility struct {
	Time  float64        `json:"time"`  // Game time in seconds
	Team2 TeamVisibility `json:"team2"`
	Team3 TeamVisibility `json:"team3"`
}

type VisibilityOutput struct {
	Visible map[uint32]TickVisibility `json:"visible"`
}

func main() {
	// Command-line flags
	maxTick := flag.Int("maxtick", 0, "Maximum tick to parse (0 = unlimited)")
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
	lastReportedMinute := uint32(0)
	const ticksPerMinute = uint32(1800) // 30 ticks/sec * 60 sec = 1800 ticks/minute

	p.OnEntity(func(e *manta.Entity, op manta.EntityOp) error {
		entityCount++
		lastTick = p.Tick

		// Report progress every minute of game time
		currentMinute := p.Tick / ticksPerMinute
		if currentMinute > lastReportedMinute {
			gameTime := float64(p.Tick) / 30.0
			fmt.Printf("Progress: %d minutes (%.1f seconds) - Processed %d entities, captured %d ticks\n",
				currentMinute, gameTime, entityCount, len(ticks))
			lastReportedMinute = currentMinute
		}

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

		// Get vision range for this entity
		dayVision := int32(0)
		if isHero {
			// Store vision ranges for heroes (static data)
			if _, exists := visionRanges[className]; !exists {
				dayVision, _ = e.GetInt32("m_iDayTimeVisionRange")
				nightVision, _ := e.GetInt32("m_iNightTimeVisionRange")
				visionRanges[className] = VisionRange{
					Day:   dayVision,
					Night: nightVision,
				}
			} else {
				dayVision = visionRanges[className].Day
			}
		} else if isTower || isFort {
			// Get vision range for towers/forts
			dayVision, _ = e.GetInt32("m_iDayTimeVisionRange")
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
			Team:        teamNum,
			VisionRange: dayVision,
			ClassName:   className,
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
		fmt.Printf("Parsing up to tick %d (%.1f minutes)...\n", *maxTick, float64(*maxTick)/1800.0)
	} else {
		fmt.Printf("Parsing entire replay (unlimited)...\n")
	}

	if err := p.Start(); err != nil {
		// Check if it's our max tick error
		if !strings.Contains(err.Error(), "reached max tick") {
			log.Fatalf("parser error: %s", err)
		}
	}

	finalGameTime := float64(lastTick) / 30.0
	fmt.Printf("\n✓ Parsing complete: %d entities processed up to tick %d (%.1f minutes / %.1f seconds)\n",
		entityCount, lastTick, finalGameTime/60.0, finalGameTime)

	// Calculate visibility for each tick
	fmt.Printf("Calculating visibility for %d ticks...\n", len(ticks))
	visibility := calculateVisibility(ticks)

	// Write visibility JSON output
	visFile, err := os.Create("visibility.json")
	if err != nil {
		log.Fatalf("unable to create visibility JSON file: %s", err)
	}
	defer visFile.Close()

	encoder := json.NewEncoder(visFile)
	encoder.SetIndent("", "  ")

	output := VisibilityOutput{
		Visible: visibility,
	}

	if err := encoder.Encode(output); err != nil {
		log.Fatalf("unable to encode visibility JSON: %s", err)
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

	// Visibility summary
	fmt.Fprintf(summaryFile, "\n\nVISIBILITY ANALYSIS\n")
	fmt.Fprintf(summaryFile, "-------------------\n")

	visibleCount := 0
	totalHeroChecks := 0
	for _, tickVis := range visibility {
		for _, heroVis := range tickVis.Team2 {
			totalHeroChecks++
			if heroVis.Visible {
				visibleCount++
			}
		}
		for _, heroVis := range tickVis.Team3 {
			totalHeroChecks++
			if heroVis.Visible {
				visibleCount++
			}
		}
	}

	if totalHeroChecks > 0 {
		visiblePct := float64(visibleCount) / float64(totalHeroChecks) * 100
		fmt.Fprintf(summaryFile, "Total hero visibility checks: %d\n", totalHeroChecks)
		fmt.Fprintf(summaryFile, "Heroes visible: %d (%.1f%%)\n", visibleCount, visiblePct)
		fmt.Fprintf(summaryFile, "Heroes not visible: %d (%.1f%%)\n", totalHeroChecks-visibleCount, 100-visiblePct)
	}

	fmt.Fprintf(summaryFile, "\n\nFull visibility data available in visibility.json\n")

	log.Printf("Output written to visibility.json and vision.txt")
	log.Printf("Captured %d hero vision ranges, %d ticks, analyzed %d hero positions\n",
		len(visionRanges), len(ticks), totalHeroChecks)
}

// calculateVisibility computes which heroes are visible to the opposing team each tick
func calculateVisibility(ticks map[uint32]TickData) map[uint32]TickVisibility {
	visibility := make(map[uint32]TickVisibility)

	for tick, tickData := range ticks {
		tickVis := TickVisibility{
			Time:  float64(tick) / 30.0, // Convert tick to seconds
			Team2: make(TeamVisibility),
			Team3: make(TeamVisibility),
		}

		// Check Team 2 heroes against Team 3 entities
		for heroKey, hero := range tickData.Team2 {
			if !strings.HasPrefix(hero.ClassName, "CDOTA_Unit_Hero_") || hero.Team != 2 {
				continue
			}

			seenBy := []string{}
			for entityKey, entity := range tickData.Team3 {
				// Only check entities from opposing team with vision range
				if entity.Team != 3 || entity.VisionRange <= 0 {
					continue
				}
				if isVisible(hero, entity) {
					seenBy = append(seenBy, entityKey)
				}
			}

			tickVis.Team2[heroKey] = HeroVisibility{
				Visible: len(seenBy) > 0,
				SeenBy:  seenBy,
			}
		}

		// Check Team 3 heroes against Team 2 entities
		for heroKey, hero := range tickData.Team3 {
			if !strings.HasPrefix(hero.ClassName, "CDOTA_Unit_Hero_") || hero.Team != 3 {
				continue
			}

			seenBy := []string{}
			for entityKey, entity := range tickData.Team2 {
				// Only check entities from opposing team with vision range
				if entity.Team != 2 || entity.VisionRange <= 0 {
					continue
				}
				if isVisible(hero, entity) {
					seenBy = append(seenBy, entityKey)
				}
			}

			tickVis.Team3[heroKey] = HeroVisibility{
				Visible: len(seenBy) > 0,
				SeenBy:  seenBy,
			}
		}

		visibility[tick] = tickVis
	}

	return visibility
}

// isVisible checks if a hero is within the vision range of an entity
func isVisible(hero EntityData, observer EntityData) bool {
	// Calculate 2D distance (ignoring Z for simplicity, cells are in map coordinates)
	dx := float64(hero.CellX) - float64(observer.CellX)
	dy := float64(hero.CellY) - float64(observer.CellY)

	// Using squared distance to avoid sqrt for performance
	distSquared := dx*dx + dy*dy
	visionRange := float64(observer.VisionRange)
	visionRangeSquared := visionRange * visionRange

	return distSquared <= visionRangeSquared
}
