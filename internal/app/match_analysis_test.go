package app

import "testing"

func TestTypeEffectivenessUsesCanonicalPokemonChart(t *testing.T) {
	cases := []struct {
		name     string
		attacker Pokemon
		defender Pokemon
		want     float64
	}{
		{
			name:     "fire beats grass",
			attacker: Pokemon{Type1: "fire"},
			defender: Pokemon{Type1: "grass"},
			want:     2,
		},
		{
			name:     "electric has no effect on ground",
			attacker: Pokemon{Type1: "electric"},
			defender: Pokemon{Type1: "ground"},
			want:     0,
		},
		{
			name:     "ice is four times effective against dragon flying",
			attacker: Pokemon{Type1: "ice"},
			defender: Pokemon{Type1: "dragon", Type2: "flying"},
			want:     4,
		},
		{
			name:     "dual attacker uses best attacking type",
			attacker: Pokemon{Type1: "normal", Type2: "water"},
			defender: Pokemon{Type1: "rock"},
			want:     2,
		},
		{
			name:     "best attacking type still hits both defender types",
			attacker: Pokemon{Type1: "fire", Type2: "grass"},
			defender: Pokemon{Type1: "bug", Type2: "steel"},
			want:     4,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := typeEffectiveness(tc.attacker, tc.defender); got != tc.want {
				t.Fatalf("type effectiveness = %.2f, want %.2f", got, tc.want)
			}
		})
	}
}

func TestAnalyzeMatchIncludesOverallAndKeyMatchups(t *testing.T) {
	teamA, teamB := llmTestTeams()
	analysis := AnalyzeMatch(teamA, teamB)
	if analysis.Overall.TeamAPower <= 0 || analysis.Overall.TeamBPower <= 0 {
		t.Fatalf("overall powers = %+v", analysis.Overall)
	}
	if analysis.Overall.Favorite == "" || analysis.Overall.Summary == "" {
		t.Fatalf("overall missing favorite or summary = %+v", analysis.Overall)
	}
	if len(analysis.KeyMatchups) < 4 {
		t.Fatalf("key matchup count = %d", len(analysis.KeyMatchups))
	}
	if analysis.KeyMatchups[0].Summary == "" || analysis.KeyMatchups[0].TeamATypeEdge <= 0 {
		t.Fatalf("first matchup = %+v", analysis.KeyMatchups[0])
	}
}
