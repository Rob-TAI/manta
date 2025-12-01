package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"sort"
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

	targetTick := uint32(9272)
	tickData := make(map[string]struct {
		ClassName   string
		Team        uint64
		CellX       uint64
		CellY       uint64
		CellZ       uint64
		VisionRange int32
	})

	p.OnEntity(func(e *manta.Entity, op manta.EntityOp) error {
		if p.Tick < targetTick {
			return nil
		}
		if p.Tick > targetTick {
			return fmt.Errorf("done")
		}

		className := e.GetClassName()
		isHero := strings.HasPrefix(className, "CDOTA_Unit_Hero_")
		isTower := className == "CDOTA_BaseNPC_Tower"
		isFort := className == "CDOTA_BaseNPC_Fort"

		if !isHero && !isTower && !isFort {
			return nil
		}

		cellX, _ := e.GetUint64("CBodyComponent.m_cellX")
		cellY, _ := e.GetUint64("CBodyComponent.m_cellY")
		cellZ, _ := e.GetUint64("CBodyComponent.m_cellZ")
		teamNum, _ := e.GetUint64("m_iTeamNum")
		visionRange, _ := e.GetInt32("m_iDayTimeVisionRange")

		entityKey := fmt.Sprintf("%s_%d", className, e.GetIndex())
		tickData[entityKey] = struct {
			ClassName   string
			Team        uint64
			CellX       uint64
			CellY       uint64
			CellZ       uint64
			VisionRange int32
		}{
			ClassName:   className,
			Team:        teamNum,
			CellX:       cellX,
			CellY:       cellY,
			CellZ:       cellZ,
			VisionRange: visionRange,
		}

		return nil
	})

	if err := p.Start(); err != nil {
		if !strings.Contains(err.Error(), "done") {
			log.Fatalf("parser error: %s", err)
		}
	}

	fmt.Printf("TICK: %d (%.2fs, %.2f minutes)\n", targetTick, float64(targetTick)/30.0, float64(targetTick)/1800.0)
	fmt.Printf("%s\n\n", strings.Repeat("=", 80))

	// Separate entities
	team2Heroes := []string{}
	team3Heroes := []string{}
	team3Towers := []string{}

	for key, data := range tickData {
		isHero := strings.HasPrefix(data.ClassName, "CDOTA_Unit_Hero_")
		if isHero && data.Team == 2 {
			team2Heroes = append(team2Heroes, key)
		} else if isHero && data.Team == 3 {
			team3Heroes = append(team3Heroes, key)
		} else if !isHero && data.Team == 3 {
			team3Towers = append(team3Towers, key)
		}
	}

	// Analyze each Team 2 hero
	for _, heroKey := range team2Heroes {
		heroData := tickData[heroKey]
		fmt.Printf("\n%s (Team 2):\n", heroKey)
		fmt.Printf("  Position: (%d, %d, %d)\n", heroData.CellX, heroData.CellY, heroData.CellZ)

		// Calculate distances to all Team 3 towers
		type distCheck struct {
			entity   string
			distance float64
			vision   int32
			inRange  bool
		}
		var checks []distCheck

		for _, towerKey := range team3Towers {
			towerData := tickData[towerKey]
			dx := float64(heroData.CellX) - float64(towerData.CellX)
			dy := float64(heroData.CellY) - float64(towerData.CellY)
			dist := math.Sqrt(dx*dx + dy*dy)
			inRange := dist <= float64(towerData.VisionRange)

			checks = append(checks, distCheck{
				entity:   towerKey,
				distance: dist,
				vision:   towerData.VisionRange,
				inRange:  inRange,
			})
		}

		// Sort by distance
		sort.Slice(checks, func(i, j int) bool {
			return checks[i].distance < checks[j].distance
		})

		// Show all in-range towers
		inRangeCount := 0
		fmt.Printf("  Towers that can see this hero:\n")
		for _, check := range checks {
			if check.inRange {
				fmt.Printf("    %s: dist=%.1f, vision=%d (IN RANGE)\n",
					check.entity, check.distance, check.vision)
				inRangeCount++
			}
		}
		if inRangeCount == 0 {
			fmt.Printf("    (none)\n")
		}

		// Show closest 3 out-of-range towers
		fmt.Printf("  Closest out-of-range towers:\n")
		shown := 0
		for _, check := range checks {
			if !check.inRange && shown < 3 {
				fmt.Printf("    %s: dist=%.1f, vision=%d (OUT OF RANGE)\n",
					check.entity, check.distance, check.vision)
				shown++
			}
		}
	}

	// Show some tower positions for reference
	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	fmt.Printf("SAMPLE TEAM 3 TOWER POSITIONS:\n")
	fmt.Printf("%s\n", strings.Repeat("=", 80))
	for i, towerKey := range team3Towers {
		if i >= 5 {
			break
		}
		towerData := tickData[towerKey]
		fmt.Printf("%s: pos=(%d, %d, %d) vision=%d\n",
			towerKey, towerData.CellX, towerData.CellY, towerData.CellZ, towerData.VisionRange)
	}
}
