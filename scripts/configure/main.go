//go:build !test

/* scripts/configure
 * Generates config.toml. PandaScore tournaments are chosen per channel with
 * /config; Liquipedia needs the tournament page for its webhook server.
 *
 * PandaScore usage:
 *   go run ./scripts/configure -source pandascore
 *
 * Liquipedia usage:
 *   go run ./scripts/configure -source liquipedia -url https://liquipedia.net/counterstrike/Foo/2026/Bar
 *   go run ./scripts/configure -source liquipedia -url <url> -stage Stage_1
 *   go run ./scripts/configure -source liquipedia -url <url> -out custom.toml
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

	tourneyURL := flag.String("url", "", "Liquipedia tournament URL (liquipedia source only)")
	stage := flag.String("stage", "", `Stage name to use, skips interactive picker (liquipedia source only)`)

	flag.Parse()

	var cfg tournamentConfig

	switch *source {
	case "liquipedia":
		cfg = runLiquipedia(*tourneyURL, *stage)

	case "pandascore":
		cfg = tournamentConfig{DataSource: "pandascore"}

	default:
		log.Fatalf("unknown -source %q — use \"liquipedia\" or \"pandascore\"", *source)
	}

	if err := writeConfig(*out, cfg); err != nil {
		log.Fatalf("write config: %v", err)
	}
	fmt.Printf("\nWrote %s:\n", *out)
	fmt.Printf("  data_source     = %q\n", cfg.DataSource)
	if cfg.DataSource == "liquipedia" {
		fmt.Printf("  tournament_name = %q\n", cfg.Name)
		fmt.Printf("  round           = %q\n", cfg.Round)
		fmt.Printf("  page            = %q\n", cfg.Page)
	}
}

func runLiquipedia(tourneyURL, stage string) tournamentConfig {
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
		Name:       tournamentName,
		Page:       chosenPage,
		Round:      chosenStage,
	}
}
