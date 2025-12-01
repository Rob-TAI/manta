import json
import numpy as np

# load hero_sequences.json

with open("hero_sequences.json", "r") as f:
    data = json.load(f)

# print keys
print(data.keys())
