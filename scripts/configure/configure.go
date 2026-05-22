/* configure.go
 * Pure helpers used by the configure script. Kept separate from main.go
 * so they're reachable from tests (main.go uses //go:build !test).
 */

package main

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"pickems-bot/api/external"
)

const liquipediaBase = "https://liquipedia.net/counterstrike/"

type tournamentConfig struct {
	Name         string
	Page         string
	Round        string
	Format       string // empty = auto-detect; set to "swiss" or "single-elimination" for multi-stage pages
	UpcomingOnly bool
	Test         bool
}

func fetchWikitext(path string) (string, error) {
	wikitext, err := external.GetWikitext(liquipediaBase + path + "?action=raw")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(wikitext) == "" {
		return "", fmt.Errorf("empty response (page may not exist)")
	}
	return wikitext, nil
}

func pagePathFromURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if !strings.Contains(u.Host, "liquipedia.net") {
		return "", fmt.Errorf("expected a liquipedia.net URL")
	}
	path := strings.TrimPrefix(u.Path, "/counterstrike/")
	path = strings.Trim(path, "/")
	if path == "" || path == strings.Trim(u.Path, "/") {
		return "", fmt.Errorf("URL must point to a /counterstrike/ page")
	}
	return path, nil
}

var nameRe = regexp.MustCompile(`\|\s*name\s*=\s*([^|\n]+)`)

func extractTournamentName(wikitext string) string {
	m := nameRe.FindStringSubmatch(wikitext)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

var relativeLinkRe = regexp.MustCompile(`\[\[/([^|\]/]+)`)

// extractStages returns top-level sub-stage names (e.g. Stage_1, Playoffs, Europe)
// found in the page wikitext. Catches multiple reference forms: wikilinks
// ([[Base/Stage 1|...]]), relative wikilinks ([[/Stage 1|...]]), section
// transclusions ({{#section:Base/Stage 1|...}}), and bare template params
// (|tournament4=Base/Playoffs). Nested paths collapse to their first segment.
func extractStages(wikitext, basePath string) []string {
	baseSpaces := strings.ReplaceAll(basePath, "_", " ")
	seen := map[string]struct{}{}

	add := func(sub string) {
		sub = strings.TrimSpace(sub)
		if i := strings.Index(sub, "/"); i >= 0 {
			sub = sub[:i]
		}
		if sub == "" || strings.HasPrefix(sub, "#") {
			return
		}
		seen[strings.ReplaceAll(sub, " ", "_")] = struct{}{}
	}

	for _, m := range relativeLinkRe.FindAllStringSubmatch(wikitext, -1) {
		add(m[1])
	}

	pat := fmt.Sprintf(`(?:%s|%s)/([A-Za-z0-9 _\-]+)`,
		regexp.QuoteMeta(basePath), regexp.QuoteMeta(baseSpaces))
	absRe := regexp.MustCompile(pat)
	for _, m := range absRe.FindAllStringSubmatch(wikitext, -1) {
		add(m[1])
	}

	stages := make([]string, 0, len(seen))
	for s := range seen {
		stages = append(stages, s)
	}
	sort.Strings(stages)
	return stages
}

func resolveStage(flagValue string, stages []string, basePath string, in io.Reader, out io.Writer) (round, page string, err error) {
	switch {
	case flagValue != "":
		return flagValue, basePath + "/" + flagValue, nil
	case len(stages) == 0:
		fmt.Fprintln(out, "No sub-stages found — using base page as the round.")
		return "Main", basePath, nil
	case len(stages) == 1:
		fmt.Fprintf(out, "Single stage detected: %s\n", stages[0])
		return stages[0], basePath + "/" + stages[0], nil
	default:
		picked, err := pickStage(stages, in, out)
		if err != nil {
			return "", "", err
		}
		return picked, basePath + "/" + picked, nil
	}
}

func pickStage(stages []string, in io.Reader, out io.Writer) (string, error) {
	fmt.Fprintln(out, "\nAvailable stages:")
	for i, s := range stages {
		fmt.Fprintf(out, "  [%d] %s\n", i+1, s)
	}
	fmt.Fprint(out, "Pick a stage [1]: ")

	reader := bufio.NewReader(in)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)

	idx := 1
	if line != "" {
		if _, err := fmt.Sscanf(line, "%d", &idx); err != nil {
			return "", fmt.Errorf("invalid input: %w", err)
		}
	}
	if idx < 1 || idx > len(stages) {
		return "", fmt.Errorf("selection out of range")
	}
	return stages[idx-1], nil
}

var nonAlnum = regexp.MustCompile(`[^A-Za-z0-9]+`)

// mongoSafe normalises a free-form name into a Mongo-friendly database name.
func mongoSafe(s string) string {
	s = nonAlnum.ReplaceAllString(strings.TrimSpace(s), "_")
	return strings.Trim(s, "_")
}

func writeConfig(path string, c tournamentConfig) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	fmt.Fprintln(f, "# Tournament configuration. Generated/updated by `go run ./scripts/configure`.")
	fmt.Fprintln(f)
	fmt.Fprintf(f, "tournament_name = %q\n", c.Name)
	fmt.Fprintf(f, "page           = %q\n", c.Page)
	fmt.Fprintf(f, "round          = %q\n", c.Round)
	fmt.Fprintf(f, "format         = %q\n", c.Format)
	fmt.Fprintf(f, "upcoming_only  = %t\n", c.UpcomingOnly)
	fmt.Fprintf(f, "test           = %t\n", c.Test)
	return nil
}
