# Vision Events in Manta (Dota 2 Replay Parser)

**Status: Work in Progress** ⚠️

This document outlines vision-related data structures and events found in the Manta Dota 2 replay parser. Tracking per-frame player visibility is currently **not fully implemented** and requires further investigation.

## Overview

Determining if a player can see another player in Dota 2 requires combining multiple data sources:
1. Entity lifecycle events (Enter/Leave)
2. Team assignments and player IDs
3. Fog of War (FoW) mechanics
4. Vision modifiers (invisibility, wards, true sight)
5. Combat log visibility flags

---

## Key Proto File Findings

### 1. Combat Log Visibility (Most Direct Approach)

**File:** `dota/dota_shared_enums.proto` (lines 767-809)

The `CMsgDOTACombatLogEntry` message includes per-team visibility flags:

```protobuf
message CMsgDOTACombatLogEntry {
    optional DOTA_COMBATLOG_TYPES type = 1;
    optional uint32 target_name = 2;
    optional uint32 attacker_name = 4;
    // ... other fields ...
    optional bool is_visible_radiant = 11;  // ← Visibility for Radiant team
    optional bool is_visible_dire = 12;     // ← Visibility for Dire team
    // ...
}
```

**Usage:** These flags indicate whether specific combat events were visible to each team. This could be used to infer vision states during combat.

**Limitations:** Only provides visibility during combat events, not continuous per-frame visibility.

---

### 2. Entity Lifecycle Events

**File:** `entity.go` (lines 10-22)

Manta tracks entity state changes through `EntityOp` flags:

```go
const (
    EntityOpNone           EntityOp = 0x00
    EntityOpCreated        EntityOp = 0x01
    EntityOpUpdated        EntityOp = 0x02
    EntityOpDeleted        EntityOp = 0x04
    EntityOpEntered        EntityOp = 0x08  // ← Entity became visible
    EntityOpLeft           EntityOp = 0x10  // ← Entity became invisible
    EntityOpCreatedEntered EntityOp = EntityOpCreated | EntityOpEntered
    EntityOpUpdatedEntered EntityOp = EntityOpUpdated | EntityOpEntered
    EntityOpDeletedLeft    EntityOp = EntityOpDeleted | EntityOpLeft
)
```

**Key Events:**
- `EntityOpEntered`: Entity enters the observer's view
- `EntityOpLeft`: Entity leaves the observer's view

**Implementation:** See `entity.go:222` (`onCSVCMsg_PacketEntities`) for entity update handling.

**Potential Use:** By tracking `EntityOpEntered`/`EntityOpLeft` events per player perspective, you could build a visibility state machine.

---

### 3. Fog of War (FoW) Properties

#### Linear Projectiles
**File:** `dota/dota_usermessages.proto` (lines 688-701)

```protobuf
message CDOTAUserMsg_CreateLinearProjectile {
    optional CMsgVector origin = 1;
    optional CMsgVector2D velocity = 2;
    // ...
    optional float fow_radius = 9;        // ← Reveals FoW in this radius
    optional bool sticky_fow_reveal = 10;  // ← Persistent reveal
    optional float distance = 11;
    // ...
}
```

#### Particle Manager FoW
**File:** `dota/usermessages.proto` (lines 448-455)

```protobuf
message SetParticleFoWProperties {
    optional int32 fow_control_point = 1;
    optional int32 fow_control_point2 = 2;
    optional float fow_radius = 3;
}

message SetParticleShouldCheckFoW {
    optional bool check_fow = 1;
}
```

**Events:**
- `GAME_PARTICLE_MANAGER_EVENT_SET_FOW_PROPERTIES = 15`
- `GAME_PARTICLE_MANAGER_EVENT_SET_SHOULD_CHECK_FOW = 17`

---

### 4. Vision-Related Entity Properties

#### Sight Range
**File:** `dota/dota_gcmessages_common.proto` (lines 1558-1559)

```protobuf
optional uint32 sight_range_day = 33;
optional uint32 sight_range_night = 34;
```

#### Vision Modifiers
**File:** `dota/dota_scenariomessages.proto` (lines 216-217)

```protobuf
optional int32 moonshard_consumed_bonus_night_vision = 101;
optional int32 wardtruesight_range = 110;
```

**File:** `dota/dota_shared_enums.proto` (line 817)

```protobuf
optional bool invisibility_modifier = 50;
```

#### Combat Log Type
**File:** `dota/dota_shared_enums.proto` (line 640)

```protobuf
DOTA_COMBATLOG_REVEALED_INVISIBLE = 22;
```

---

### 5. Ward Events & Statistics

#### Ward Placement/Destruction
**File:** `dota/dota_clientmessages.proto` (lines 415-418)

```protobuf
message CDOTAClientMsg_SetDesiredWardPlacement {
    optional uint32 ward_index = 1;
    optional float ward_x = 2;
    optional float ward_y = 3;
}
```

#### Chat Messages
**File:** `dota/dota_usermessages.proto` (lines 274-275)

```protobuf
CHAT_MESSAGE_OBSERVER_WARD_KILLED = 105;
CHAT_MESSAGE_SENTRY_WARD_KILLED = 106;
```

#### Match Statistics
**Files:** Multiple

- `observer_wards_placed` / `sentry_wards_placed` (match stats)
- `wards_dewarded` / `observer_wards_dewarded` (kill tracking)
- `wards_placed` (player stats)

---

### 6. Potentially Visible Set (PVS) - Deprecated

**File:** `dota/netmessages.proto` (lines 451-479)

```protobuf
message CSVCMsg_PacketEntities {
    // ...
    optional uint32 has_pvs_vis_bits_deprecated = 16;  // ← Old visibility system
    // ...
}
```

**Note:** The `pvs_vis_bits` field is deprecated but suggests entity visibility was historically tracked per packet.

---

## Implementation Strategy (Proposed)

To track per-frame player visibility, consider:

### 1. **Monitor Entity Enter/Leave Events**

Register a callback using `Parser.OnEntity()`:

```go
p.OnEntity(func(e *Entity, op EntityOp) error {
    if op.Flag(EntityOpEntered) {
        // Entity became visible
        // Track which player(s) can now see this entity
    }
    if op.Flag(EntityOpLeft) {
        // Entity became invisible
        // Track which player(s) can no longer see this entity
    }
    return nil
})
```

### 2. **Extract Entity Properties**

For each entity, retrieve:
- `m_iPlayerID` - Owner player ID
- `m_iTeamNum` - Team number (2=Radiant, 3=Dire)
- `m_vecX`, `m_vecY` - Position coordinates
- Vision-related modifiers

Example from `entity.go`:
```go
playerID, ok := entity.GetInt32("m_iPlayerID")
teamNum, ok := entity.GetInt32("m_iTeamNum")
```

### 3. **Track Vision State Per Player**

Maintain a data structure like:

```go
type VisionState struct {
    tick           int
    observerPlayer int32
    visibleEntities map[int32]bool // entity index -> visible
}
```

### 4. **Correlate with Combat Log**

Use `is_visible_radiant` / `is_visible_dire` from combat log entries to validate vision calculations during events.

---

## Known Limitations & Open Questions

### Questions:
1. **Entity perspective:** Are Enter/Leave events from a specific player's perspective or global?
2. **Observer wards:** How are ward vision ranges calculated in real-time?
3. **High ground vision:** Is elevation/high ground vision tracked explicitly?
4. **Smoke of Deceit:** How is smoke invisibility represented?
5. **Spectator vision:** Do replays track what each player actually saw, or only server-side truth?

### Limitations:
- No explicit "Player X can see Player Y" event found
- Vision calculation may require geometry/map data
- FoW mechanics may need custom implementation
- Entity Enter/Leave events need validation against actual player perspective

---

## Related Files to Investigate

- `entity.go` - Entity lifecycle and state management
- `dota/dota_shared_enums.proto` - Combat log and enum definitions
- `dota/dota_usermessages.proto` - User messages including FoW
- `dota/netmessages.proto` - Network messages and entity packets
- `parser.go` - Main parser logic and callbacks

---

## Next Steps

1. ✅ Document proto file findings
2. ⬜ Test `EntityOpEntered`/`EntityOpLeft` events with sample replay
3. ⬜ Determine if events are player-specific or global
4. ⬜ Extract team/player assignments for all hero entities
5. ⬜ Build prototype vision tracker
6. ⬜ Validate against combat log visibility flags
7. ⬜ Handle edge cases (invisibility, wards, smoke)

---

**Last Updated:** 2025-12-01
**Status:** Research & Documentation Phase
