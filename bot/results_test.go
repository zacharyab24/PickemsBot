package bot

import (
	"fmt"
	"strings"
	"testing"

	"pickems-bot/sources"

	"github.com/bwmarrin/discordgo"
)

// region canonicalElimRound tests

func TestCanonicalElimRound(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// Grand Final variants
		{"Grand final: A vs B", "Grand Final"},
		{"Grand Final", "Grand Final"},
		{"GRAND FINAL", "Grand Final"},
		// Semi-finals variants
		{"Semifinal 1: A vs B", "Semi-finals"},
		{"Semi-Final 2", "Semi-finals"},
		{"Semi final", "Semi-finals"},
		// Quarter-finals variants
		{"Quarterfinal 1: A vs B", "Quarter-finals"},
		{"Quarter-Final 2", "Quarter-finals"},
		{"Quarter final", "Quarter-finals"},
		// Round of N
		{"Round of 16", "Round of 16"},
		{"Round of 32", "Round of 32"},
		// Unknown section passed through unchanged
		{"Play-in Stage", "Play-in Stage"},
		{"Group A", "Group A"},
	}
	for _, tt := range tests {
		got := canonicalElimRound(tt.input)
		if got != tt.want {
			t.Errorf("canonicalElimRound(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// endregion

// region buildResultMatchSection tests

// sectionParts extracts the display text and button (label, style) from a match Section.
func sectionParts(t *testing.T, n sources.MatchNode) (text, label string, style discordgo.ButtonStyle) {
	t.Helper()
	s := buildResultMatchSection(n, 0)

	td, ok := s.Components[0].(discordgo.TextDisplay)
	if !ok {
		t.Fatalf("expected TextDisplay as first component, got %T", s.Components[0])
	}
	btn, ok := s.Accessory.(discordgo.Button)
	if !ok {
		t.Fatalf("expected Button as accessory, got %T", s.Accessory)
	}
	if !btn.Disabled {
		t.Error("expected button to be disabled")
	}
	return td.Content, btn.Label, btn.Style
}

func TestBuildResultMatchSection_WinnerTeam1(t *testing.T) {
	n := sources.MatchNode{Team1: "Alpha", Team2: "Beta", Winner: "Alpha", Score: "2-1", Status: "completed"}
	text, label, style := sectionParts(t, n)
	if text != "**Alpha** vs Beta" {
		t.Errorf("text = %q, want %q", text, "**Alpha** vs Beta")
	}
	if label != "2 - 1" {
		t.Errorf("label = %q, want %q", label, "2 - 1")
	}
	if style != discordgo.SuccessButton {
		t.Errorf("style = %v, want SuccessButton", style)
	}
}

func TestBuildResultMatchSection_WinnerTeam2(t *testing.T) {
	n := sources.MatchNode{Team1: "Alpha", Team2: "Beta", Winner: "Beta", Score: "0-2", Status: "completed"}
	text, label, _ := sectionParts(t, n)
	if text != "Alpha vs **Beta**" {
		t.Errorf("text = %q, want %q", text, "Alpha vs **Beta**")
	}
	if label != "0 - 2" {
		t.Errorf("label = %q, want %q", label, "0 - 2")
	}
}

func TestBuildResultMatchSection_Pending(t *testing.T) {
	n := sources.MatchNode{Team1: "Alpha", Team2: "Beta", Winner: "", Score: "", Status: "pending"}
	text, label, style := sectionParts(t, n)
	if text != "Alpha vs Beta" {
		t.Errorf("text = %q, want %q", text, "Alpha vs Beta")
	}
	if label != "-" {
		t.Errorf("label = %q, want %q", label, "-")
	}
	if style != discordgo.SecondaryButton {
		t.Errorf("style = %v, want SecondaryButton", style)
	}
}

func TestBuildResultMatchSection_TBDVsTBD_NoBolding(t *testing.T) {
	n := sources.MatchNode{Team1: "TBD", Team2: "TBD", Winner: "TBD", Score: "", Status: "pending"}
	text, _, _ := sectionParts(t, n)
	if text != "TBD vs TBD" {
		t.Errorf("text = %q, want %q", text, "TBD vs TBD")
	}
}

func TestBuildResultMatchSection_InProgress(t *testing.T) {
	n := sources.MatchNode{Team1: "Alpha", Team2: "Beta", Winner: "", Score: "", Status: "in_progress"}
	_, label, style := sectionParts(t, n)
	if label != "Live" {
		t.Errorf("label = %q, want %q", label, "Live")
	}
	if style != discordgo.PrimaryButton {
		t.Errorf("style = %v, want PrimaryButton", style)
	}
}

// endregion

// region result grouping tests

// assertGroups checks each group's label and match count.
func assertGroups(t *testing.T, groups []resultGroup, wantLabels []string, wantCounts []int) {
	t.Helper()
	if len(groups) != len(wantLabels) {
		t.Fatalf("expected %d groups, got %d", len(wantLabels), len(groups))
	}
	for i, g := range groups {
		if g.label != wantLabels[i] {
			t.Errorf("group[%d] label = %q, want %q", i, g.label, wantLabels[i])
		}
		if wantCounts != nil && len(g.nodes) != wantCounts[i] {
			t.Errorf("group[%d] match count = %d, want %d", i, len(g.nodes), wantCounts[i])
		}
	}
}

func TestSingleElimResultGroups_GroupsAndOrders(t *testing.T) {
	nodes := []sources.MatchNode{
		{Team1: "A", Team2: "B", Winner: "A", Score: "2-1", Section: "Quarterfinal 1: A vs B", Status: "completed"},
		{Team1: "C", Team2: "D", Winner: "C", Score: "2-0", Section: "Quarterfinal 2: C vs D", Status: "completed"},
		{Team1: "A", Team2: "C", Winner: "A", Score: "2-1", Section: "Semifinal 1: A vs C", Status: "completed"},
		{Team1: "A", Team2: "E", Winner: "", Score: "", Section: "Grand final: A vs E", Status: "pending"},
	}
	assertGroups(t, singleElimResultGroups(nodes),
		[]string{"Quarter-finals", "Semi-finals", "Grand Final"}, []int{2, 1, 1})
}

func TestSingleElimResultGroups_SkipsMissingRounds(t *testing.T) {
	nodes := []sources.MatchNode{
		{Team1: "A", Team2: "B", Winner: "A", Score: "2-0", Section: "Semifinal 1: A vs B", Status: "completed"},
		{Team1: "A", Team2: "C", Winner: "", Score: "", Section: "Grand final: A vs C", Status: "pending"},
	}
	assertGroups(t, singleElimResultGroups(nodes), []string{"Semi-finals", "Grand Final"}, nil)
}

func TestSingleElimResultGroups_AllRoundsPresent(t *testing.T) {
	nodes := []sources.MatchNode{
		{Team1: "A", Team2: "B", Section: "Round of 32: A vs B", Status: "completed", Winner: "A", Score: "2-0"},
		{Team1: "A", Team2: "C", Section: "Round of 16: A vs C", Status: "completed", Winner: "A", Score: "2-1"},
		{Team1: "A", Team2: "D", Section: "Quarterfinal 1: A vs D", Status: "completed", Winner: "A", Score: "2-0"},
		{Team1: "A", Team2: "E", Section: "Semifinal 1: A vs E", Status: "completed", Winner: "A", Score: "2-1"},
		{Team1: "A", Team2: "F", Section: "Grand final: A vs F", Status: "pending"},
	}
	assertGroups(t, singleElimResultGroups(nodes),
		[]string{"Round of 32", "Round of 16", "Quarter-finals", "Semi-finals", "Grand Final"}, nil)
}

func TestChronologicalResultGroups_PreservesOrderNoGrouping(t *testing.T) {
	nodes := []sources.MatchNode{
		{Team1: "A", Team2: "B", Winner: "A", Score: "2-0", Status: "completed"},
		{Team1: "C", Team2: "D", Status: "pending"},
	}
	groups := chronologicalResultGroups(nodes)
	assertGroups(t, groups, []string{"Results"}, []int{2})
	if groups[0].nodes[0].Team1 != "A" || groups[0].nodes[1].Team1 != "C" {
		t.Errorf("expected input order preserved, got %+v", groups[0].nodes)
	}
}

func TestSwissResultGroups_SortedByRound(t *testing.T) {
	nodes := []sources.MatchNode{
		{Team1: "A", Team2: "B", Winner: "A", Score: "2-0", Section: "Round 2", Status: "completed"},
		{Team1: "C", Team2: "D", Winner: "C", Score: "2-1", Section: "Round 2", Status: "completed"},
		{Team1: "E", Team2: "F", Winner: "E", Score: "2-0", Section: "Round 1", Status: "completed"},
		{Team1: "G", Team2: "H", Winner: "", Score: "", Section: "Round 3", Status: "pending"},
	}
	assertGroups(t, swissResultGroups(nodes),
		[]string{"Round 1", "Round 2", "Round 3"}, []int{1, 2, 1})
}

// endregion

// region pagination tests

// countComponents counts every component in the tree, nested ones included,
// the way Discord does for its per-message cap.
func countComponents(comps []discordgo.MessageComponent) int {
	n := 0
	for _, c := range comps {
		n++
		switch c := c.(type) {
		case discordgo.Container:
			n += countComponents(c.Components)
		case discordgo.ActionsRow:
			n += countComponents(c.Components)
		case discordgo.Section:
			n += countComponents(c.Components)
			if c.Accessory != nil {
				n++
			}
		}
	}
	return n
}

// matchNodes builds count completed matches in the given section.
func matchNodes(section string, count int) []sources.MatchNode {
	nodes := make([]sources.MatchNode, count)
	for i := range nodes {
		team := fmt.Sprintf("%s T%d", section, i)
		nodes[i] = sources.MatchNode{Team1: team, Team2: team + "b", Winner: team, Score: "2-0", Section: section, Status: "completed"}
	}
	return nodes
}

// navRow returns the trailing ActionsRow of a rendered page, or nil if none.
func navRow(comps []discordgo.MessageComponent) *discordgo.ActionsRow {
	if row, ok := comps[len(comps)-1].(discordgo.ActionsRow); ok {
		return &row
	}
	return nil
}

func TestPaginateResultGroups_EveryPageFitsAndKeepsAllMatchesInOrder(t *testing.T) {
	var nodes []sources.MatchNode
	for r := 1; r <= 5; r++ {
		nodes = append(nodes, matchNodes(fmt.Sprintf("Round %d", r), 8)...)
	}
	pages := paginateResultGroups(swissResultGroups(nodes))
	if len(pages) < 2 {
		t.Fatalf("expected multiple pages for 40 matches, got %d", len(pages))
	}

	var seen []sources.MatchNode
	for p := range pages {
		if got := countComponents(renderResultsPage(pages, p)); got > maxMessageComponents {
			t.Errorf("page %d has %d components, cap is %d", p, got, maxMessageComponents)
		}
		for _, g := range pages[p] {
			seen = append(seen, g.nodes...)
		}
	}
	if len(seen) != len(nodes) {
		t.Fatalf("expected %d matches across pages, got %d", len(nodes), len(seen))
	}
	for i := range nodes {
		if seen[i] != nodes[i] {
			t.Fatalf("match %d out of order: got %+v, want %+v", i, seen[i], nodes[i])
		}
	}
}

func TestPaginateResultGroups_SplitsOversizedGroup(t *testing.T) {
	// A 6-team round robin is 15 matches in one group, which can't fit one container.
	pages := paginateResultGroups(chronologicalResultGroups(matchNodes("Group A", 15)))
	if len(pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(pages))
	}
	if pages[0][0].label != "Results" || pages[1][0].label != "Results (cont.)" {
		t.Errorf("labels = %q, %q; want %q, %q", pages[0][0].label, pages[1][0].label, "Results", "Results (cont.)")
	}
	for p := range pages {
		if got := countComponents(renderResultsPage(pages, p)); got > maxMessageComponents {
			t.Errorf("page %d has %d components, cap is %d", p, got, maxMessageComponents)
		}
	}
}

func TestPaginateResultGroups_DoesNotSplitGroupsThatFit(t *testing.T) {
	groups := []resultGroup{
		{label: "Round 1", nodes: matchNodes("Round 1", 8)},
		{label: "Round 2", nodes: matchNodes("Round 2", 8)},
	}
	pages := paginateResultGroups(groups)
	if len(pages) != 2 {
		t.Fatalf("expected each round on its own page, got %d pages", len(pages))
	}
	for p, page := range pages {
		if len(page) != 1 || len(page[0].nodes) != 8 {
			t.Errorf("page %d: expected one whole round of 8, got %+v", p, page)
		}
	}
}

func TestRenderResultsPage_SinglePageHasNoNav(t *testing.T) {
	pages := paginateResultGroups(chronologicalResultGroups(matchNodes("Group A", 3)))
	comps := renderResultsPage(pages, 0)
	if navRow(comps) != nil {
		t.Error("expected no nav row for a single page")
	}
}

func TestRenderResultsPage_NavButtons(t *testing.T) {
	pages := paginateResultGroups(chronologicalResultGroups(matchNodes("Group A", 30)))
	if len(pages) != 3 {
		t.Fatalf("expected 3 pages, got %d", len(pages))
	}

	tests := []struct {
		page                       int
		prevID, label, nextID      string
		prevDisabled, nextDisabled bool
	}{
		{0, "results_page:-1", "Page 1/3", "results_page:1", true, false},
		{1, "results_page:0", "Page 2/3", "results_page:2", false, false},
		{2, "results_page:1", "Page 3/3", "results_page:3", false, true},
		// Out-of-range requests clamp to the nearest page.
		{9, "results_page:1", "Page 3/3", "results_page:3", false, true},
		{-4, "results_page:-1", "Page 1/3", "results_page:1", true, false},
	}
	for _, tt := range tests {
		row := navRow(renderResultsPage(pages, tt.page))
		if row == nil {
			t.Fatalf("page %d: expected nav row", tt.page)
		}
		prev := row.Components[0].(discordgo.Button)
		ind := row.Components[1].(discordgo.Button)
		next := row.Components[2].(discordgo.Button)
		if prev.CustomID != tt.prevID || prev.Disabled != tt.prevDisabled {
			t.Errorf("page %d prev = (%q, disabled=%v), want (%q, %v)", tt.page, prev.CustomID, prev.Disabled, tt.prevID, tt.prevDisabled)
		}
		if ind.Label != tt.label || !ind.Disabled {
			t.Errorf("page %d indicator = (%q, disabled=%v), want (%q, true)", tt.page, ind.Label, ind.Disabled, tt.label)
		}
		if next.CustomID != tt.nextID || next.Disabled != tt.nextDisabled {
			t.Errorf("page %d next = (%q, disabled=%v), want (%q, %v)", tt.page, next.CustomID, next.Disabled, tt.nextID, tt.nextDisabled)
		}
	}
}

func TestRenderResultsPage_WinnerBolded(t *testing.T) {
	pages := paginateResultGroups(swissResultGroups([]sources.MatchNode{
		{Team1: "A", Team2: "B", Winner: "A", Score: "2-0", Section: "Round 1", Status: "completed"},
	}))
	c := renderResultsPage(pages, 0)[0].(discordgo.Container)
	text := c.Components[2].(discordgo.Section).Components[0].(discordgo.TextDisplay).Content
	if !strings.Contains(text, "**A**") {
		t.Errorf("expected winner A to be bolded, got: %s", text)
	}
}

// endregion
