package app

import "testing"

func TestBuildMatchFromSimulationResolvesRefsAndScore(t *testing.T) {
	teamA := Team{
		ID:         "a",
		Name:       "Time A",
		AlaDireita: Pokemon{ID: 25, Name: "Pikachu"},
		Pivo:       Pokemon{ID: 68, Name: "Machamp"},
	}
	teamB := Team{
		ID:          "b",
		Name:        "Time B",
		Fixo:        Pokemon{ID: 95, Name: "Onix"},
		AlaEsquerda: Pokemon{ID: 26, Name: "Raichu"},
	}
	payload := SimulationPayload{
		Events: []SimulationEvent{
			{Minute: 27, Type: "goal", TeamRef: "team_b", PokemonRef: "ala_esquerda", Narrative: "{{team_b.ala_esquerda}} empatou para o {{team_b.name}}."},
			{Minute: 18, Type: "goal", TeamRef: "team_a", PokemonRef: "pivo", Narrative: "{{team_a.pivo}} abriu o placar contra {{team_b.fixo}}."},
		},
		Consequences: []SimulationConsequence{
			{TeamRef: "team_b", PokemonRef: "fixo", EffectDescription: "{{team_b.fixo}} ficou cansado marcando {{team_a.pivo}}."},
		},
	}
	normalizeSimulationPayload(&payload)

	match := BuildMatchFromSimulation(teamA, teamB, payload)

	if match.Events[0].Minute != 18 {
		t.Fatalf("first event minute = %d, want 18", match.Events[0].Minute)
	}
	if match.Events[0].TeamID != "a" || match.Events[0].PokemonID != 68 {
		t.Fatalf("first goal attribution = %+v", match.Events[0])
	}
	if match.Events[0].Narrative != "Machamp abriu o placar contra Onix." {
		t.Fatalf("first narrative = %q", match.Events[0].Narrative)
	}
	if match.Events[1].TeamID != "b" || match.Events[1].PokemonID != 26 {
		t.Fatalf("second goal attribution = %+v", match.Events[1])
	}
	if match.ScoreTeamA != 1 || match.ScoreTeamB != 1 {
		t.Fatalf("score = %d x %d, want 1 x 1", match.ScoreTeamA, match.ScoreTeamB)
	}
	if len(match.Consequences) != 1 || match.Consequences[0].TeamID != "b" || match.Consequences[0].PokemonID != 95 {
		t.Fatalf("consequence attribution = %+v", match.Consequences)
	}
	if match.Consequences[0].EffectDescription != "Onix ficou cansado marcando Machamp." {
		t.Fatalf("consequence text = %q", match.Consequences[0].EffectDescription)
	}
}

func TestLoadSimulationPayloadFromDefaultFile(t *testing.T) {
	payload, err := LoadSimulationPayload("")
	if err != nil {
		t.Fatal(err)
	}
	if len(payload.Events) == 0 {
		t.Fatal("expected sample events")
	}
	if payload.Events[0].Minute != 0 {
		t.Fatalf("first sample minute = %d, want 0", payload.Events[0].Minute)
	}
}

func TestParseSimulationPayloadMergesBuildUpAndResolution(t *testing.T) {
	payload, err := ParseSimulationPayload([]byte(`{
		"events": [
			{"minute": 0, "type": "kickoff", "team_ref": null, "pokemon_ref": null, "narrative_build_up": "A bola espera no centro.", "narrative_resolution": "Comeca o jogo."},
			{"minute": 20, "type": "halftime", "team_ref": null, "pokemon_ref": null, "narrative_build_up": "O relogio chega aos 20.", "narrative_resolution": "Intervalo."},
			{"minute": 40, "type": "fulltime", "team_ref": null, "pokemon_ref": null, "narrative_build_up": "Ultimo apito armado.", "narrative_resolution": "Fim de jogo."}
		],
		"consequences": []
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := payload.Events[0].Narrative; got != "A bola espera no centro. Comeca o jogo." {
		t.Fatalf("merged narrative = %q", got)
	}
}

func TestParseSimulationPayloadNormalizesTeamNameReferencesOnlyInNarrativeText(t *testing.T) {
	payload, err := ParseSimulationPayload([]byte(`{
		"events": [
			{"minute": 0, "type": "kickoff", "team_ref": null, "pokemon_ref": null, "narrative_build_up": "Time A sobe linhas contra team B.", "narrative_resolution": "{{team_a.pivo}} puxa a pressao."},
			{"minute": 20, "type": "halftime", "team_ref": null, "pokemon_ref": null, "narrative_build_up": "team_a respira.", "narrative_resolution": "Team B conversa."},
			{"minute": 40, "type": "fulltime", "team_ref": "team_a", "pokemon_ref": "pivo", "narrative_build_up": "O time B tentou ate o fim.", "narrative_resolution": "Fim."}
		],
		"consequences": [
			{"team_ref": "team_b", "pokemon_ref": "fixo", "effect_description": "team_a cansou o time B."}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := payload.Events[0].Narrative; got != "{{team_a.name}} sobe linhas contra {{team_b.name}}. {{team_a.pivo}} puxa a pressao." {
		t.Fatalf("normalized narrative = %q", got)
	}
	if payload.Events[2].TeamRef != "team_a" || payload.Events[2].PokemonRef != "pivo" {
		t.Fatalf("structured refs were changed: %+v", payload.Events[2])
	}
	if got := payload.Consequences[0].EffectDescription; got != "{{team_a.name}} cansou o {{team_b.name}}." {
		t.Fatalf("normalized consequence = %q", got)
	}
}

func TestParseSimulationPayloadRejectsMissingRequiredEvents(t *testing.T) {
	_, err := ParseSimulationPayload([]byte(`{
		"events": [
			{"minute": 0, "type": "kickoff", "narrative": "Comeca."},
			{"minute": 40, "type": "fulltime", "narrative": "Fim."}
		],
		"consequences": []
	}`))
	if err == nil {
		t.Fatal("expected missing halftime error")
	}
}
