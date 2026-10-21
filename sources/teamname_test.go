package sources

import "testing"

func TestNormalizeTeamName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Team Liquid", "liquid"},
		{"FaZe Clan", "faze"},
		{"MOUZ", "mouz"},
		{"Natus Vincere", "natus vincere"},
		{"BIG Esports", "big"},
		{"G2 Gaming", "g2"},
		{"Ninjas in Pyjamas Team", "ninjas in pyjamas"},
		{"9z Team Org", "9z team"},
		{"BLAST.tv", "blasttv"},
		{"Rare Atom's", "rare atoms"},
		{"  Vitality  ", "vitality"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := NormalizeTeamName(tt.in); got != tt.want {
				t.Errorf("NormalizeTeamName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
