package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const defaultSampleMatchPath = "examples/sample_match.json"

type SimulationPayload struct {
	Events       []SimulationEvent       `json:"events"`
	Consequences []SimulationConsequence `json:"consequences"`
}

type SimulationEvent struct {
	Minute              int    `json:"minute"`
	Type                string `json:"type"`
	Narrative           string `json:"narrative,omitempty"`
	NarrativeBuildUp    string `json:"narrative_build_up,omitempty"`
	NarrativeResolution string `json:"narrative_resolution,omitempty"`
	TeamRef             string `json:"team_ref,omitempty"`
	PokemonRef          string `json:"pokemon_ref,omitempty"`
}

type SimulationConsequence struct {
	TeamRef           string `json:"team_ref"`
	PokemonRef        string `json:"pokemon_ref"`
	EffectDescription string `json:"effect_description"`
}

func SimulateMatch(teamA Team, teamB Team) MatchResult {
	payload, err := LoadSimulationPayload(sampleMatchPath())
	if err != nil {
		payload = SimulationPayload{
			Events: []SimulationEvent{
				{Minute: 0, Type: "kickoff", Narrative: "A partida comeca sem roteiro de simulacao disponivel."},
				{Minute: 40, Type: "fulltime", Narrative: "Fim de jogo sem eventos registrados."},
			},
		}
	}
	return BuildMatchFromSimulation(teamA, teamB, payload)
}

func LoadSimulationPayload(path string) (SimulationPayload, error) {
	if path == "" {
		path = resolveSampleMatchPath()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return SimulationPayload{}, err
	}
	return ParseSimulationPayload(data)
}

func ParseSimulationPayload(data []byte) (SimulationPayload, error) {
	var payload SimulationPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return SimulationPayload{}, err
	}
	normalizeSimulationPayload(&payload)
	if err := ValidateSimulationPayload(payload); err != nil {
		return SimulationPayload{}, err
	}
	return payload, nil
}

func BuildMatchFromSimulation(teamA Team, teamB Team, payload SimulationPayload) MatchResult {
	events := make([]MatchEvent, 0, len(payload.Events))
	for _, event := range payload.Events {
		teamID, pokemonID := resolveSimulationRefs(teamA, teamB, event.TeamRef, event.PokemonRef)
		events = append(events, MatchEvent{
			Minute:    event.Minute,
			Type:      event.Type,
			Narrative: renderSimulationText(teamA, teamB, event.Narrative),
			TeamID:    teamID,
			PokemonID: pokemonID,
		})
	}

	consequences := make([]MatchConsequence, 0, len(payload.Consequences))
	for _, consequence := range payload.Consequences {
		teamID, pokemonID := resolveSimulationRefs(teamA, teamB, consequence.TeamRef, consequence.PokemonRef)
		consequences = append(consequences, MatchConsequence{
			TeamID:            teamID,
			PokemonID:         pokemonID,
			EffectDescription: renderSimulationText(teamA, teamB, consequence.EffectDescription),
		})
	}

	match := MatchResult{
		ID:           fmt.Sprintf("match-%d", time.Now().UnixNano()),
		TeamA:        teamA,
		TeamB:        teamB,
		StartTime:    time.Now(),
		Events:       events,
		Consequences: consequences,
	}
	match.ScoreTeamA, match.ScoreTeamB = match.Score()
	return match
}

func renderSimulationText(teamA Team, teamB Team, text string) string {
	if text == "" {
		return ""
	}
	replacements := []string{}
	appendTeamReplacements := func(prefix string, team Team) {
		replacements = append(replacements,
			"{{"+prefix+".name}}", team.Name,
			"{{"+prefix+".goleiro}}", pokemonDisplayName(team.Goalkeeper.Name),
			"{{"+prefix+".fixo}}", pokemonDisplayName(team.Fixo.Name),
			"{{"+prefix+".ala_esquerda}}", pokemonDisplayName(team.AlaEsquerda.Name),
			"{{"+prefix+".ala_direita}}", pokemonDisplayName(team.AlaDireita.Name),
			"{{"+prefix+".pivo}}", pokemonDisplayName(team.Pivo.Name),
		)
	}
	appendTeamReplacements("team_a", teamA)
	appendTeamReplacements("team_b", teamB)
	return strings.NewReplacer(replacements...).Replace(text)
}

func normalizeSimulationPayload(payload *SimulationPayload) {
	for i := range payload.Events {
		if strings.TrimSpace(payload.Events[i].Narrative) == "" {
			payload.Events[i].Narrative = joinNarrativeParts(payload.Events[i].NarrativeBuildUp, payload.Events[i].NarrativeResolution)
		}
		payload.Events[i].Narrative = normalizeTeamNamePlaceholders(payload.Events[i].Narrative)
		payload.Events[i].NarrativeBuildUp = normalizeTeamNamePlaceholders(payload.Events[i].NarrativeBuildUp)
		payload.Events[i].NarrativeResolution = normalizeTeamNamePlaceholders(payload.Events[i].NarrativeResolution)
		payload.Events[i].Type = strings.TrimSpace(payload.Events[i].Type)
		payload.Events[i].TeamRef = normalizeNullableRef(payload.Events[i].TeamRef)
		payload.Events[i].PokemonRef = normalizeNullableRef(payload.Events[i].PokemonRef)
	}
	for i := range payload.Consequences {
		payload.Consequences[i].EffectDescription = normalizeTeamNamePlaceholders(payload.Consequences[i].EffectDescription)
	}
	sort.SliceStable(payload.Events, func(i int, j int) bool {
		return payload.Events[i].Minute < payload.Events[j].Minute
	})
}

var (
	teamANameReferencePattern = regexp.MustCompile(`(?i)\b(?:time|team)\s+a\b|\bteam_a\b`)
	teamBNameReferencePattern = regexp.MustCompile(`(?i)\b(?:time|team)\s+b\b|\bteam_b\b`)
)

func normalizeTeamNamePlaceholders(text string) string {
	if text == "" {
		return ""
	}
	segments := placeholderPattern.Split(text, -1)
	placeholders := placeholderPattern.FindAllString(text, -1)
	var out strings.Builder
	for i, segment := range segments {
		segment = teamANameReferencePattern.ReplaceAllString(segment, "{{team_a.name}}")
		segment = teamBNameReferencePattern.ReplaceAllString(segment, "{{team_b.name}}")
		out.WriteString(segment)
		if i < len(placeholders) {
			out.WriteString(placeholders[i])
		}
	}
	return out.String()
}

var placeholderPattern = regexp.MustCompile(`\{\{[^{}]+\}\}`)

func ValidateSimulationPayload(payload SimulationPayload) error {
	if len(payload.Events) < 3 {
		return errors.New("simulation payload must contain at least kickoff, halftime and fulltime")
	}
	required := map[string]int{"kickoff": 0, "halftime": 20, "fulltime": 40}
	found := map[string]bool{}
	for _, event := range payload.Events {
		if strings.TrimSpace(event.Narrative) == "" {
			return fmt.Errorf("event at minute %d has empty narrative", event.Minute)
		}
		if !validEventType(event.Type) {
			return fmt.Errorf("invalid event type %q", event.Type)
		}
		if wantMinute, ok := required[event.Type]; ok && event.Minute == wantMinute {
			found[event.Type] = true
		}
	}
	for eventType := range required {
		if !found[eventType] {
			return fmt.Errorf("missing required %s event", eventType)
		}
	}
	return nil
}

func validEventType(eventType string) bool {
	switch eventType {
	case "kickoff", "close_chance", "foul", "goal", "injury", "halftime", "fulltime":
		return true
	default:
		return false
	}
}

func joinNarrativeParts(buildUp string, resolution string) string {
	buildUp = strings.TrimSpace(buildUp)
	resolution = strings.TrimSpace(resolution)
	switch {
	case buildUp == "":
		return resolution
	case resolution == "":
		return buildUp
	default:
		return buildUp + " " + resolution
	}
}

func normalizeNullableRef(value string) string {
	value = strings.TrimSpace(value)
	if value == "null" {
		return ""
	}
	return value
}

func resolveSimulationRefs(teamA Team, teamB Team, teamRef string, pokemonRef string) (string, int) {
	team := Team{}
	switch teamRef {
	case "team_a":
		team = teamA
	case "team_b":
		team = teamB
	default:
		return "", 0
	}

	pokemonID := 0
	switch pokemonRef {
	case "goleiro":
		pokemonID = team.Goalkeeper.ID
	case "fixo":
		pokemonID = team.Fixo.ID
	case "ala_esquerda":
		pokemonID = team.AlaEsquerda.ID
	case "ala_direita":
		pokemonID = team.AlaDireita.ID
	case "pivo":
		pokemonID = team.Pivo.ID
	}
	return team.ID, pokemonID
}

func sampleMatchPath() string {
	if path := os.Getenv("FUTEMON_SAMPLE_MATCH_JSON"); path != "" {
		return path
	}
	return resolveSampleMatchPath()
}

func resolveSampleMatchPath() string {
	workingDir, err := os.Getwd()
	if err != nil {
		return defaultSampleMatchPath
	}
	for dir := workingDir; ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, defaultSampleMatchPath)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return defaultSampleMatchPath
}
