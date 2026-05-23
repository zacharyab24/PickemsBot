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
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"pickems-bot/sources"
	"pickems-bot/store"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: fetchtest <liquipedia|pandascore> <page|seriesID> <round>\n")
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
		fetcher = store.NewLiquipediaFetcher(apiKey, arg)

	case "pandascore":
		apiKey := os.Getenv("PANDASCORE_API_KEY")
		if apiKey == "" {
			log.Fatal("PANDASCORE_API_KEY not set")
		}
		seriesID, err := strconv.Atoi(arg)
		if err != nil {
			log.Fatalf("seriesID must be an integer, got %q", arg)
		}
		fetcher = store.NewPandaScoreFetcher(apiKey, seriesID)

	default:
		log.Fatalf("unknown source %q — use 'liquipedia' or 'pandascore'", source)
	}

	fmt.Printf("=== FetchMatchData (round=%q) ===\n", round)
	_, matchNodes, err := fetcher.FetchMatchData(round)
	if err != nil {
		log.Fatalf("FetchMatchData error: %v", err)
	}
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
	fmt.Printf("  %-12s %-25s %-25s %-25s %-8s %s\n", "ID", "Team1", "Team2", "Winner", "Score", "Status")
	fmt.Printf("  %-12s %-25s %-25s %-25s %-8s %s\n", "---", "-----", "-----", "------", "-----", "------")
	for _, n := range nodes {
		fmt.Printf("  %-12s %-25s %-25s %-25s %-8s %s\n", n.ID[:min(12, len(n.ID))], n.Team1, n.Team2, n.Winner, n.Score, n.Status)
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
