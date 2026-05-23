//go:build !test

/* scripts/configure
 * Generates config.toml from a tournament URL or PandaScore series ID.
 *
 * Liquipedia usage:
 *   go run ./scripts/configure -source liquipedia -url https://liquipedia.net/counterstrike/Foo/2026/Bar
 *   go run ./scripts/configure -source liquipedia -url <url> -stage Stage_1
 *   go run ./scripts/configure -source liquipedia -url <url> -out custom.toml
 *
 * PandaScore usage:
 *   go run ./scripts/configure -source pandascore -series-id 10488 -name "IEM_Cologne_2026" -round "Stage_1"
 */

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	source := flag.String("source", "liquipedia", `Data source: "liquipedia" or "pandascore"`)
	out := flag.String("out", "config.toml", "Output config file path")

	// Liquipedia flags
	tourneyURL := flag.String("url", "", "Liquipedia tournament URL (liquipedia source only)")
	stage := flag.String("stage", "", `Stage name to use, skips interactive picker (liquipedia source only)`)
	format := flag.String("format", "", `Format override: "swiss" or "single-elimination" (liquipedia source only)`)

	// PandaScore flags
	seriesId := flag.Int("series-id", 0, "PandaScore series ID (pandascore source only)")
	name := flag.String("name", "", "Tournament name, used as MongoDB database name (pandascore source only)")
	round := flag.String("round", "", "Round/stage name (pandascore source only)")

	flag.Parse()

	var cfg tournamentConfig

	switch *source {
	case "liquipedia":
		cfg = runLiquipedia(*tourneyURL, *stage, *format)

	case "pandascore":
		cfg = runPandaScore(*seriesId, *name, *round)

	default:
		log.Fatalf("unknown -source %q — use \"liquipedia\" or \"pandascore\"", *source)
	}

	if err := writeConfig(*out, cfg); err != nil {
		log.Fatalf("write config: %v", err)
	}
	fmt.Printf("\nWrote %s:\n", *out)
	fmt.Printf("  data_source     = %q\n", cfg.DataSource)
	fmt.Printf("  tournament_name = %q\n", cfg.Name)
	fmt.Printf("  round           = %q\n", cfg.Round)
	switch cfg.DataSource {
	case "liquipedia":
		fmt.Printf("  page            = %q\n", cfg.Page)
	case "pandascore":
		fmt.Printf("  series_id       = %d\n", cfg.SeriesId)
	}
}

func runLiquipedia(tourneyURL, stage, format string) tournamentConfig {
	if tourneyURL == "" {
		log.Fatal("missing required flag: -url")
	}

	basePath, err := pagePathFromURL(tourneyURL)
	if err != nil {
		log.Fatalf("invalid URL: %v", err)
	}

	fmt.Printf("Fetching %s...\n", basePath)
	wikitext, err := fetchWikitext(basePath)
	if err != nil {
		log.Fatalf("fetch wikitext: %v", err)
	}

	tournamentName := extractTournamentName(wikitext)
	if tournamentName == "" {
		log.Fatal("could not extract tournament name from wikitext (|name= field missing)")
	}
	fmt.Printf("Tournament: %s\n", tournamentName)

	stages := extractStages(wikitext, basePath)
	chosenStage, chosenPage, err := resolveStage(stage, stages, basePath, os.Stdin, os.Stdout)
	if err != nil {
		log.Fatalf("resolve stage: %v", err)
	}

	if chosenPage != basePath {
		fmt.Printf("Validating %s...\n", chosenPage)
		if _, err := fetchWikitext(chosenPage); err != nil {
			log.Fatalf("chosen stage page does not load: %v", err)
		}
	}

	return tournamentConfig{
		DataSource: "liquipedia",
		Name:       mongoSafe(tournamentName),
		Page:       chosenPage,
		Round:      chosenStage,
		Format:     format,
	}
}

func runPandaScore(seriesId int, name, round string) tournamentConfig {
	if seriesId == 0 {
		log.Fatal("missing required flag: -series-id")
	}
	if name == "" {
		log.Fatal("missing required flag: -name")
	}
	if round == "" {
		log.Fatal("missing required flag: -round")
	}

	return tournamentConfig{
		DataSource: "pandascore",
		Name:       mongoSafe(name),
		SeriesId:   seriesId,
		Round:      round,
	}
}
