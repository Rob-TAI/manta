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
	CellX int32 `json:"m_cellX"`
	CellY int32 `json:"m_cellY"`
	CellZ int32 `json:"m_cellZ"`
}

type EntityData struct {
	Position
	Team        uint64
	VisionRange int32
	ClassName   string
}

type HeroData struct {
	Position
	Team    uint64   `json:"team"`
	Time    float64  `json:"time"`
	Visible bool     `json:"visible"`
	SeenBy  []string `json:"seen_by"`
}

type TickData struct {
	Team2 map[string]EntityData
	Team3 map[string]EntityData
}

type TickOutput struct {
	Time   float64             `json:"time"`
	Heroes map[string]HeroData `json:"heroes"`
}

type CombinedOutput struct {
	Ticks map[uint32]TickOutput `json:"ticks"`
}

func main() {
	replayPath := flag.String("replay", "replay.dem", "Path to replay .dem file")
	outPath := flag.String("out", "visibility_and_pos.json", "Output JSON file path")
	maxTick := flag.Int("maxtick", 0, "Maximum tick to parse (0 = unlimited)")
	cellSize := flag.Float64("cellsize", 128.0, "World units per cell (Source2 grid). 128 is a common value.")
	flag.Parse()

	f, err := os.Open(*replayPath)
	if err != nil {
		log.Fatalf("unable to open replay file %q: %s", *replayPath, err)
	}
	defer f.Close()

	p, err := manta.NewStreamParser(f)
	if err != nil {
		log.Fatalf("unable to create parser: %s", err)
	}

	// Cache hero day vision by class name (usually static; you can remove caching if you want it tick-accurate).
	visionRanges := make(map[string]int32)

	// Per-tick entity snapshots (only for entities that update/appear that tick).
	ticks := make(map[uint32]TickData)

	entityCount := 0
	lastTick := uint32(0)
	lastReportedMinute := uint32(0)
	const ticksPerMinute = uint32(1800) // 30 ticks/sec * 60 sec

	p.OnEntity(func(e *manta.Entity, op manta.EntityOp) error {
		entityCount++
		lastTick = p.Tick

		// Progress every minute of game time
		currentMinute := p.Tick / ticksPerMinute
		if currentMinute > lastReportedMinute {
			gameTimeSec := float64(p.Tick) / 30.0
			fmt.Printf("Progress: %d minutes (%.1f seconds) - Processed %d entities, captured %d ticks\n",
				currentMinute, gameTimeSec, entityCount, len(ticks))
			lastReportedMinute = currentMinute
		}

		// Stop at max tick if requested
		if *maxTick > 0 && p.Tick > uint32(*maxTick) {
			return fmt.Errorf("reached max tick %d", *maxTick)
		}

		className := e.GetClassName()

		isHero := strings.HasPrefix(className, "CDOTA_Unit_Hero_")
		isTower := className == "CDOTA_BaseNPC_Tower"
		isFort := className == "CDOTA_BaseNPC_Fort"
		if !isHero && !isTower && !isFort {
			return nil
		}

		// IMPORTANT: cells are grid indices; treat them as signed (int32), not uint64.
		cellX, _ := e.GetInt32("CBodyComponent.m_cellX")
		cellY, _ := e.GetInt32("CBodyComponent.m_cellY")
		cellZ, _ := e.GetInt32("CBodyComponent.m_cellZ")
		teamNum, _ := e.GetUint64("m_iTeamNum")

		dayVision := int32(0)
		if isHero {
			if v, ok := visionRanges[className]; ok {
				dayVision = v
			} else {
				v, _ := e.GetInt32("m_iDayTimeVisionRange")
				visionRanges[className] = v
				dayVision = v
			}
		} else {
			// Towers / forts
			dayVision, _ = e.GetInt32("m_iDayTimeVisionRange")
		}

		// Ensure tick entry exists
		td, ok := ticks[p.Tick]
		if !ok {
			td = TickData{
				Team2: make(map[string]EntityData),
				Team3: make(map[string]EntityData),
			}
		}

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

		entityKey := fmt.Sprintf("%s_%d", className, e.GetIndex())
		if teamNum == 2 {
			td.Team2[entityKey] = entityData
		} else if teamNum == 3 {
			td.Team3[entityKey] = entityData
		}

		ticks[p.Tick] = td
		return nil
	})

	if *maxTick > 0 {
		fmt.Printf("Parsing up to tick %d (%.1f minutes)...\n", *maxTick, float64(*maxTick)/1800.0)
	} else {
		fmt.Printf("Parsing entire replay (unlimited)...\n")
	}

	if err := p.Start(); err != nil {
		// Stop-error is expected when max tick is reached
		if !strings.Contains(err.Error(), "reached max tick") {
			log.Fatalf("parser error: %s", err)
		}
	}

	finalGameTime := float64(lastTick) / 30.0
	fmt.Printf("\n[OK] Parsing complete: %d entities processed up to tick %d (%.1f minutes / %.1f seconds)\n",
		entityCount, lastTick, finalGameTime/60.0, finalGameTime)

	fmt.Printf("Calculating visibility and building output for %d ticks...\n", len(ticks))
	output := buildCombinedOutput(ticks, *cellSize)

	outFile, err := os.Create(*outPath)
	if err != nil {
		log.Fatalf("unable to create output JSON file %q: %s", *outPath, err)
	}
	defer outFile.Close()

	enc := json.NewEncoder(outFile)
	enc.SetIndent("", "  ")
	if err := enc.Encode(output); err != nil {
		log.Fatalf("unable to encode JSON: %s", err)
	}

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

	fmt.Printf("\n[OK] Data written to %s\n", *outPath)
	fmt.Printf("Statistics:\n")
	fmt.Printf("  - Total ticks captured: %d\n", len(output.Ticks))
	fmt.Printf("  - Total hero snapshots: %d\n", totalHeroes)
	if totalHeroes > 0 {
		fmt.Printf("  - Heroes visible: %d (%.1f%%)\n", totalVisible, float64(totalVisible)/float64(totalHeroes)*100.0)
		fmt.Printf("  - Heroes invisible: %d (%.1f%%)\n", totalInvisible, float64(totalInvisible)/float64(totalHeroes)*100.0)
	}
}

// buildCombinedOutput creates the combined output structure with a simple distance-based visibility check.
// NOTE: This ignores terrain/trees/highground/night vision/invis/true sight/etc.
// It’s purely “within enemy hero day vision radius”.
func buildCombinedOutput(ticks map[uint32]TickData, cellSize float64) CombinedOutput {
	output := CombinedOutput{
		Ticks: make(map[uint32]TickOutput),
	}

	for tick, tickData := range ticks {
		tickOut := TickOutput{
			Time:   float64(tick) / 30.0,
			Heroes: make(map[string]HeroData),
		}

		// Team 2 heroes (seen by Team 3 heroes)
		for heroKey, hero := range tickData.Team2 {
			if hero.Team != 2 || !strings.HasPrefix(hero.ClassName, "CDOTA_Unit_Hero_") {
				continue
			}

			seenBy := make([]string, 0, 2)
			for observerKey, observer := range tickData.Team3 {
				if observer.Team != 3 || observer.VisionRange <= 0 {
					continue
				}
				if !strings.HasPrefix(observer.ClassName, "CDOTA_Unit_Hero_") {
					continue
				}

				if isVisible(hero, observer, cellSize) {
					seenBy = append(seenBy, observerKey)
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

		// Team 3 heroes (seen by Team 2 heroes)
		for heroKey, hero := range tickData.Team3 {
			if hero.Team != 3 || !strings.HasPrefix(hero.ClassName, "CDOTA_Unit_Hero_") {
				continue
			}

			seenBy := make([]string, 0, 2)
			for observerKey, observer := range tickData.Team2 {
				if observer.Team != 2 || observer.VisionRange <= 0 {
					continue
				}
				if !strings.HasPrefix(observer.ClassName, "CDOTA_Unit_Hero_") {
					continue
				}

				if isVisible(hero, observer, cellSize) {
					seenBy = append(seenBy, observerKey)
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

// isVisible checks if a hero is within observer's vision range using grid cells converted to world units.
func isVisible(hero EntityData, observer EntityData, cellSize float64) bool {
	dxCells := int64(hero.CellX) - int64(observer.CellX)
	dyCells := int64(hero.CellY) - int64(observer.CellY)

	// Convert cell delta -> world-units delta
	dx := float64(dxCells) * cellSize
	dy := float64(dyCells) * cellSize

	distSq := dx*dx + dy*dy
	vr := float64(observer.VisionRange)
	return distSq <= vr*vr
}
