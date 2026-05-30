package sources

import "strings"

// NormalizeTeamName returns a canonical lowercase key used for cross-source name
// comparison (PandaScore, Liquipedia, VRS). The original name is never modified —
// this function is only for map keys and comparisons, not display.
func NormalizeTeamName(name string) string {
	s := strings.ToLower(name)
	s = strings.TrimPrefix(s, "team ")
	for _, suffix := range []string{" team", " esports", " gaming", " clan", " org", " organization"} {
		s = strings.TrimSuffix(s, suffix)
	}
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, ".", "")
	return strings.TrimSpace(s)
}
