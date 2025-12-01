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

	firstTick := uint32(0)
	firstTickData := make(map[string]struct {
		ClassName   string
		Team        uint64
		CellX       uint64
		CellY       uint64
		CellZ       uint64
		VisionRange int32
	})

	p.OnEntity(func(e *manta.Entity, op manta.EntityOp) error {
		// Capture first tick only
		if firstTick == 0 {
			firstTick = p.Tick
		}

		if p.Tick != firstTick {
			return fmt.Errorf("done with first tick")
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
		firstTickData[entityKey] = struct {
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
		if !strings.Contains(err.Error(), "done with first tick") {
			log.Fatalf("parser error: %s", err)
		}
	}

	fmt.Printf("FIRST TICK: %d (%.2fs)\n", firstTick, float64(firstTick)/30.0)
	fmt.Printf("%s\n\n", strings.Repeat("=", 80))

	// Separate entities by type and team
	team2Heroes := make(map[string]interface{})
	team3Heroes := make(map[string]interface{})
	team2Towers := make(map[string]interface{})
	team3Towers := make(map[string]interface{})

	for key, data := range firstTickData {
		isHero := strings.HasPrefix(data.ClassName, "CDOTA_Unit_Hero_")
		if isHero {
			if data.Team == 2 {
				team2Heroes[key] = data
			} else if data.Team == 3 {
				team3Heroes[key] = data
			}
		} else {
			if data.Team == 2 {
				team2Towers[key] = data
			} else if data.Team == 3 {
				team3Towers[key] = data
			}
		}
	}

	fmt.Printf("TEAM 2 (Radiant) Heroes: %d\n", len(team2Heroes))
	for key, v := range team2Heroes {
		data := v.(struct {
			ClassName   string
			Team        uint64
			CellX       uint64
			CellY       uint64
			CellZ       uint64
			VisionRange int32
		})
		fmt.Printf("  %s: pos=(%d,%d,%d) vision=%d\n", key, data.CellX, data.CellY, data.CellZ, data.VisionRange)
	}

	fmt.Printf("\nTEAM 3 (Dire) Heroes: %d\n", len(team3Heroes))
	for key, v := range team3Heroes {
		data := v.(struct {
			ClassName   string
			Team        uint64
			CellX       uint64
			CellY       uint64
			CellZ       uint64
			VisionRange int32
		})
		fmt.Printf("  %s: pos=(%d,%d,%d) vision=%d\n", key, data.CellX, data.CellY, data.CellZ, data.VisionRange)
	}

	fmt.Printf("\nTEAM 2 (Radiant) Towers/Forts: %d\n", len(team2Towers))
	fmt.Printf("\nTEAM 3 (Dire) Towers/Forts: %d\n", len(team3Towers))

	// Calculate distance from Team 2 heroes to Team 3 towers
	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	fmt.Printf("VISIBILITY ANALYSIS\n")
	fmt.Printf("%s\n\n", strings.Repeat("=", 80))

	for heroKey, hv := range team2Heroes {
		heroData := hv.(struct {
			ClassName   string
			Team        uint64
			CellX       uint64
			CellY       uint64
			CellZ       uint64
			VisionRange int32
		})

		fmt.Printf("\n%s (Team 2):\n", heroKey)

		// Check which Team 3 entities can see this hero
		type visCheck struct {
			entity   string
			distance float64
			inRange  bool
			vision   int32
		}
		var checks []visCheck

		for towerKey, tv := range team3Towers {
			towerData := tv.(struct {
				ClassName   string
				Team        uint64
				CellX       uint64
				CellY       uint64
				CellZ       uint64
				VisionRange int32
			})

			dx := float64(heroData.CellX) - float64(towerData.CellX)
			dy := float64(heroData.CellY) - float64(towerData.CellY)
			dist := math.Sqrt(dx*dx + dy*dy)
			inRange := dist <= float64(towerData.VisionRange)

			checks = append(checks, visCheck{
				entity:   towerKey,
				distance: dist,
				inRange:  inRange,
				vision:   towerData.VisionRange,
			})
		}

		// Sort by distance
		sort.Slice(checks, func(i, j int) bool {
			return checks[i].distance < checks[j].distance
		})

		// Show closest 5 and any in range
		fmt.Printf("  Closest Team 3 entities:\n")
		shown := 0
		for _, check := range checks {
			if shown < 5 || check.inRange {
				status := "OUT OF RANGE"
				if check.inRange {
					status = "*** IN RANGE ***"
				}
				fmt.Printf("    %s: dist=%.1f vision=%d %s\n",
					check.entity, check.distance, check.vision, status)
				shown++
			}
			if shown >= 10 {
				break
			}
		}
	}
}
