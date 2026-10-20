//go:build !test

// fetchtest is a manual smoke-test tool for verifying both data sources
// return well-formed match data before running the full bot.
//
// Usage:
//
//	go run ./scripts/fetchtest liquipedia  <page>     <round>
//	go run ./scripts/fetchtest pandascore  <seriesID> <round>
//
// Examples:
//
//	go run ./scripts/fetchtest liquipedia CCT/2026/Europe/Series_2 "Playoffs"
//	go run ./scripts/fetchtest pandascore 10607 "Playoffs"
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"pickems-bot/sources"
	"pickems-bot/store"
	"pickems-bot/tournament"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: fetchtest <liquipedia|pandascore> <page|seriesID> <round> [pandascoreTournamentID]\n")
		os.Exit(1)
	}

	source := os.Args[1]
	arg := os.Args[2]
	round := os.Args[3]

	var fetcher store.DataSourceFetcher

	switch source {
	case "liquipedia":
		apiKey := os.Getenv("LIQUIDPEDIADB_API_KEY")
		if apiKey == "" {
			log.Fatal("LIQUIDPEDIADB_API_KEY not set")
		}
		fetcher = store.NewLiquipediaFetcher("https://api.liquipedia.net/api/v3/match", apiKey, arg)

	case "pandascore":
		apiKey := os.Getenv("PANDASCORE_API_KEY")
		if apiKey == "" {
			log.Fatal("PANDASCORE_API_KEY not set")
		}
		seriesID, err := strconv.Atoi(arg)
		if err != nil {
			log.Fatalf("seriesID must be an integer, got %q", arg)
		}
		tournamentID := 0
		if len(os.Args) > 4 {
			if tournamentID, err = strconv.Atoi(os.Args[4]); err != nil {
				log.Fatalf("tournamentID must be an integer, got %q", os.Args[4])
			}
		}
		fetcher = store.NewPandaScoreFetcher("https://api.pandascore.co/csgo/matches", apiKey, seriesID, tournamentID)
		if tournamentID == 0 {
			printSeriesTournaments(apiKey, seriesID)
		} else {
			printBracket(apiKey, tournamentID)
		}

	default:
		log.Fatalf("unknown source %q — use 'liquipedia' or 'pandascore'", source)
	}

	fmt.Printf("=== FetchMatchData (round=%q) ===\n", round)
	result, matchNodes, kind, err := fetcher.FetchMatchData(round, "")
	if err != nil {
		log.Fatalf("FetchMatchData error: %v", err)
	}
	fmt.Printf("kind: %s, result: %v\n", kind, result != nil)
	printMatchNodes(matchNodes)

	fmt.Printf("\n=== FetchSchedule ===\n")
	schedule, err := fetcher.FetchSchedule()
	if err != nil {
		log.Fatalf("FetchSchedule error: %v", err)
	}
	printSchedule(schedule)
}

func printMatchNodes(nodes []sources.MatchNode) {
	if len(nodes) == 0 {
		fmt.Println("  (no matches)")
		return
	}
	fmt.Printf("  %-12s %-25s %-25s %-25s %-8s %-12s %s\n", "ID", "Team1", "Team2", "Winner", "Score", "Status", "Section")
	fmt.Printf("  %-12s %-25s %-25s %-25s %-8s %-12s %s\n", "---", "-----", "-----", "------", "-----", "------", "-------")
	for _, n := range nodes {
		fmt.Printf("  %-12s %-25s %-25s %-25s %-8s %-12s %s\n", n.ID[:min(12, len(n.ID))], n.Team1, n.Team2, n.Winner, n.Score, n.Status, n.Section)
	}
	fmt.Printf("  Total: %d matches\n", len(nodes))
}

func printSchedule(matches []sources.ScheduledMatch) {
	if len(matches) == 0 {
		fmt.Println("  (no scheduled matches)")
		return
	}
	fmt.Printf("  %-25s %-25s %-12s %-6s %-8s %s\n", "Team1", "Team2", "EpochTime", "BestOf", "Finished", "StreamURL")
	fmt.Printf("  %-25s %-25s %-12s %-6s %-8s %s\n", "-----", "-----", "---------", "------", "--------", "---------")
	for _, m := range matches {
		stream := m.StreamURL
		if stream == "" {
			stream = "(none)"
		}
		fmt.Printf("  %-25s %-25s %-12d %-6s %-8v %s\n", m.Team1, m.Team2, m.EpochTime, m.BestOf, m.Finished, stream)
	}
	fmt.Printf("  Total: %d matches\n", len(matches))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// printSeriesTournaments lists the tournaments in a series, to pick one to pass as pandascoreTournamentID.
func printSeriesTournaments(apiKey string, seriesID int) {
	raw, err := sources.GetPandaScoreMatches("https://api.pandascore.co/csgo/matches", apiKey, seriesID, 0)
	if err != nil {
		log.Fatalf("GetPandaScoreMatches error: %v", err)
	}
	var matches []struct {
		TournamentID int `json:"tournament_id"`
		Tournament   struct {
			Name string `json:"name"`
		} `json:"tournament"`
	}
	if err := json.Unmarshal([]byte(raw), &matches); err != nil {
		log.Fatalf("decode matches: %v", err)
	}
	counts := map[int]int{}
	names := map[int]string{}
	var order []int
	for _, m := range matches {
		if counts[m.TournamentID] == 0 {
			order = append(order, m.TournamentID)
		}
		counts[m.TournamentID]++
		names[m.TournamentID] = m.Tournament.Name
	}
	fmt.Println("=== Tournaments in series ===")
	for _, id := range order {
		fmt.Printf("  %-10d %-30s %d matches\n", id, names[id], counts[id])
	}
	fmt.Println()
}

// printBracket shows the bracket feeder edges and the format checkAndStoreFormat would store.
func printBracket(apiKey string, tournamentID int) {
	bracket, err := sources.GetPandaScoreBracket(apiKey, tournamentID)
	if err != nil {
		log.Fatalf("GetPandaScoreBracket error: %v", err)
	}
	fmt.Println("=== Bracket ===")
	for _, m := range bracket {
		var teams []string
		for _, o := range m.Opponents {
			if o.Opponent != nil {
				teams = append(teams, o.Opponent.Name)
			}
		}
		fmt.Printf("  %-10d %-40v fed by %+v\n", m.ID, teams, m.PreviousMatches)
	}
	fmt.Printf("detected kind: %s\n\n", tournament.DetectKindFromBracket(bracket, sources.CountTeams(bracket)))
}
