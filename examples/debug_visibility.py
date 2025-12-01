#!/usr/bin/env python3
"""
Debug visibility calculation at the first frame
"""

import json
import math

# Load the data
with open('visibility_and_pos.json', 'r') as f:
    data = json.load(f)

# Get the earliest tick
ticks = sorted([int(k) for k in data['ticks'].keys()])
earliest_tick = str(ticks[0])
print(f"Analyzing tick {earliest_tick} (time: {data['ticks'][earliest_tick]['time']:.2f}s)")
print("="*80)

# Get heroes
heroes = data['ticks'][earliest_tick]['heroes']

# Load original data to get tower positions
# We need to parse the replay to get tower positions, but for now let's analyze what we have

for hero_key, hero_data in heroes.items():
    print(f"\n{hero_key}:")
    print(f"  Position: ({hero_data['m_cellX']}, {hero_data['m_cellY']}, {hero_data['m_cellZ']})")
    print(f"  Team: {hero_data['team']}")
    print(f"  Visible: {hero_data['visible']}")
    print(f"  Seen by {len(hero_data['seen_by'])} entities:")
    for entity in hero_data['seen_by'][:5]:  # Show first 5
        print(f"    - {entity}")
    if len(hero_data['seen_by']) > 5:
        print(f"    ... and {len(hero_data['seen_by']) - 5} more")
