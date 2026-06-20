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

func TestAnalyzeMatchIncludesOverallAndPhaseMatchups(t *testing.T) {
	teamA, teamB := llmTestTeams()
	analysis := AnalyzeMatch(teamA, teamB)
	if analysis.Overall.TeamAPower <= 0 || analysis.Overall.TeamBPower <= 0 {
		t.Fatalf("overall powers = %+v", analysis.Overall)
	}
	if analysis.Overall.Favorite == "" || analysis.Overall.Summary == "" {
		t.Fatalf("overall missing favorite or summary = %+v", analysis.Overall)
	}
	if len(analysis.PhaseMatchups) != 2 {
		t.Fatalf("phase matchup count = %d", len(analysis.PhaseMatchups))
	}
	expectedRefs := [][2]string{
		{"team_a.attack", "team_b.defense"},
		{"team_b.attack", "team_a.defense"},
	}
	for i, matchup := range analysis.PhaseMatchups {
		if matchup.AttackRef != expectedRefs[i][0] || matchup.DefenseRef != expectedRefs[i][1] {
			t.Fatalf("phase matchup refs[%d] = %+v", i, matchup)
		}
		if matchup.Label == "" || matchup.Summary == "" || matchup.AttackScore <= 0 || matchup.DefenseScore <= 0 {
			t.Fatalf("phase matchup[%d] missing fields = %+v", i, matchup)
		}
		switch matchup.Advantage {
		case "attack", "defense", "neutral":
		default:
			t.Fatalf("phase matchup[%d] advantage = %q", i, matchup.Advantage)
		}
	}
}

func TestPhaseAdvantageUsesStrongTypeInfluence(t *testing.T) {
	if got := phaseAdvantage(100, 130, 2); got != "attack" {
		t.Fatalf("phase advantage = %q, want attack", got)
	}
	if got := phaseAdvantage(100, 100, 1); got != "neutral" {
		t.Fatalf("neutral phase advantage = %q, want neutral", got)
	}
}
