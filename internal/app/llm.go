package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultOpenRouterBaseURL = "https://openrouter.ai/api/v1"
const defaultOpenRouterModel = "openai/gpt-oss-120b:free"
const defaultSystemPromptPath = "examples/systemprompt.md"
const defaultOpenRouterTimeout = 120 * time.Second

type MatchGenerator interface {
	GenerateMatch(ctx context.Context, teamA Team, teamB Team) (MatchResult, error)
}

type LocalMatchGenerator struct{}

func (LocalMatchGenerator) GenerateMatch(_ context.Context, teamA Team, teamB Team) (MatchResult, error) {
	return SimulateMatch(teamA, teamB), nil
}

type FallbackMatchGenerator struct {
	Primary  MatchGenerator
	Fallback MatchGenerator
}

func (g FallbackMatchGenerator) GenerateMatch(ctx context.Context, teamA Team, teamB Team) (MatchResult, error) {
	if g.Primary != nil {
		match, err := g.Primary.GenerateMatch(ctx, teamA, teamB)
		if err == nil {
			return match, nil
		}
		log.Printf("match generator primary failed, using fallback: %v", err)
	}
	if g.Fallback == nil {
		return MatchResult{}, errors.New("match generator failed and no fallback is configured")
	}
	return g.Fallback.GenerateMatch(ctx, teamA, teamB)
}

type OpenRouterMatchGenerator struct {
	APIKey     string
	Model      string
	BaseURL    string
	PromptPath string
	StrictJSON bool
	HTTPClient *http.Client
}

type OpenRouterError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *OpenRouterError) Error() string {
	status := strings.TrimSpace(e.Status)
	if status == "" {
		status = fmt.Sprintf("%d", e.StatusCode)
	}
	body := strings.TrimSpace(e.Body)
	if body == "" {
		return "openrouter returned " + status
	}
	return "openrouter returned " + status + ": " + body
}

func NewMatchGeneratorFromEnv() MatchGenerator {
	local := LocalMatchGenerator{}
	if os.Getenv("FUTEMON_LLM_DISABLED") == "1" {
		return local
	}
	apiKey := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	if apiKey == "" {
		return local
	}
	openRouter := NewOpenRouterMatchGeneratorFromEnv(apiKey)
	if os.Getenv("FUTEMON_LLM_FALLBACK_ON_ERROR") == "1" {
		return FallbackMatchGenerator{
			Primary:  openRouter,
			Fallback: local,
		}
	}
	return openRouter
}

func NewOpenRouterMatchGeneratorFromEnv(apiKey string) OpenRouterMatchGenerator {
	model := strings.TrimSpace(os.Getenv("OPENROUTER_MODEL"))
	if model == "" {
		model = defaultOpenRouterModel
	}
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("OPENROUTER_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = defaultOpenRouterBaseURL
	}
	promptPath := strings.TrimSpace(os.Getenv("FUTEMON_LLM_PROMPT_PATH"))
	if promptPath == "" {
		promptPath = defaultSystemPromptPath
	}
	return OpenRouterMatchGenerator{
		APIKey:     apiKey,
		Model:      model,
		BaseURL:    baseURL,
		PromptPath: promptPath,
		StrictJSON: os.Getenv("FUTEMON_LLM_STRICT_SCHEMA") == "1",
		HTTPClient: &http.Client{Timeout: openRouterTimeout()},
	}
}

func openRouterTimeout() time.Duration {
	value := strings.TrimSpace(os.Getenv("OPENROUTER_TIMEOUT_SECONDS"))
	if value == "" {
		return defaultOpenRouterTimeout
	}
	seconds, err := time.ParseDuration(value + "s")
	if err != nil || seconds <= 0 {
		return defaultOpenRouterTimeout
	}
	return seconds
}

func (g OpenRouterMatchGenerator) GenerateMatch(ctx context.Context, teamA Team, teamB Team) (MatchResult, error) {
	if strings.TrimSpace(g.APIKey) == "" {
		return MatchResult{}, errors.New("missing OpenRouter API key")
	}
	systemPrompt, err := loadSystemPrompt(g.PromptPath)
	if err != nil {
		return MatchResult{}, err
	}
	content, err := g.complete(ctx, systemPrompt, buildMatchUserPrompt(teamA, teamB, AnalyzeMatch(teamA, teamB)))
	if err != nil {
		return MatchResult{}, err
	}
	payload, err := ParseSimulationPayload([]byte(extractJSONObject(content)))
	if err != nil {
		return MatchResult{}, err
	}
	return BuildMatchFromSimulation(teamA, teamB, payload), nil
}

func (g OpenRouterMatchGenerator) complete(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	client := g.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}
	baseURL := strings.TrimRight(g.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultOpenRouterBaseURL
	}
	model := strings.TrimSpace(g.Model)
	if model == "" {
		model = defaultOpenRouterModel
	}

	requestBody := openRouterChatRequest{
		Model: model,
		Messages: []openRouterMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature:         0.85,
		MaxCompletionTokens: 2600,
	}
	if g.StrictJSON {
		requestBody.ResponseFormat = matchResponseFormat()
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+g.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "http://localhost:8080")
	req.Header.Set("X-Title", "Futemon")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &OpenRouterError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       strings.TrimSpace(string(responseBody)),
		}
	}
	var parsed openRouterChatResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", errors.New("openrouter returned an empty completion")
	}
	return parsed.Choices[0].Message.Content, nil
}

type openRouterChatRequest struct {
	Model               string              `json:"model"`
	Messages            []openRouterMessage `json:"messages"`
	Temperature         float64             `json:"temperature,omitempty"`
	MaxCompletionTokens int                 `json:"max_completion_tokens,omitempty"`
	ResponseFormat      map[string]any      `json:"response_format,omitempty"`
	Stream              bool                `json:"stream"`
}

type openRouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterChatResponse struct {
	Choices []struct {
		Message openRouterMessage `json:"message"`
	} `json:"choices"`
}

func loadSystemPrompt(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		path = defaultSystemPromptPath
	}
	data, err := os.ReadFile(path)
	if err == nil {
		return string(data), nil
	}
	if os.IsNotExist(err) && !filepath.IsAbs(path) {
		data, err = os.ReadFile(resolveDataPath(path))
		if err == nil {
			return string(data), nil
		}
	}
	return "", err
}

func resolveDataPath(relativePath string) string {
	workingDir, err := os.Getwd()
	if err != nil {
		return relativePath
	}
	for dir := workingDir; ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, relativePath)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return relativePath
}

func buildMatchUserPrompt(teamA Team, teamB Team, analysis MatchAnalysis) string {
	payload := struct {
		Instruction    string         `json:"instruction"`
		TeamA          llmTeamPayload `json:"team_a"`
		TeamB          llmTeamPayload `json:"team_b"`
		ServerAnalysis MatchAnalysis  `json:"server_analysis"`
	}{
		Instruction:    "Simule e narre esta partida de futsal Pokemon usando exatamente o formato JSON definido no system prompt. Use server_analysis como contexto factual, mas voce ainda decide o roteiro, placar e resultado.",
		TeamA:          llmTeam(teamA),
		TeamB:          llmTeam(teamB),
		ServerAnalysis: analysis,
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	return string(data)
}

type llmTeamPayload struct {
	Positions []llmPlayerPayload `json:"positions"`
}

type llmPlayerPayload struct {
	Position           string `json:"position"`
	Ref                string `json:"ref"`
	PokemonID          int    `json:"pokemon_id"`
	Name               string `json:"name"`
	Type1              string `json:"type_1"`
	Type2              string `json:"type_2,omitempty"`
	Ability            string `json:"ability"`
	AbilityDescription string `json:"ability_description,omitempty"`
	HP                 int    `json:"hp"`
	Attack             int    `json:"attack"`
	Defense            int    `json:"defense"`
	SpecialAttack      int    `json:"special_attack"`
	SpecialDefense     int    `json:"special_defense"`
	Speed              int    `json:"speed"`
	Description        string `json:"description,omitempty"`
}

func llmTeam(team Team) llmTeamPayload {
	roster := team.Roster()
	positions := make([]llmPlayerPayload, 0, len(roster))
	refs := map[string]string{
		"Goleiro":      "goleiro",
		"Fixo":         "fixo",
		"Ala Esquerda": "ala_esquerda",
		"Ala Direita":  "ala_direita",
		"Pivo":         "pivo",
	}
	for _, player := range roster {
		pokemon := player.Pokemon
		positions = append(positions, llmPlayerPayload{
			Position:           player.Position,
			Ref:                refs[player.Position],
			PokemonID:          pokemon.ID,
			Name:               pokemonDisplayName(pokemon.Name),
			Type1:              pokemon.Type1,
			Type2:              pokemon.Type2,
			Ability:            abilityDisplayName(player.Ability),
			AbilityDescription: abilityDescription(pokemon, player.Ability),
			HP:                 pokemon.HP,
			Attack:             pokemon.Attack,
			Defense:            pokemon.Defense,
			SpecialAttack:      pokemon.SpecialAttack,
			SpecialDefense:     pokemon.SpecialDefense,
			Speed:              pokemon.Speed,
			Description:        pokemon.Description,
		})
	}
	return llmTeamPayload{
		Positions: positions,
	}
}

func abilityDescription(pokemon Pokemon, selected string) string {
	selected = strings.TrimSpace(selected)
	if selected == "" {
		return ""
	}
	for _, ability := range pokemonAbilities(pokemon) {
		if strings.EqualFold(ability.Name, selected) || strings.EqualFold(abilityDisplayName(ability.Name), selected) {
			return strings.TrimSpace(ability.Description)
		}
	}
	return ""
}

func extractJSONObject(content string) string {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end >= start {
		return content[start : end+1]
	}
	return content
}

func matchResponseFormat() map[string]any {
	stringOrNull := []any{"string", "null"}
	return map[string]any{
		"type": "json_schema",
		"json_schema": map[string]any{
			"name":   "futemon_match",
			"strict": true,
			"schema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"events", "consequences"},
				"properties": map[string]any{
					"events": map[string]any{
						"type":     "array",
						"minItems": 5,
						"maxItems": 10,
						"items": map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"required":             []string{"minute", "type", "team_ref", "pokemon_ref", "narrative_build_up", "narrative_resolution"},
							"properties": map[string]any{
								"minute": map[string]any{"type": "integer", "minimum": 0, "maximum": 40},
								"type": map[string]any{
									"type": "string",
									"enum": []string{"kickoff", "close_chance", "foul", "goal", "injury", "halftime", "fulltime"},
								},
								"team_ref": map[string]any{
									"type": stringOrNull,
									"enum": []any{"team_a", "team_b", nil},
								},
								"pokemon_ref": map[string]any{
									"type": stringOrNull,
									"enum": []any{"goleiro", "goalkeeper", "fixo", "ala_esquerda", "ala_direita", "pivo", nil},
								},
								"narrative_build_up":   map[string]any{"type": "string", "minLength": 1},
								"narrative_resolution": map[string]any{"type": "string", "minLength": 1},
							},
						},
					},
					"consequences": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"required":             []string{"team_ref", "pokemon_ref", "effect_description"},
							"properties": map[string]any{
								"team_ref": map[string]any{
									"type": "string",
									"enum": []string{"team_a", "team_b"},
								},
								"pokemon_ref": map[string]any{
									"type": "string",
									"enum": []string{"goleiro", "goalkeeper", "fixo", "ala_esquerda", "ala_direita", "pivo"},
								},
								"effect_description": map[string]any{"type": "string", "minLength": 1},
							},
						},
					},
				},
			},
		},
	}
}
