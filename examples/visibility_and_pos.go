package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/dotabuff/manta"
)

type Position struct {
	CellX int32   `json:"m_cellX"`
	CellY int32   `json:"m_cellY"`
	CellZ int32   `json:"m_cellZ"`
	VecX  float32 `json:"m_vecX"`
	VecY  float32 `json:"m_vecY"`
	VecZ  float32 `json:"m_vecZ"`
}

type EntityData struct {
	Position
	Team        uint64
	VisionRange int32
	ClassName   string
}

type HeroData struct {
	Position
	Team    uint64  `json:"team"`
	Time    float64 `json:"time"`
	Visible bool    `json:"visible"`
	SeenBy  []string `json:"seen_by"`
}

type TickData struct {
	Team2 map[string]EntityData
	Team3 map[string]EntityData
}

type TickOutput struct {
	Time   float64              `json:"time"`
	Heroes map[string]HeroData  `json:"heroes"`
}

type CombinedOutput struct {
	Ticks map[uint32]TickOutput `json:"ticks"`
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
	visionRanges := make(map[string]int32)

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
		// Cell fields are stored as uint64 but may represent signed values
		cellXRaw, _ := e.GetUint64("CBodyComponent.m_cellX")
		cellYRaw, _ := e.GetUint64("CBodyComponent.m_cellY")
		cellZRaw, _ := e.GetUint64("CBodyComponent.m_cellZ")
		cellX := int32(cellXRaw)
		cellY := int32(cellYRaw)
		cellZ := int32(cellZRaw)

		// Get within-cell offset vectors
		vecX, _ := e.GetFloat32("CBodyComponent.m_vecX")
		vecY, _ := e.GetFloat32("CBodyComponent.m_vecY")
		vecZ, _ := e.GetFloat32("CBodyComponent.m_vecZ")

		teamNum, _ := e.GetUint64("m_iTeamNum")

		// Get vision range for this entity
		dayVision := int32(0)
		if isHero {
			// Store vision ranges for heroes (static data)
			if _, exists := visionRanges[className]; !exists {
				dayVision, _ = e.GetInt32("m_iDayTimeVisionRange")
				visionRanges[className] = dayVision
			} else {
				dayVision = visionRanges[className]
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
				VecX:  vecX,
				VecY:  vecY,
				VecZ:  vecZ,
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
	fmt.Printf("\n[OK] Parsing complete: %d entities processed up to tick %d (%.1f minutes / %.1f seconds)\n",
		entityCount, lastTick, finalGameTime/60.0, finalGameTime)

	// Calculate visibility and build output
	fmt.Printf("Calculating visibility and building output for %d ticks...\n", len(ticks))
	output := buildCombinedOutput(ticks)

	// Write JSON output
	outFile, err := os.Create("visibility_and_pos.json")
	if err != nil {
		log.Fatalf("unable to create output JSON file: %s", err)
	}
	defer outFile.Close()

	encoder := json.NewEncoder(outFile)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(output); err != nil {
		log.Fatalf("unable to encode JSON: %s", err)
	}

	// Calculate statistics
	totalHeroes := 0
	totalVisible := 0
	totalInvisible := 0

	for _, tickOut := range output.Ticks {
		for _, hero := range tickOut.Heroes {
			totalHeroes++
			if hero.Visible {
				totalVisible++
			} else {
				totalInvisible++
			}
		}
	}

	fmt.Printf("\n[OK] Data written to visibility_and_pos.json\n")
	fmt.Printf("Statistics:\n")
	fmt.Printf("  - Total ticks captured: %d\n", len(output.Ticks))
	fmt.Printf("  - Total hero snapshots: %d\n", totalHeroes)
	fmt.Printf("  - Heroes visible: %d (%.1f%%)\n", totalVisible, float64(totalVisible)/float64(totalHeroes)*100)
	fmt.Printf("  - Heroes invisible: %d (%.1f%%)\n", totalInvisible, float64(totalInvisible)/float64(totalHeroes)*100)
}

// buildCombinedOutput creates the combined output structure with visibility calculation
func buildCombinedOutput(ticks map[uint32]TickData) CombinedOutput {
	output := CombinedOutput{
		Ticks: make(map[uint32]TickOutput),
	}

	for tick, tickData := range ticks {
		tickOut := TickOutput{
			Time:   float64(tick) / 30.0,
			Heroes: make(map[string]HeroData),
		}

		// Process Team 2 heroes
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

			tickOut.Heroes[heroKey] = HeroData{
				Position: hero.Position,
				Team:     hero.Team,
				Time:     float64(tick) / 30.0,
				Visible:  len(seenBy) > 0,
				SeenBy:   seenBy,
			}
		}

		// Process Team 3 heroes
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

			tickOut.Heroes[heroKey] = HeroData{
				Position: hero.Position,
				Team:     hero.Team,
				Time:     float64(tick) / 30.0,
				Visible:  len(seenBy) > 0,
				SeenBy:   seenBy,
			}
		}

		output.Ticks[tick] = tickOut
	}

	return output
}

// isVisible checks if a hero is within the vision range of an entity
func isVisible(hero EntityData, observer EntityData) bool {
	const cellSize float64 = 250.0 // Dota 2 cell size (250 units per cell)

	// Calculate true world position: position = cell * 250 + vec
	heroX := float64(hero.CellX)*cellSize + float64(hero.VecX)
	heroY := float64(hero.CellY)*cellSize + float64(hero.VecY)

	observerX := float64(observer.CellX)*cellSize + float64(observer.VecX)
	observerY := float64(observer.CellY)*cellSize + float64(observer.VecY)

	// Calculate distance
	dx := heroX - observerX
	dy := heroY - observerY

	// Using squared distance to avoid sqrt for performance
	distSquared := dx*dx + dy*dy
	visionRange := float64(observer.VisionRange)
	visionRangeSquared := visionRange * visionRange

	return distSquared <= visionRangeSquared
}
