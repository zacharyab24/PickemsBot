import requests
import mwparserfromhell
import re

def get_matchlist_ids(page_title):
    # Format the URL
    base_url = "https://liquipedia.net/counterstrike/"
    full_url = f"{base_url}{page_title}?action=raw"
    print(f"Fetching wikitext from: {full_url}")

    # Set headers to comply with Liquipedia API requirements
    headers = {
        "User-Agent": "LiquipediaDataFetcher/1.0 (your_email@example.com)",
        "Accept-Encoding": "gzip"
    }

    response = requests.get(full_url, headers=headers)
    if response.status_code != 200:
        print(f"Failed to fetch page. Status code: {response.status_code}")
        return []

    # Parse wikitext using mwparserfromhell
    wikicode = mwparserfromhell.parse(response.text)
    templates = wikicode.filter_templates()

    ids = []

    for template in templates:
        if template.name.matches("Matchlist"):
            id_param = template.get("id").value.strip() if template.has("id") else None
            if id_param:
                ids.append(str(id_param))

    return ids

# Example usage
page = "PGL/2024/Copenhagen/Opening_Stage"
ids = get_matchlist_ids(page)

if ids:
    print(f"Found {len(ids)} matchlist IDs:")
    for id_ in ids:
        print(f"- {id_}")
else:
    print("No matchlist IDs found.")
