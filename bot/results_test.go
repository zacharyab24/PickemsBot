package bot

import (
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
	if label != "2 — 1" {
		t.Errorf("label = %q, want %q", label, "2 — 1")
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
	if label != "0 — 2" {
		t.Errorf("label = %q, want %q", label, "0 — 2")
	}
}

func TestBuildResultMatchSection_Pending(t *testing.T) {
	n := sources.MatchNode{Team1: "Alpha", Team2: "Beta", Winner: "", Score: "", Status: "pending"}
	text, label, style := sectionParts(t, n)
	if text != "Alpha vs Beta" {
		t.Errorf("text = %q, want %q", text, "Alpha vs Beta")
	}
	if label != "—" {
		t.Errorf("label = %q, want %q", label, "—")
	}
	if style != discordgo.SecondaryButton {
		t.Errorf("style = %v, want SecondaryButton", style)
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

// region buildSingleElimResultComponents tests

// containerHeading returns the round label from the first TextDisplay in a Container.
func containerHeading(t *testing.T, comp discordgo.MessageComponent) string {
	t.Helper()
	c, ok := comp.(discordgo.Container)
	if !ok {
		t.Fatalf("expected Container, got %T", comp)
	}
	td, ok := c.Components[0].(discordgo.TextDisplay)
	if !ok {
		t.Fatalf("expected TextDisplay as first component, got %T", c.Components[0])
	}
	// Strip leading "## " prefix added by buildRoundContainer.
	if len(td.Content) > 3 {
		return td.Content[3:]
	}
	return td.Content
}

// containerMatchCount returns the number of Section components in a Container
// (skips the leading TextDisplay heading and Separator).
func containerMatchCount(t *testing.T, comp discordgo.MessageComponent) int {
	t.Helper()
	c := comp.(discordgo.Container)
	return len(c.Components) - 2 // heading + separator
}

func TestBuildSingleElimResultComponents_GroupsAndOrders(t *testing.T) {
	nodes := []sources.MatchNode{
		{Team1: "A", Team2: "B", Winner: "A", Score: "2-1", Section: "Quarterfinal 1: A vs B", Status: "completed"},
		{Team1: "C", Team2: "D", Winner: "C", Score: "2-0", Section: "Quarterfinal 2: C vs D", Status: "completed"},
		{Team1: "A", Team2: "C", Winner: "A", Score: "2-1", Section: "Semifinal 1: A vs C", Status: "completed"},
		{Team1: "A", Team2: "E", Winner: "", Score: "", Section: "Grand final: A vs E", Status: "pending"},
	}
	result := buildSingleElimResultComponents(nodes)

	if len(result) != 3 {
		t.Fatalf("expected 3 containers, got %d", len(result))
	}

	wantLabels := []string{"Quarter-finals", "Semi-finals", "Grand Final"}
	wantCounts := []int{2, 1, 1}
	for i, comp := range result {
		if got := containerHeading(t, comp); got != wantLabels[i] {
			t.Errorf("container[%d] label = %q, want %q", i, got, wantLabels[i])
		}
		if got := containerMatchCount(t, comp); got != wantCounts[i] {
			t.Errorf("container[%d] match count = %d, want %d", i, got, wantCounts[i])
		}
	}
}

func TestBuildSingleElimResultComponents_SkipsMissingRounds(t *testing.T) {
	nodes := []sources.MatchNode{
		{Team1: "A", Team2: "B", Winner: "A", Score: "2-0", Section: "Semifinal 1: A vs B", Status: "completed"},
		{Team1: "A", Team2: "C", Winner: "", Score: "", Section: "Grand final: A vs C", Status: "pending"},
	}
	result := buildSingleElimResultComponents(nodes)
	if len(result) != 2 {
		t.Fatalf("expected 2 containers (no QF), got %d", len(result))
	}
	if got := containerHeading(t, result[0]); got != "Semi-finals" {
		t.Errorf("first container = %q, want Semi-finals", got)
	}
	if got := containerHeading(t, result[1]); got != "Grand Final" {
		t.Errorf("second container = %q, want Grand Final", got)
	}
}

func TestBuildSingleElimResultComponents_AllRoundsPresent(t *testing.T) {
	nodes := []sources.MatchNode{
		{Team1: "A", Team2: "B", Section: "Round of 32: A vs B", Status: "completed", Winner: "A", Score: "2-0"},
		{Team1: "A", Team2: "C", Section: "Round of 16: A vs C", Status: "completed", Winner: "A", Score: "2-1"},
		{Team1: "A", Team2: "D", Section: "Quarterfinal 1: A vs D", Status: "completed", Winner: "A", Score: "2-0"},
		{Team1: "A", Team2: "E", Section: "Semifinal 1: A vs E", Status: "completed", Winner: "A", Score: "2-1"},
		{Team1: "A", Team2: "F", Section: "Grand final: A vs F", Status: "pending"},
	}
	result := buildSingleElimResultComponents(nodes)
	if len(result) != 5 {
		t.Fatalf("expected 5 containers, got %d", len(result))
	}
	wantOrder := []string{"Round of 32", "Round of 16", "Quarter-finals", "Semi-finals", "Grand Final"}
	for i, comp := range result {
		if got := containerHeading(t, comp); got != wantOrder[i] {
			t.Errorf("container[%d] = %q, want %q", i, got, wantOrder[i])
		}
	}
}

// endregion

// region buildSwissResultComponents tests

func TestBuildSwissResultComponents_SortedByRound(t *testing.T) {
	nodes := []sources.MatchNode{
		{Team1: "A", Team2: "B", Winner: "A", Score: "2-0", Section: "Round 2", Status: "completed"},
		{Team1: "C", Team2: "D", Winner: "C", Score: "2-1", Section: "Round 2", Status: "completed"},
		{Team1: "E", Team2: "F", Winner: "E", Score: "2-0", Section: "Round 1", Status: "completed"},
		{Team1: "G", Team2: "H", Winner: "", Score: "", Section: "Round 3", Status: "pending"},
	}
	result := buildSwissResultComponents(nodes)

	if len(result) != 3 {
		t.Fatalf("expected 3 containers, got %d", len(result))
	}
	wantLabels := []string{"Round 1", "Round 2", "Round 3"}
	for i, comp := range result {
		if got := containerHeading(t, comp); got != wantLabels[i] {
			t.Errorf("container[%d] = %q, want %q", i, got, wantLabels[i])
		}
	}
	if got := containerMatchCount(t, result[1]); got != 2 {
		t.Errorf("Round 2 has %d matches, want 2", got)
	}
}

func TestBuildSwissResultComponents_SingleRound(t *testing.T) {
	nodes := []sources.MatchNode{
		{Team1: "A", Team2: "B", Winner: "A", Score: "2-0", Section: "Round 1", Status: "completed"},
		{Team1: "C", Team2: "D", Winner: "C", Score: "2-1", Section: "Round 1", Status: "completed"},
	}
	result := buildSwissResultComponents(nodes)
	if len(result) != 1 {
		t.Fatalf("expected 1 container, got %d", len(result))
	}
	if got := containerMatchCount(t, result[0]); got != 2 {
		t.Errorf("Round 1 has %d matches, want 2", got)
	}
}

// endregion
