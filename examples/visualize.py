import json
import plotly.graph_objects as go
from plotly.subplots import make_subplots
import sys
import webbrowser
import os

def load_data(filename="visibility_and_pos.json"):
    """Load the visibility and position data from JSON file."""
    with open(filename, 'r') as f:
        data = json.load(f)
    return data

def sample_ticks(ticks_dict, sample_rate=30):
    """Sample every Nth tick to reduce data size."""
    # Get sorted tick numbers
    all_ticks = sorted([int(tick) for tick in ticks_dict.keys()])

    # Sample every sample_rate ticks
    sampled_ticks = all_ticks[::sample_rate]

    return sampled_ticks

def create_visualization(filename="visibility_and_pos.json", sample_rate=30):
    """Create an interactive plotly visualization of hero positions."""
    print(f"Loading data from {filename}...")
    data = load_data(filename)
    ticks = data['ticks']

    print(f"Total ticks: {len(ticks)}")

    # Find first tick with actual hero data
    all_tick_nums = sorted([int(tick) for tick in ticks.keys()])
    first_data_tick = None
    for tick_num in all_tick_nums:
        if len(ticks[str(tick_num)]['heroes']) > 0:
            first_data_tick = tick_num
            break

    if first_data_tick is None:
        print("No hero data found in any ticks!")
        return None

    print(f"First tick with hero data: {first_data_tick} (time: {ticks[str(first_data_tick)]['time']:.1f}s)")

    # Filter ticks to only include those from first_data_tick onwards
    filtered_ticks = [t for t in all_tick_nums if t >= first_data_tick]

    # Sample ticks from the filtered list
    sampled_tick_nums = filtered_ticks[::sample_rate]
    print(f"Sampled ticks: {len(sampled_tick_nums)} (every {sample_rate} frames, starting from tick {first_data_tick})")

    # Prepare data for animation
    frames = []

    # Track unique heroes across all ticks
    all_heroes = set()
    for tick_num in sampled_tick_nums:
        tick_data = ticks[str(tick_num)]
        all_heroes.update(tick_data['heroes'].keys())

    print(f"Total unique heroes: {len(all_heroes)}")

    # Create frames for animation
    for tick_num in sampled_tick_nums:
        tick_str = str(tick_num)
        tick_data = ticks[tick_str]
        heroes = tick_data['heroes']
        time = tick_data['time']

        # Separate by team
        team2_x, team2_y, team2_names, team2_visible = [], [], [], []
        team3_x, team3_y, team3_names, team3_visible = [], [], [], []

        for hero_key, hero_data in heroes.items():
            hero_name = hero_key.split('_')
            if len(hero_name) >= 4:
                hero_name = '_'.join(hero_name[3:-2])  # Extract hero name
            else:
                hero_name = hero_key

            x = hero_data['pos_x']
            y = hero_data['pos_y']
            team = hero_data['team']
            visible = hero_data['visible']
            seen_by = hero_data.get('seen_by', [])

            hover_text = f"{hero_name}<br>Pos: ({x:.1f}, {y:.1f})<br>Visible: {visible}"
            if seen_by:
                hover_text += f"<br>Seen by: {', '.join(seen_by)}"

            if team == 2:
                team2_x.append(x)
                team2_y.append(y)
                team2_names.append(hover_text)
                team2_visible.append(visible)
            elif team == 3:
                team3_x.append(x)
                team3_y.append(y)
                team3_names.append(hover_text)
                team3_visible.append(visible)

        # Create frame
        frame = go.Frame(
            data=[
                # Team 2 (visible)
                go.Scatter(
                    x=[team2_x[i] for i in range(len(team2_x)) if team2_visible[i]],
                    y=[team2_y[i] for i in range(len(team2_y)) if team2_visible[i]],
                    mode='markers+text',
                    marker=dict(size=15, color='green', symbol='circle', line=dict(width=2, color='darkgreen')),
                    text=[team2_names[i].split('<br>')[0] for i in range(len(team2_names)) if team2_visible[i]],
                    textposition='top center',
                    textfont=dict(size=8),
                    hovertext=[team2_names[i] for i in range(len(team2_names)) if team2_visible[i]],
                    hoverinfo='text',
                    name='Team 2 (Visible)',
                    showlegend=True
                ),
                # Team 2 (invisible)
                go.Scatter(
                    x=[team2_x[i] for i in range(len(team2_x)) if not team2_visible[i]],
                    y=[team2_y[i] for i in range(len(team2_y)) if not team2_visible[i]],
                    mode='markers+text',
                    marker=dict(size=15, color='lightgreen', symbol='circle-open', line=dict(width=2, color='green')),
                    text=[team2_names[i].split('<br>')[0] for i in range(len(team2_names)) if not team2_visible[i]],
                    textposition='top center',
                    textfont=dict(size=8),
                    hovertext=[team2_names[i] for i in range(len(team2_names)) if not team2_visible[i]],
                    hoverinfo='text',
                    name='Team 2 (Invisible)',
                    showlegend=True
                ),
                # Team 3 (visible)
                go.Scatter(
                    x=[team3_x[i] for i in range(len(team3_x)) if team3_visible[i]],
                    y=[team3_y[i] for i in range(len(team3_y)) if team3_visible[i]],
                    mode='markers+text',
                    marker=dict(size=15, color='red', symbol='circle', line=dict(width=2, color='darkred')),
                    text=[team3_names[i].split('<br>')[0] for i in range(len(team3_names)) if team3_visible[i]],
                    textposition='top center',
                    textfont=dict(size=8),
                    hovertext=[team3_names[i] for i in range(len(team3_names)) if team3_visible[i]],
                    hoverinfo='text',
                    name='Team 3 (Visible)',
                    showlegend=True
                ),
                # Team 3 (invisible)
                go.Scatter(
                    x=[team3_x[i] for i in range(len(team3_x)) if not team3_visible[i]],
                    y=[team3_y[i] for i in range(len(team3_y)) if not team3_visible[i]],
                    mode='markers+text',
                    marker=dict(size=15, color='lightcoral', symbol='circle-open', line=dict(width=2, color='red')),
                    text=[team3_names[i].split('<br>')[0] for i in range(len(team3_names)) if not team3_visible[i]],
                    textposition='top center',
                    textfont=dict(size=8),
                    hovertext=[team3_names[i] for i in range(len(team3_names)) if not team3_visible[i]],
                    hoverinfo='text',
                    name='Team 3 (Invisible)',
                    showlegend=True
                )
            ],
            name=f"Tick {tick_num}",
            layout=go.Layout(
                title=f"Dota 2 Hero Positions - Tick {tick_num} (Time: {time:.1f}s / {time/60:.1f}m)",
                annotations=[
                    dict(
                        text=f"Tick: {tick_num} | Time: {time:.1f}s ({time/60:.1f}m)",
                        xref="paper", yref="paper",
                        x=0.5, y=1.05, showarrow=False,
                        font=dict(size=14, color="black")
                    )
                ]
            )
        )
        frames.append(frame)

    # Create initial frame (first sampled tick)
    first_tick_num = sampled_tick_nums[0]
    first_tick_data = ticks[str(first_tick_num)]
    initial_heroes = first_tick_data['heroes']

    team2_x, team2_y, team2_names, team2_visible = [], [], [], []
    team3_x, team3_y, team3_names, team3_visible = [], [], [], []

    for hero_key, hero_data in initial_heroes.items():
        hero_name = hero_key.split('_')
        if len(hero_name) >= 4:
            hero_name = '_'.join(hero_name[3:-2])
        else:
            hero_name = hero_key

        x = hero_data['pos_x']
        y = hero_data['pos_y']
        team = hero_data['team']
        visible = hero_data['visible']
        seen_by = hero_data.get('seen_by', [])

        hover_text = f"{hero_name}<br>Pos: ({x:.1f}, {y:.1f})<br>Visible: {visible}"
        if seen_by:
            hover_text += f"<br>Seen by: {', '.join(seen_by)}"

        if team == 2:
            team2_x.append(x)
            team2_y.append(y)
            team2_names.append(hover_text)
            team2_visible.append(visible)
        elif team == 3:
            team3_x.append(x)
            team3_y.append(y)
            team3_names.append(hover_text)
            team3_visible.append(visible)

    # Create figure with initial data
    fig = go.Figure(
        data=[
            go.Scatter(
                x=[team2_x[i] for i in range(len(team2_x)) if team2_visible[i]],
                y=[team2_y[i] for i in range(len(team2_y)) if team2_visible[i]],
                mode='markers+text',
                marker=dict(size=15, color='green', symbol='circle', line=dict(width=2, color='darkgreen')),
                text=[team2_names[i].split('<br>')[0] for i in range(len(team2_names)) if team2_visible[i]],
                textposition='top center',
                textfont=dict(size=8),
                hovertext=[team2_names[i] for i in range(len(team2_names)) if team2_visible[i]],
                hoverinfo='text',
                name='Team 2 (Visible)'
            ),
            go.Scatter(
                x=[team2_x[i] for i in range(len(team2_x)) if not team2_visible[i]],
                y=[team2_y[i] for i in range(len(team2_y)) if not team2_visible[i]],
                mode='markers+text',
                marker=dict(size=15, color='lightgreen', symbol='circle-open', line=dict(width=2, color='green')),
                text=[team2_names[i].split('<br>')[0] for i in range(len(team2_names)) if not team2_visible[i]],
                textposition='top center',
                textfont=dict(size=8),
                hovertext=[team2_names[i] for i in range(len(team2_names)) if not team2_visible[i]],
                hoverinfo='text',
                name='Team 2 (Invisible)'
            ),
            go.Scatter(
                x=[team3_x[i] for i in range(len(team3_x)) if team3_visible[i]],
                y=[team3_y[i] for i in range(len(team3_y)) if team3_visible[i]],
                mode='markers+text',
                marker=dict(size=15, color='red', symbol='circle', line=dict(width=2, color='darkred')),
                text=[team3_names[i].split('<br>')[0] for i in range(len(team3_names)) if team3_visible[i]],
                textposition='top center',
                textfont=dict(size=8),
                hovertext=[team3_names[i] for i in range(len(team3_names)) if team3_visible[i]],
                hoverinfo='text',
                name='Team 3 (Visible)'
            ),
            go.Scatter(
                x=[team3_x[i] for i in range(len(team3_x)) if not team3_visible[i]],
                y=[team3_y[i] for i in range(len(team3_y)) if not team3_visible[i]],
                mode='markers+text',
                marker=dict(size=15, color='lightcoral', symbol='circle-open', line=dict(width=2, color='red')),
                text=[team3_names[i].split('<br>')[0] for i in range(len(team3_names)) if not team3_visible[i]],
                textposition='top center',
                textfont=dict(size=8),
                hovertext=[team3_names[i] for i in range(len(team3_names)) if not team3_visible[i]],
                hoverinfo='text',
                name='Team 3 (Invisible)'
            )
        ],
        frames=frames,
        layout=go.Layout(
            title=f"Dota 2 Hero Positions - Tick {first_tick_num}",
            xaxis=dict(title="X Position", range=[8000, 24000]),
            yaxis=dict(title="Y Position", range=[9000, 24000], scaleanchor="x", scaleratio=1),
            hovermode='closest',
            updatemenus=[
                dict(
                    type="buttons",
                    showactive=False,
                    buttons=[
                        dict(label="Play",
                             method="animate",
                             args=[None, {"frame": {"duration": 100, "redraw": True},
                                        "fromcurrent": True,
                                        "transition": {"duration": 0}}]),
                        dict(label="Pause",
                             method="animate",
                             args=[[None], {"frame": {"duration": 0, "redraw": False},
                                          "mode": "immediate",
                                          "transition": {"duration": 0}}])
                    ],
                    x=0.1,
                    y=1.15
                )
            ],
            sliders=[
                dict(
                    active=0,
                    yanchor="top",
                    y=-0.1,
                    xanchor="left",
                    currentvalue=dict(
                        prefix="Tick: ",
                        visible=True,
                        xanchor="right"
                    ),
                    pad=dict(b=10, t=50),
                    len=0.9,
                    x=0.1,
                    steps=[
                        dict(
                            args=[[f.name], {"frame": {"duration": 100, "redraw": True},
                                           "mode": "immediate",
                                           "transition": {"duration": 0}}],
                            label=str(sampled_tick_nums[i]),
                            method="animate"
                        )
                        for i, f in enumerate(frames)
                    ]
                )
            ],
            width=1200,
            height=1200
        )
    )

    print("Saving visualization to hero_positions.html...")
    output_file = "hero_positions.html"
    fig.write_html(output_file)
    print("Done! Opening in browser...")

    # Open in browser automatically
    file_path = os.path.abspath(output_file)
    webbrowser.open('file://' + file_path)

    return fig

if __name__ == "__main__":
    # Check if custom sample rate is provided
    sample_rate = 30
    if len(sys.argv) > 1:
        try:
            sample_rate = int(sys.argv[1])
            print(f"Using custom sample rate: {sample_rate}")
        except ValueError:
            print(f"Invalid sample rate, using default: {sample_rate}")

    create_visualization(sample_rate=sample_rate)
