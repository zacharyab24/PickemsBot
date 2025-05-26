import json
import os
import requests
from dotenv import load_dotenv

load_dotenv()

API_KEY = os.getenv('LIQUIDPEDIADB_API_KEY')

bracket_ids = {
    "25lqANNoNV",
    "HDFMCYPolL",
    "n8zf3vI5ha",
    "PpHDgrEvSg",
    "iOiwJxPcN4",
    "WyqNMd1lBx",
    "vwWaUjSEWk",
    "XgACJYwHG9",
    "QwpPeVu7cK"
}

conditions_string = " OR ".join([f"[[match2bracketid::{bid}]]" for bid in bracket_ids])

# Define the endpoint and parameters
url = "https://api.liquipedia.net/api/v3/match"
params = {
    "limit": 100,
    "wiki": "counterstrike",
    "conditions": conditions_string,
    "rawstreams": "false",
    "streamurls": "false"
}

headers = {
    "Authorization": f"Apikey {API_KEY}",
}

# Send GET request
response = requests.get(url, headers=headers, params=params)

# Check response and print results
if response.status_code == 200:
    data = response.json()
    print("Success. Number of matches:", len(data.get("result", [])))
else:
    print("Error:", response.status_code, response.text)

# Extract match info
matches = data.get("result", [])

# Display results

wins = {}
loses = {}
teams = []

for match in matches:
    match_id = match.get("match2id")
    opponents = match.get("match2opponents", [])
    winner = match.get("winner", "")
    finished = match.get("finished", 0)
    
    # Skip unfinished matches or matches without a clear winner
    if not finished or not winner:
        continue
    
    # Skip matches that don't have exactly 2 opponents
    if len(opponents) != 2:
        continue
    
    # Initialize teams if not seen before
    for team in opponents:
        name = team.get("name", "Unknown")
        if name not in teams:
            teams.append(name)
            wins[name] = 0
            loses[name] = 0
    
    # Get both teams
    team1 = opponents[0]
    team2 = opponents[1]
    team1_name = team1.get("name", "Unknown")
    team2_name = team2.get("name", "Unknown")
    
    # Determine winner based on the "winner" field
    # winner field contains "1" for first opponent, "2" for second opponent
    try:
        winner_index = int(winner)
        if winner_index == 1:
            wins[team1_name] += 1
            loses[team2_name] += 1
        elif winner_index == 2:
            wins[team2_name] += 1
            loses[team1_name] += 1
        else:
            print(f"Unexpected winner value '{winner}' for match {match_id}")
            
    except (ValueError, TypeError):
        print(f"Skipping match {match_id} due to invalid winner field: {winner}")
        continue

# Sort teams by their record (wins first, then by losses)
def sort_key(team):
    return (-wins[team], loses[team])

sorted_teams = sorted(teams, key=sort_key)

# Print results
print("\nTeam Records (sorted by performance):")
for team in sorted_teams:
    print(f"{team}: {wins[team]}-{loses[team]}")
