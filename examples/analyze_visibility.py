#!/usr/bin/env python3
"""
Analyze Dota 2 visibility data from visibility.json
Creates a DataFrame with ticks as rows and heroes as columns
"""

import json
import pandas as pd
from pathlib import Path


def load_visibility_data(json_path='visibility.json'):
    """Load visibility data from JSON file"""
    with open(json_path, 'r') as f:
        data = json.load(f)
    return data['visible']


def extract_hero_name(hero_key):
    """Extract clean hero name from entity key like 'CDOTA_Unit_Hero_Abaddon_1292'"""
    # Remove the CDOTA_Unit_Hero_ prefix and the _XXXX suffix (entity ID)
    parts = hero_key.split('_')
    if len(parts) >= 4:
        # Find where the entity ID starts (last numeric part)
        hero_name = '_'.join(parts[3:-1])  # Skip CDOTA_Unit_Hero and entity ID
        return hero_name if hero_name else parts[-2]  # Fallback to second-to-last part
    return hero_key


def create_visibility_dataframe(visibility_data):
    """
    Create a pandas DataFrame from visibility data

    Returns:
        DataFrame with:
        - Index: tick number
        - Columns: time (seconds) + hero names (team2_HeroName and team3_HeroName)
        - Values: time (float) and boolean visibility (True if visible to enemy team)
    """
    # Collect all unique heroes across all ticks
    all_heroes = set()

    for tick_str, tick_data in visibility_data.items():
        for team in ['team2', 'team3']:
            if team in tick_data:
                for hero_key in tick_data[team].keys():
                    hero_name = extract_hero_name(hero_key)
                    all_heroes.add(f"{team}_{hero_name}")

    # Sort heroes for consistent column ordering
    sorted_heroes = sorted(all_heroes)

    # Create dictionary to build DataFrame
    data_dict = {'tick': [], 'time': []}
    for hero in sorted_heroes:
        data_dict[hero] = []

    # Process each tick
    for tick_str, tick_data in sorted(visibility_data.items(), key=lambda x: int(x[0])):
        tick = int(tick_str)
        data_dict['tick'].append(tick)

        # Extract time from tick data
        time = tick_data.get('time', tick / 30.0)  # Fallback to calculation if not present
        data_dict['time'].append(time)

        # For each hero column, check if they're visible
        for hero_col in sorted_heroes:
            team, hero_name = hero_col.split('_', 1)

            visible = False
            if team in tick_data:
                # Find this hero in the tick data
                for hero_key, hero_vis in tick_data[team].items():
                    if extract_hero_name(hero_key) == hero_name:
                        visible = hero_vis['visible']
                        break

            data_dict[hero_col].append(visible)

    # Create DataFrame
    df = pd.DataFrame(data_dict)
    df.set_index('tick', inplace=True)

    return df


def print_summary(df):
    """Print summary statistics about the visibility data"""
    print("\n" + "="*80)
    print("VISIBILITY ANALYSIS SUMMARY")
    print("="*80)

    # Count hero columns (excluding 'time' column)
    hero_cols = [col for col in df.columns if col not in ['time']]
    print(f"\nDataFrame Shape: {df.shape[0]} ticks × {len(hero_cols)} heroes (+ time column)")
    print(f"Tick Range: {df.index.min()} to {df.index.max()}")

    # Use actual time column data
    if 'time' in df.columns:
        print(f"Game Time: {df['time'].min():.1f}s to {df['time'].max():.1f}s ({df['time'].max()/60:.1f} minutes)")
    else:
        print(f"Game Time: {df.index.min()/30:.1f}s to {df.index.max()/30:.1f}s ({df.index.max()/1800:.1f} minutes)")

    print("\n" + "-"*80)
    print("VISIBILITY STATISTICS BY HERO")
    print("-"*80)

    # Calculate visibility percentage for each hero (exclude 'time' column)
    hero_cols = [col for col in df.columns if col not in ['time']]
    visibility_pct = (df[hero_cols].sum() / len(df) * 100).sort_values(ascending=False)

    for hero, pct in visibility_pct.items():
        visible_count = df[hero].sum()
        total_count = len(df)
        print(f"{hero:40s}: {visible_count:5d}/{total_count:5d} ticks visible ({pct:5.1f}%)")

    print("\n" + "-"*80)
    print("TEAM STATISTICS")
    print("-"*80)

    team2_cols = [col for col in df.columns if col.startswith('team2_')]
    team3_cols = [col for col in df.columns if col.startswith('team3_')]

    if team2_cols:
        team2_avg = df[team2_cols].mean().mean() * 100
        print(f"Team 2 Average Visibility: {team2_avg:.1f}%")

    if team3_cols:
        team3_avg = df[team3_cols].mean().mean() * 100
        print(f"Team 3 Average Visibility: {team3_avg:.1f}%")

    print("\n" + "-"*80)
    print("SAMPLE DATA (first 10 ticks)")
    print("-"*80)
    print(df.head(10))

    print("\n")


def main():
    """Main function"""
    print("Loading visibility data...")

    # Check if visibility.json exists
    json_path = Path('visibility.json')
    if not json_path.exists():
        print(f"Error: {json_path} not found!")
        print("Please run vision.go first to generate the visibility data.")
        return

    # Load and process data
    visibility_data = load_visibility_data(json_path)
    print(f"Loaded visibility data for {len(visibility_data)} ticks")

    # Create DataFrame
    print("Creating DataFrame...")
    df = create_visibility_dataframe(visibility_data)

    # Print summary
    print_summary(df)

    # Save to CSV
    csv_path = 'visibility_dataframe.csv'
    print(f"Saving DataFrame to {csv_path}...")
    df.to_csv(csv_path)
    print(f"[OK] Saved to {csv_path}")

    # Save to pickle for faster loading
    pickle_path = 'visibility_dataframe.pkl'
    print(f"Saving DataFrame to {pickle_path}...")
    df.to_pickle(pickle_path)
    print(f"[OK] Saved to {pickle_path}")

    print("\n" + "="*80)
    print("DONE! You can now use the DataFrame:")
    print("  - Load from CSV: df = pd.read_csv('visibility_dataframe.csv', index_col='tick')")
    print("  - Load from pickle: df = pd.read_pickle('visibility_dataframe.pkl')")
    print("="*80)

    return df


if __name__ == '__main__':
    df = main()
