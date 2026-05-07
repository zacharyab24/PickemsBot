//go:build !test

/* scripts/configure
 * Generates config.toml from a Liquipedia tournament URL by parsing the
 * page wikitext for the tournament name and available sub-stages.
 *
 * Usage:
 *   go run ./scripts/configure -url https://liquipedia.net/counterstrike/Foo/2026/Bar
 *   go run ./scripts/configure -url <url> -stage Stage_1
 *   go run ./scripts/configure -url <url> -out custom.toml
 */

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	tourneyURL := flag.String("url", "", "Liquipedia tournament URL (required)")
	stage := flag.String("stage", "", "Stage name to use (skips interactive picker)")
	out := flag.String("out", "config.toml", "Output config file path")
	flag.Parse()

	if *tourneyURL == "" {
		log.Fatal("missing required flag: -url")
	}

	basePath, err := pagePathFromURL(*tourneyURL)
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

	chosenStage, chosenPage, err := resolveStage(*stage, stages, basePath, os.Stdin, os.Stdout)
	if err != nil {
		log.Fatalf("resolve stage: %v", err)
	}

	if chosenPage != basePath {
		fmt.Printf("Validating %s...\n", chosenPage)
		if _, err := fetchWikitext(chosenPage); err != nil {
			log.Fatalf("chosen stage page does not load: %v", err)
		}
	}

	cfg := tournamentConfig{
		Name:  mongoSafe(tournamentName),
		Page:  chosenPage,
		Round: chosenStage,
	}

	if err := writeConfig(*out, cfg); err != nil {
		log.Fatalf("write config: %v", err)
	}
	fmt.Printf("\nWrote %s:\n", *out)
	fmt.Printf("  tournament_name = %q\n", cfg.Name)
	fmt.Printf("  page            = %q\n", cfg.Page)
	fmt.Printf("  round           = %q\n", cfg.Round)
}
