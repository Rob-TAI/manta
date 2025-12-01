package main

import (
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

func main() {
	maxTick := flag.Int("maxtick", 10000, "Maximum tick to parse")
	flag.Parse()

	f, err := os.Open("replay.dem")
	if err != nil {
		log.Fatalf("unable to open file: %s", err)
	}
	defer f.Close()

	p, err := manta.NewStreamParser(f)
	if err != nil {
		log.Fatalf("unable to create parser: %s", err)
	}

	heroes := make(map[string]EntityData)
	observers := make(map[string]EntityData)
	sampledTick := uint32(0)

	p.OnEntity(func(e *manta.Entity, op manta.EntityOp) error {
		if *maxTick > 0 && p.Tick > uint32(*maxTick) {
			return fmt.Errorf("reached max tick")
		}

		className := e.GetClassName()
		isHero := strings.HasPrefix(className, "CDOTA_Unit_Hero_")
		isTower := className == "CDOTA_BaseNPC_Tower"
		isFort := className == "CDOTA_BaseNPC_Fort"

		if !isHero && !isTower && !isFort {
			return nil
		}

		// Read cell coords as uint64 and cast to int32 to handle signed values
		cellXRaw, _ := e.GetUint64("CBodyComponent.m_cellX")
		cellYRaw, _ := e.GetUint64("CBodyComponent.m_cellY")
		cellZRaw, _ := e.GetUint64("CBodyComponent.m_cellZ")
		cellX := int32(cellXRaw)
		cellY := int32(cellYRaw)
		cellZ := int32(cellZRaw)
		teamNum, _ := e.GetUint64("m_iTeamNum")
		dayVision, _ := e.GetInt32("m_iDayTimeVisionRange")

		// Debug: print first few reads
		if len(heroes) == 0 && isHero {
			fmt.Printf("First hero %s: cellX=%d, cellY=%d, cellZ=%d\n",
				className, cellX, cellY, cellZ)
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

		// Sample at any tick if we haven't captured enough data yet
		if len(heroes) < 10 || len(observers) < 10 {
			sampledTick = p.Tick
			if isHero {
				heroes[entityKey] = entityData
			} else if isTower || isFort {
				observers[entityKey] = entityData
			}
		}

		return nil
	})

	fmt.Printf("Parsing up to tick %d...\n", *maxTick)
	if err := p.Start(); err != nil && !strings.Contains(err.Error(), "reached max tick") {
		log.Fatalf("parser error: %s", err)
	}

	fmt.Printf("\nSampled data at tick %d (%.1f seconds):\n\n", sampledTick, float64(sampledTick)/30.0)

	// Print some heroes
	fmt.Println("=== HEROES ===")
	count := 0
	for key, hero := range heroes {
		fmt.Printf("%s: Team=%d, Pos=(%d, %d, %d), VisionRange=%d\n",
			key, hero.Team, hero.CellX, hero.CellY, hero.CellZ, hero.VisionRange)
		count++
		if count >= 5 {
			break
		}
	}

	// Print some observers
	fmt.Println("\n=== OBSERVERS (Towers/Forts) ===")
	count = 0
	for key, obs := range observers {
		fmt.Printf("%s: Team=%d, Pos=(%d, %d, %d), VisionRange=%d\n",
			key, obs.Team, obs.CellX, obs.CellY, obs.CellZ, obs.VisionRange)
		count++
		if count >= 5 {
			break
		}
	}

	// Calculate some sample distances
	fmt.Println("\n=== SAMPLE VISIBILITY CHECKS ===")
	const cellSize float64 = 128.0

	for heroKey, hero := range heroes {
		fmt.Printf("\nHero: %s (Team %d) at (%d, %d)\n", heroKey, hero.Team, hero.CellX, hero.CellY)

		checkedCount := 0
		for obsKey, obs := range observers {
			if obs.Team == hero.Team || obs.VisionRange <= 0 {
				continue
			}

			dxCells := int64(hero.CellX) - int64(obs.CellX)
			dyCells := int64(hero.CellY) - int64(obs.CellY)
			dx := float64(dxCells) * cellSize
			dy := float64(dyCells) * cellSize
			distSquared := dx*dx + dy*dy
			dist := fmt.Sprintf("%.0f", (distSquared))
			vr := float64(obs.VisionRange)
			visible := distSquared <= vr*vr

			fmt.Printf("  vs %s: cellDelta=(%d,%d), worldDelta=(%.0f,%.0f), dist²=%s, visionRange=%.0f, visible=%v\n",
				obsKey, dxCells, dyCells, dx, dy, dist, vr, visible)

			checkedCount++
			if checkedCount >= 3 {
				break
			}
		}

		// Only check one hero
		break
	}
}
