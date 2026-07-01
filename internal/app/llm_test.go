package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenRouterMatchGeneratorBuildsMatchFromStructuredResponse(t *testing.T) {
	var requestBody openRouterChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [
				{
					"message": {
						"role": "assistant",
						"content": "{\"events\":[{\"minute\":0,\"type\":\"kickoff\",\"time_ref\":null,\"pokemon_ref\":null,\"narrative_build_up\":\"A bola fica parada no centro.\",\"narrative_resolution\":\"Comeca a partida.\"},{\"minute\":18,\"type\":\"goal\",\"time_ref\":\"time_da_casa\",\"pokemon_ref\":\"pivo\",\"narrative_build_up\":\"Machamp gira sobre Onix e prepara o chute.\",\"narrative_resolution\":\"{goal} GOOOL do Time da Casa!\"},{\"minute\":20,\"type\":\"halftime\",\"time_ref\":null,\"pokemon_ref\":null,\"narrative_build_up\":\"O relogio chega ao intervalo.\",\"narrative_resolution\":\"Pausa para respirar.\"},{\"minute\":40,\"type\":\"fulltime\",\"time_ref\":null,\"pokemon_ref\":null,\"narrative_build_up\":\"O apito vai a boca.\",\"narrative_resolution\":\"Fim de jogo.\"}],\"consequences\":[]}"
					}
				}
			]
		}`))
	}))
	defer server.Close()

	promptPath := filepath.Join(t.TempDir(), "systemprompt.md")
	if err := os.WriteFile(promptPath, []byte("Retorne JSON."), 0600); err != nil {
		t.Fatal(err)
	}

	teamA, teamB := llmTestTeams()
	generator := OpenRouterMatchGenerator{
		APIKey:     "test-key",
		Model:      "openai/gpt-oss-120b:free",
		BaseURL:    server.URL,
		PromptPath: promptPath,
		StrictJSON: true,
		HTTPClient: server.Client(),
	}
	match, err := generator.GenerateMatch(context.Background(), teamA, teamB)
	if err != nil {
		t.Fatal(err)
	}
	if requestBody.Model != "openai/gpt-oss-120b:free" {
		t.Fatalf("model = %q", requestBody.Model)
	}
	if requestBody.ResponseFormat["type"] != "json_schema" {
		t.Fatalf("response_format = %+v", requestBody.ResponseFormat)
	}
	if len(requestBody.Messages) != 2 || !strings.Contains(requestBody.Messages[1].Content, "time_da_casa") || !strings.Contains(requestBody.Messages[1].Content, "analise_do_servidor") {
		t.Fatalf("messages = %+v", requestBody.Messages)
	}
	if strings.Contains(requestBody.Messages[1].Content, "Kanto Press") || strings.Contains(requestBody.Messages[1].Content, "Paleta Bolada") || strings.Contains(requestBody.Messages[1].Content, "team-a") || strings.Contains(requestBody.Messages[1].Content, "team_a") {
		t.Fatalf("user-controlled team text leaked into prompt: %s", requestBody.Messages[1].Content)
	}
	if !strings.Contains(requestBody.Messages[1].Content, "confrontos") || !strings.Contains(requestBody.Messages[1].Content, "dinamica_da_partida") {
		t.Fatalf("user prompt did not include matchup analysis: %s", requestBody.Messages[1].Content)
	}
	if match.ScoreTeamA != 1 || match.ScoreTeamB != 0 {
		t.Fatalf("score = %d x %d", match.ScoreTeamA, match.ScoreTeamB)
	}
	if match.Events[1].TeamID != teamA.ID || match.Events[1].PokemonID != teamA.Pivo.ID {
		t.Fatalf("goal attribution = %+v", match.Events[1])
	}
	if !strings.Contains(match.Events[1].Narrative, "{goal} GOOOL") {
		t.Fatalf("goal narrative = %q", match.Events[1].Narrative)
	}
}

func TestOpenRouterMatchGeneratorDoesNotSendStrictSchemaByDefault(t *testing.T) {
	var requestBody openRouterChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [
				{
					"message": {
						"role": "assistant",
						"content": "{\"events\":[{\"minute\":0,\"type\":\"kickoff\",\"time_ref\":null,\"pokemon_ref\":null,\"narrative_build_up\":\"A bola fica parada.\",\"narrative_resolution\":\"Comeca.\"},{\"minute\":20,\"type\":\"halftime\",\"time_ref\":null,\"pokemon_ref\":null,\"narrative_build_up\":\"Intervalo chegando.\",\"narrative_resolution\":\"Pausa.\"},{\"minute\":40,\"type\":\"fulltime\",\"time_ref\":null,\"pokemon_ref\":null,\"narrative_build_up\":\"Apito pronto.\",\"narrative_resolution\":\"Fim.\"}],\"consequences\":[]}"
					}
				}
			]
		}`))
	}))
	defer server.Close()

	promptPath := filepath.Join(t.TempDir(), "systemprompt.md")
	if err := os.WriteFile(promptPath, []byte("Retorne JSON."), 0600); err != nil {
		t.Fatal(err)
	}

	teamA, teamB := llmTestTeams()
	generator := OpenRouterMatchGenerator{
		APIKey:     "test-key",
		Model:      "openai/gpt-oss-120b:free",
		BaseURL:    server.URL,
		PromptPath: promptPath,
		HTTPClient: server.Client(),
	}
	if _, err := generator.GenerateMatch(context.Background(), teamA, teamB); err != nil {
		t.Fatal(err)
	}
	if requestBody.ResponseFormat != nil {
		t.Fatalf("response_format should be nil by default: %+v", requestBody.ResponseFormat)
	}
}

func TestOpenRouterMatchGeneratorReturnsTypedHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"temporarily rate-limited upstream"}}`, http.StatusTooManyRequests)
	}))
	defer server.Close()

	promptPath := filepath.Join(t.TempDir(), "systemprompt.md")
	if err := os.WriteFile(promptPath, []byte("Retorne JSON."), 0600); err != nil {
		t.Fatal(err)
	}

	teamA, teamB := llmTestTeams()
	generator := OpenRouterMatchGenerator{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		PromptPath: promptPath,
		HTTPClient: server.Client(),
	}
	_, err := generator.GenerateMatch(context.Background(), teamA, teamB)
	var openRouterErr *OpenRouterError
	if !errors.As(err, &openRouterErr) {
		t.Fatalf("error = %v, want OpenRouterError", err)
	}
	if openRouterErr.StatusCode != http.StatusTooManyRequests || !strings.Contains(openRouterErr.Body, "rate-limited") {
		t.Fatalf("openrouter error = %+v", openRouterErr)
	}
}

func TestBuildMatchUserPromptIncludesAbilityDescription(t *testing.T) {
	teamA, teamB := llmTestTeams()
	teamA.Goalkeeper.Abilities = `[{"name":"torrent","description":"Powers up Water-type moves in a pinch."}]`

	prompt := buildMatchUserPrompt(teamA, teamB, AnalyzeMatch(teamA, teamB))
	if !strings.Contains(prompt, `"ability_description": "Powers up Water-type moves in a pinch."`) {
		t.Fatalf("prompt did not include ability description: %s", prompt)
	}
	if !strings.Contains(prompt, `"indice_time_da_casa"`) || !strings.Contains(prompt, `"diferenca_percentual_forca"`) {
		t.Fatalf("prompt did not include normalized power context: %s", prompt)
	}
	if !strings.Contains(prompt, `"dinamica_da_partida"`) || !strings.Contains(prompt, `"faixa_total_gols_sugerida"`) {
		t.Fatalf("prompt did not include match dynamics: %s", prompt)
	}
	if !strings.Contains(prompt, `"chance_de_zebra"`) || !strings.Contains(prompt, `"sorteio"`) || !strings.Contains(prompt, `"probabilidade_percentual"`) {
		t.Fatalf("prompt did not include structured upset chance: %s", prompt)
	}
}

func TestBuildMatchDynamicsIncludesStructuredUpsetSignal(t *testing.T) {
	teamA, teamB := llmTestTeams()
	analysis := AnalyzeMatch(teamA, teamB)
	dynamics := BuildMatchDynamics(analysis)
	if dynamics.ChanceDeZebra.Sorteio == "" || dynamics.ChanceDeZebra.NivelBase == "" {
		t.Fatalf("upset signal missing fields: %+v", dynamics.ChanceDeZebra)
	}
	if dynamics.ChanceDeZebra.Sorteio == "nao_aplicavel" {
		return
	}
	if dynamics.ChanceDeZebra.Beneficiado != "time_da_casa" && dynamics.ChanceDeZebra.Beneficiado != "time_visitante" {
		t.Fatalf("upset beneficiary = %q", dynamics.ChanceDeZebra.Beneficiado)
	}
	if dynamics.ChanceDeZebra.ProbabilidadePercentual < 3 || dynamics.ChanceDeZebra.ProbabilidadePercentual > 50 {
		t.Fatalf("upset probability = %d", dynamics.ChanceDeZebra.ProbabilidadePercentual)
	}
	switch dynamics.ChanceDeZebra.Sorteio {
	case "ativada", "latente":
	default:
		t.Fatalf("upset draw = %q", dynamics.ChanceDeZebra.Sorteio)
	}
}

func TestMatchResponseFormatUsesPortugueseGoalkeeperRef(t *testing.T) {
	data, err := json.Marshal(matchResponseFormat())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "goalkeeper") {
		t.Fatalf("response format should not expose goalkeeper ref: %s", string(data))
	}
	if strings.Contains(string(data), "team_a") || strings.Contains(string(data), "team_b") || !strings.Contains(string(data), "time_ref") || !strings.Contains(string(data), "time_da_casa") {
		t.Fatalf("response format should expose Portuguese team refs only: %s", string(data))
	}
}

func TestOpenRouterTimeoutUsesEnvOrDefault(t *testing.T) {
	t.Setenv("OPENROUTER_TIMEOUT_SECONDS", "")
	if got := openRouterTimeout(); got != defaultOpenRouterTimeout {
		t.Fatalf("default timeout = %s", got)
	}
	t.Setenv("OPENROUTER_TIMEOUT_SECONDS", "3")
	if got := openRouterTimeout(); got.String() != "3s" {
		t.Fatalf("env timeout = %s", got)
	}
}

func TestFallbackMatchGeneratorUsesFallbackOnPrimaryError(t *testing.T) {
	teamA, teamB := llmTestTeams()
	generator := FallbackMatchGenerator{
		Primary:  OpenRouterMatchGenerator{APIKey: ""},
		Fallback: LocalMatchGenerator{},
	}
	match, err := generator.GenerateMatch(context.Background(), teamA, teamB)
	if err != nil {
		t.Fatal(err)
	}
	if len(match.Events) == 0 {
		t.Fatal("fallback did not generate events")
	}
}

func llmTestTeams() (Team, Team) {
	pokemon := samplePokemon()
	teamA := Team{
		ID:                 "team-a",
		Name:               "Kanto Press",
		Goalkeeper:         pokemon[9],
		GoalkeeperAbility:  "torrent",
		Fixo:               pokemon[6],
		FixoAbility:        "blaze",
		AlaEsquerda:        pokemon[25],
		AlaEsquerdaAbility: "static",
		AlaDireita:         pokemon[4],
		AlaDireitaAbility:  "blaze",
		Pivo:               pokemon[68],
		PivoAbility:        "no-guard",
	}
	teamB := Team{
		ID:                 "team-b",
		Name:               "Paleta Bolada",
		Goalkeeper:         pokemon[143],
		GoalkeeperAbility:  "thick-fat",
		Fixo:               pokemon[95],
		FixoAbility:        "sturdy",
		AlaEsquerda:        pokemon[26],
		AlaEsquerdaAbility: "static",
		AlaDireita:         pokemon[7],
		AlaDireitaAbility:  "torrent",
		Pivo:               pokemon[149],
		PivoAbility:        "inner-focus",
	}
	return teamA, teamB
}
