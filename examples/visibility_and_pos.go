package main

/*

Vision And Position Data

TODO:
- Account for day/night
- High ground? (Z position is stored, might work...)
- Invisibility & smoke
- Spells (particles) that grant vision

*/

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/dotabuff/manta"
	"github.com/dotabuff/manta/dota"
)

type Position struct {
	CellX int32
	CellY int32
	CellZ int32
	VecX  float32
	VecY  float32
	VecZ  float32
}

type WorldPosition struct {
	PosX float64 `json:"pos_x"`
	PosY float64 `json:"pos_y"`
	PosZ float64 `json:"pos_z"`
}

type EntityData struct {
	Position
	Team        uint64
	VisionRange int32
	ClassName   string
	PlayerID    int32 // Stable player ID for heroes (-1 for non-heroes)
	// Precomputed world coordinates for performance
	WorldX float64
	WorldY float64
	WorldZ float64
}

type TempViewerData struct {
	GridX  int32
	GridY  int32
	Radius int32
	Team   uint64
	// Precomputed world coordinates for performance
	WorldX float64
	WorldY float64
}

type HeroData struct {
	WorldPosition
	Team    uint64   `json:"team"`
	Time    float64  `json:"time"`
	Visible bool     `json:"visible"`
	SeenBy  []string `json:"seen_by"`
}

type UnitData struct {
	WorldPosition
	Team uint64 `json:"team"`
	Type string `json:"type"` // "tower", "creep", "fort"
}

type TickData struct {
	Team2       map[string]EntityData
	Team3       map[string]EntityData
	TempViewers []TempViewerData
}

type TickOutput struct {
	Time   float64             `json:"time"`
	Heroes map[string]HeroData `json:"heroes"`
	Units  map[string]UnitData `json:"units"`
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

	// Track min/max positions for debugging
	minX := float64(999999)
	maxX := float64(-999999)
	minY := float64(999999)
	maxY := float64(-999999)
	minZ := float64(999999)
	maxZ := float64(-999999)

	// Rolling world state - maintains last-known position of all entities
	state := TickData{
		Team2:       make(map[string]EntityData),
		Team3:       make(map[string]EntityData),
		TempViewers: []TempViewerData{},
	}

	// Output accumulator
	output := CombinedOutput{
		Ticks: make(map[uint32]TickOutput),
	}

	// Track current tick for flush detection
	currentTick := uint32(0)
	haveTick := false

	entityCount := 0
	lastTick := uint32(0)
	lastReportedMinute := uint32(0)
	const ticksPerMinute = uint32(1800) // 30 ticks/sec * 60 sec = 1800 ticks/minute

	// Flush function: snapshot current world state and compute visibility for this tick
	flush := func(tick uint32) {
		tickOut := TickOutput{
			Time:   float64(tick) / 30.0,
			Heroes: make(map[string]HeroData),
			Units:  make(map[string]UnitData),
		}

		// Process Team 2 heroes
		for heroKey, hero := range state.Team2 {
			if !strings.HasPrefix(hero.ClassName, "CDOTA_Unit_Hero_") || hero.Team != 2 {
				continue
			}

			seenBy := make([]string, 0, 8) // Preallocate with small capacity
			seenByCreep := false
			seenByTower := false
			seenByFort := false
			seenByTempViewer := false

			for entityKey, entity := range state.Team3 {
				// Only check entities from opposing team with vision range
				if entity.Team != 3 || entity.VisionRange <= 0 {
					continue
				}
				if isVisible(hero, entity) {
					// Consolidate by entity type
					if entity.ClassName == "CDOTA_BaseNPC_Creep_Lane" {
						seenByCreep = true
					} else if entity.ClassName == "CDOTA_BaseNPC_Tower" {
						seenByTower = true
					} else if entity.ClassName == "CDOTA_BaseNPC_Fort" {
						seenByFort = true
					} else if strings.HasPrefix(entity.ClassName, "CDOTA_Unit_Hero_") {
						// Keep individual hero entries
						seenBy = append(seenBy, entityKey)
					}
				}
			}

			// Check temp viewers from opposing team
			for _, viewer := range state.TempViewers {
				if viewer.Team != 3 {
					continue
				}
				if isVisibleToTempViewer(hero, viewer) {
					seenByTempViewer = true
					break
				}
			}

			// Add consolidated entries (normalized labels)
			if seenByCreep {
				seenBy = append(seenBy, "creep")
			}
			if seenByTower {
				seenBy = append(seenBy, "tower")
			}
			if seenByFort {
				seenBy = append(seenBy, "fort")
			}
			if seenByTempViewer {
				seenBy = append(seenBy, "temp_viewer")
			}

			// Sort for stable output
			sort.Strings(seenBy)

			tickOut.Heroes[heroKey] = HeroData{
				WorldPosition: WorldPosition{
					PosX: hero.WorldX,
					PosY: hero.WorldY,
					PosZ: hero.WorldZ,
				},
				Team:    hero.Team,
				Time:    float64(tick) / 30.0,
				Visible: len(seenBy) > 0,
				SeenBy:  seenBy,
			}
		}

		// Process Team 3 heroes
		for heroKey, hero := range state.Team3 {
			if !strings.HasPrefix(hero.ClassName, "CDOTA_Unit_Hero_") || hero.Team != 3 {
				continue
			}

			seenBy := make([]string, 0, 8) // Preallocate with small capacity
			seenByCreep := false
			seenByTower := false
			seenByFort := false
			seenByTempViewer := false

			for entityKey, entity := range state.Team2 {
				// Only check entities from opposing team with vision range
				if entity.Team != 2 || entity.VisionRange <= 0 {
					continue
				}
				if isVisible(hero, entity) {
					// Consolidate by entity type
					if entity.ClassName == "CDOTA_BaseNPC_Creep_Lane" {
						seenByCreep = true
					} else if entity.ClassName == "CDOTA_BaseNPC_Tower" {
						seenByTower = true
					} else if entity.ClassName == "CDOTA_BaseNPC_Fort" {
						seenByFort = true
					} else if strings.HasPrefix(entity.ClassName, "CDOTA_Unit_Hero_") {
						// Keep individual hero entries
						seenBy = append(seenBy, entityKey)
					}
				}
			}

			// Check temp viewers from opposing team
			for _, viewer := range state.TempViewers {
				if viewer.Team != 2 {
					continue
				}
				if isVisibleToTempViewer(hero, viewer) {
					seenByTempViewer = true
					break
				}
			}

			// Add consolidated entries (normalized labels)
			if seenByCreep {
				seenBy = append(seenBy, "creep")
			}
			if seenByTower {
				seenBy = append(seenBy, "tower")
			}
			if seenByFort {
				seenBy = append(seenBy, "fort")
			}
			if seenByTempViewer {
				seenBy = append(seenBy, "temp_viewer")
			}

			// Sort for stable output
			sort.Strings(seenBy)

			tickOut.Heroes[heroKey] = HeroData{
				WorldPosition: WorldPosition{
					PosX: hero.WorldX,
					PosY: hero.WorldY,
					PosZ: hero.WorldZ,
				},
				Team:    hero.Team,
				Time:    float64(tick) / 30.0,
				Visible: len(seenBy) > 0,
				SeenBy:  seenBy,
			}
		}

		output.Ticks[tick] = tickOut
	}

	// Register tick callback to flush on every tick (ensures no ticks are skipped)
	p.Callbacks.OnCNETMsg_Tick(func(m *dota.CNETMsg_Tick) error {
		// Initialize current tick tracking
		if !haveTick {
			currentTick = p.Tick
			haveTick = true
		}

		// Flush when tick changes
		if p.Tick != currentTick {
			flush(currentTick)
			currentTick = p.Tick
		}

		// Check if we've reached the tick limit
		if *maxTick > 0 && p.Tick > uint32(*maxTick) {
			return fmt.Errorf("reached max tick %d", *maxTick)
		}

		return nil
	})

	p.OnEntity(func(e *manta.Entity, op manta.EntityOp) error {
		entityCount++
		lastTick = p.Tick

		// Report progress every minute of game time
		currentMinute := p.Tick / ticksPerMinute
		if currentMinute > lastReportedMinute {
			gameTime := float64(p.Tick) / 30.0
			fmt.Printf("Progress: %d minutes (%.1f seconds) - Processed %d entities, captured %d ticks\n",
				currentMinute, gameTime, entityCount, len(output.Ticks))
			lastReportedMinute = currentMinute
		}

		className := e.GetClassName()

		// Check if this is one of our target classes
		isHero := strings.HasPrefix(className, "CDOTA_Unit_Hero_")
		isTower := className == "CDOTA_BaseNPC_Tower"
		isFort := className == "CDOTA_BaseNPC_Fort"
		isCreep := className == "CDOTA_BaseNPC_Creep_Lane"
		isTempViewer := className == "CDOTAFogOfWarTempViewers"

		if !isHero && !isTower && !isFort && !isCreep && !isTempViewer {
			return nil
		}

		// Create unique entity key
		// For heroes, try to use player ID for stable identity
		var entityKey string
		if isHero {
			if playerID, ok := e.GetInt32("m_iPlayerID"); ok && playerID >= 0 {
				// Use player ID for stable hero identity (e.g., "CDOTA_Unit_Hero_Abaddon_player_0")
				entityKey = fmt.Sprintf("%s_player_%d", className, playerID)
			} else {
				// Fallback to index/serial for heroes without player ID
				entityKey = fmt.Sprintf("%s_%d_%d", className, e.GetIndex(), e.GetSerial())
			}
		} else {
			// Non-heroes use index/serial (prevents key collisions on entity recycling)
			entityKey = fmt.Sprintf("%s_%d_%d", className, e.GetIndex(), e.GetSerial())
		}

		// Handle entity deletion/removal
		if op.Flag(manta.EntityOpDeleted) || op.Flag(manta.EntityOpLeft) {
			// Clear temp viewers if they're deleted
			if isTempViewer {
				state.TempViewers = state.TempViewers[:0]
				return nil
			}
			delete(state.Team2, entityKey)
			delete(state.Team3, entityKey)
			return nil
		}

		// Handle temp viewers separately
		if isTempViewer {
			// Replace temp viewers array (don't append indefinitely)
			state.TempViewers = state.TempViewers[:0]

			const cellSize float64 = 128.0 // Dota 2 cell size (250 units per cell)

			if teamNum, ok := e.GetUint64("m_iTeamNum"); ok {
				// Check up to 64 viewers (10 was too low)
				for i := 0; i < 64; i++ {
					prefix := fmt.Sprintf("m_TempViewerInfo.%04d", i)
					bValid, ok := e.GetBool(prefix + ".m_bValid")
					if !ok || !bValid {
						continue
					}

					gridX, _ := e.GetInt32(prefix + ".m_nGridX")
					gridY, _ := e.GetInt32(prefix + ".m_nGridY")
					radius, _ := e.GetInt32(prefix + ".m_nRadius")

					// Precompute world coordinates (grid coordinates need scaling)
					worldX := float64(gridX) * cellSize
					worldY := float64(gridY) * cellSize

					state.TempViewers = append(state.TempViewers, TempViewerData{
						GridX:  gridX,
						GridY:  gridY,
						Radius: radius,
						Team:   teamNum,
						WorldX: worldX,
						WorldY: worldY,
					})
				}
			}

			return nil
		}

		// Extract required fields for heroes/towers/forts/creeps
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

		// Get vision range and player ID for this entity
		dayVision := int32(0)
		playerID := int32(-1) // Default to -1 for non-heroes

		if isHero {
			// Store vision ranges for heroes (static data)
			if _, exists := visionRanges[className]; !exists {
				dayVision, _ = e.GetInt32("m_iDayTimeVisionRange")
				visionRanges[className] = dayVision
			} else {
				dayVision = visionRanges[className]
			}
			// Extract player ID for stable hero identity
			playerID, _ = e.GetInt32("m_iPlayerID")
		} else if isTower || isFort || isCreep {
			// Get vision range for towers/forts/creeps
			dayVision, _ = e.GetInt32("m_iDayTimeVisionRange")
		}

		// Precompute world coordinates for performance
		const cellSize float64 = 128.0 // Dota 2 cell size (128 units per cell)
		worldX := float64(cellX)*cellSize + float64(vecX)
		worldY := float64(cellY)*cellSize + float64(vecY)
		worldZ := float64(cellZ)*cellSize + float64(vecZ)

		// Track min/max for debugging
		if worldX < minX {
			minX = worldX
		}
		if worldX > maxX {
			maxX = worldX
		}
		if worldY < minY {
			minY = worldY
		}
		if worldY > maxY {
			maxY = worldY
		}
		if worldZ < minZ {
			minZ = worldZ
		}
		if worldZ > maxZ {
			maxZ = worldZ
		}

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
			PlayerID:    playerID,
			WorldX:      worldX,
			WorldY:      worldY,
			WorldZ:      worldZ,
		}

		// Update rolling world state
		if teamNum == 2 {
			state.Team2[entityKey] = entityData
		} else if teamNum == 3 {
			state.Team3[entityKey] = entityData
		}

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

	// Flush final tick
	if haveTick {
		flush(currentTick)
	}

	finalGameTime := float64(lastTick) / 30.0
	fmt.Printf("\n[OK] Parsing complete: %d entities processed up to tick %d (%.1f minutes / %.1f seconds)\n",
		entityCount, lastTick, finalGameTime/60.0, finalGameTime)

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
	fmt.Printf("\nPosition ranges:\n")
	fmt.Printf("  - X: %.2f to %.2f (range: %.2f)\n", minX, maxX, maxX-minX)
	fmt.Printf("  - Y: %.2f to %.2f (range: %.2f)\n", minY, maxY, maxY-minY)
	fmt.Printf("  - Z: %.2f to %.2f (range: %.2f)\n", minZ, maxZ, maxZ-minZ)
}

// isVisible checks if a hero is within the vision range of an entity
func isVisible(hero EntityData, observer EntityData) bool {
	// Use precomputed world coordinates
	dx := hero.WorldX - observer.WorldX
	dy := hero.WorldY - observer.WorldY

	// Using squared distance to avoid sqrt for performance
	distSquared := dx*dx + dy*dy
	visionRange := float64(observer.VisionRange)
	visionRangeSquared := visionRange * visionRange

	return distSquared <= visionRangeSquared
}

// isVisibleToTempViewer checks if a hero is within the vision range of a temp viewer (ward)
func isVisibleToTempViewer(hero EntityData, viewer TempViewerData) bool {
	// Use precomputed world coordinates
	dx := hero.WorldX - viewer.WorldX
	dy := hero.WorldY - viewer.WorldY

	// Using squared distance to avoid sqrt for performance
	distSquared := dx*dx + dy*dy
	// Radius is already in world units (no scaling needed)
	visionRange := float64(viewer.Radius)
	visionRangeSquared := visionRange * visionRange

	return distSquared <= visionRangeSquared
}
