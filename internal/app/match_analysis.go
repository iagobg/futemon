package app

import (
	"fmt"
	"math"
	"strings"
)

const typeInfluenceWeight = 0.50

type ServerMatchContext struct {
	Analysis MatchAnalysis `json:"analysis"`
}

type MatchAnalysis struct {
	Overall       OverallMatchAnalysis   `json:"overall"`
	PhaseMatchups []PhaseMatchupAnalysis `json:"phase_matchups"`
}

type OverallMatchAnalysis struct {
	TeamAPower float64 `json:"team_a_power"`
	TeamBPower float64 `json:"team_b_power"`
	Favorite   string  `json:"favorite"`
	Confidence string  `json:"confidence"`
	Summary    string  `json:"summary"`
}

type PhaseMatchupAnalysis struct {
	Label        string  `json:"label"`
	AttackRef    string  `json:"attack_ref"`
	DefenseRef   string  `json:"defense_ref"`
	AttackScore  float64 `json:"attack_score"`
	DefenseScore float64 `json:"defense_score"`
	Advantage    string  `json:"advantage"`
	Summary      string  `json:"summary"`
}

func BuildServerMatchContext(teamA Team, teamB Team) ServerMatchContext {
	return ServerMatchContext{
		Analysis: AnalyzeMatch(teamA, teamB),
	}
}

func AnalyzeMatch(teamA Team, teamB Team) MatchAnalysis {
	teamAPower := teamPower(teamA)
	teamBPower := teamPower(teamB)
	favorite := "draw"
	if teamAPower > teamBPower {
		favorite = "team_a"
	}
	if teamBPower > teamAPower {
		favorite = "team_b"
	}
	confidence := confidenceLabel(teamAPower, teamBPower)
	return MatchAnalysis{
		Overall: OverallMatchAnalysis{
			TeamAPower: round1(teamAPower),
			TeamBPower: round1(teamBPower),
			Favorite:   favorite,
			Confidence: confidence,
			Summary:    overallSummary(teamA, teamB, teamAPower, teamBPower, favorite, confidence),
		},
		PhaseMatchups: []PhaseMatchupAnalysis{
			comparePhaseMatchup("Ataque team_a vs defesa team_b", "team_a.attack", teamA, "team_b.defense", teamB),
			comparePhaseMatchup("Ataque team_b vs defesa team_a", "team_b.attack", teamB, "team_a.defense", teamA),
		},
	}
}

func teamPower(team Team) float64 {
	return positionPower("goalkeeper", team.Goalkeeper) +
		positionPower("fixo", team.Fixo) +
		positionPower("wing", team.AlaEsquerda) +
		positionPower("wing", team.AlaDireita) +
		positionPower("pivo", team.Pivo)
}

func positionPower(role string, pokemon Pokemon) float64 {
	switch role {
	case "goalkeeper":
		return float64(pokemon.HP)*0.8 + float64(pokemon.Defense)*1.45 + float64(pokemon.SpecialDefense)*1.45 + float64(pokemon.Speed)*0.45 + float64(pokemon.Attack+pokemon.SpecialAttack)*0.25
	case "fixo":
		return float64(pokemon.HP)*0.75 + float64(pokemon.Defense)*1.35 + float64(pokemon.SpecialDefense)*1.25 + float64(pokemon.Speed)*0.65 + float64(pokemon.Attack+pokemon.SpecialAttack)*0.55
	case "pivo":
		return float64(pokemon.Attack)*1.35 + float64(pokemon.SpecialAttack)*1.05 + float64(pokemon.HP)*0.8 + float64(pokemon.Defense)*0.85 + float64(pokemon.Speed)*0.65 + float64(pokemon.SpecialDefense)*0.45
	default:
		return float64(pokemon.Speed)*1.35 + float64(pokemon.SpecialAttack)*1.05 + float64(pokemon.Attack)*0.85 + float64(pokemon.Defense+pokemon.SpecialDefense)*0.65 + float64(pokemon.HP)*0.45
	}
}

func comparePhaseMatchup(label string, attackRef string, attackTeam Team, defenseRef string, defenseTeam Team) PhaseMatchupAnalysis {
	attackers := []Pokemon{attackTeam.AlaEsquerda, attackTeam.AlaDireita, attackTeam.Pivo}
	defenders := []Pokemon{defenseTeam.Fixo, defenseTeam.Goalkeeper}
	attackScore := averagePositionPower([]positionPokemon{
		{role: "wing", pokemon: attackTeam.AlaEsquerda},
		{role: "wing", pokemon: attackTeam.AlaDireita},
		{role: "pivo", pokemon: attackTeam.Pivo},
	})
	defenseScore := averagePositionPower([]positionPokemon{
		{role: "fixo", pokemon: defenseTeam.Fixo},
		{role: "goalkeeper", pokemon: defenseTeam.Goalkeeper},
	})
	typePressure := aggregateTypePressure(attackers, defenders)
	advantage := phaseAdvantage(attackScore, defenseScore, typePressure)
	return PhaseMatchupAnalysis{
		Label:        label,
		AttackRef:    attackRef,
		DefenseRef:   defenseRef,
		AttackScore:  round1(attackScore),
		DefenseScore: round1(defenseScore),
		Advantage:    advantage,
		Summary:      phaseSummary(attackRef, defenseRef, attackScore, defenseScore, advantage),
	}
}

type positionPokemon struct {
	role    string
	pokemon Pokemon
}

func averagePositionPower(players []positionPokemon) float64 {
	if len(players) == 0 {
		return 0
	}
	var total float64
	for _, player := range players {
		total += positionPower(player.role, player.pokemon)
	}
	return total / float64(len(players))
}

func aggregateTypePressure(attackers []Pokemon, defenders []Pokemon) float64 {
	if len(attackers) == 0 || len(defenders) == 0 {
		return 1
	}
	var total float64
	var count int
	for _, attacker := range attackers {
		for _, defender := range defenders {
			total += typeEffectiveness(attacker, defender)
			count++
		}
	}
	return round2(total / float64(count))
}

func phaseAdvantage(attackScore float64, defenseScore float64, typePressure float64) string {
	attackValue := attackScore * ((1 - typeInfluenceWeight) + (typePressure * typeInfluenceWeight))
	diff := attackValue - defenseScore
	threshold := math.Max(attackValue, defenseScore) * 0.06
	if math.Abs(diff) <= threshold {
		return "neutral"
	}
	if diff > 0 {
		return "attack"
	}
	return "defense"
}

func phaseSummary(attackRef string, defenseRef string, attackScore float64, defenseScore float64, advantage string) string {
	return fmt.Sprintf("%s contra %s: ataque %.1f vs defesa %.1f, vantagem %s.", attackRef, defenseRef, attackScore, defenseScore, advantage)
}

func overallSummary(teamA Team, teamB Team, powerA float64, powerB float64, favorite string, confidence string) string {
	return fmt.Sprintf("team_a %.1f vs %.1f team_b; favorito: %s (%s).", powerA, powerB, favorite, confidence)
}

func confidenceLabel(powerA float64, powerB float64) string {
	total := math.Max(1, powerA+powerB)
	gap := math.Abs(powerA-powerB) / total
	switch {
	case gap < 0.035:
		return "equilibrado"
	case gap < 0.09:
		return "leve"
	case gap < 0.16:
		return "claro"
	default:
		return "dominante"
	}
}

func typeEffectiveness(attacker Pokemon, defender Pokemon) float64 {
	best := typeAttackEffectiveness(attacker.Type1, defender)
	if attacker.Type2 != "" {
		if second := typeAttackEffectiveness(attacker.Type2, defender); second > best {
			best = second
		}
	}
	return round2(best)
}

func typeAttackEffectiveness(attackType string, defender Pokemon) float64 {
	attackType = normalizeTypeName(attackType)
	if attackType == "" {
		return 1
	}
	multiplier := defensiveMultiplier(attackType, defender.Type1)
	if defender.Type2 != "" {
		multiplier *= defensiveMultiplier(attackType, defender.Type2)
	}
	return multiplier
}

func defensiveMultiplier(attackType string, defenseType string) float64 {
	attackType = normalizeTypeName(attackType)
	defenseType = normalizeTypeName(defenseType)
	if attackType == "" || defenseType == "" {
		return 1
	}
	if matchups, ok := pokemonTypeChart[attackType]; ok {
		if multiplier, ok := matchups[defenseType]; ok {
			return multiplier
		}
	}
	return 1
}

func normalizeTypeName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func round1(value float64) float64 {
	return math.Round(value*10) / 10
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

var pokemonTypeChart = map[string]map[string]float64{
	"normal":   {"rock": 0.5, "ghost": 0, "steel": 0.5},
	"fire":     {"fire": 0.5, "water": 0.5, "grass": 2, "ice": 2, "bug": 2, "rock": 0.5, "dragon": 0.5, "steel": 2},
	"water":    {"fire": 2, "water": 0.5, "grass": 0.5, "ground": 2, "rock": 2, "dragon": 0.5},
	"electric": {"water": 2, "electric": 0.5, "grass": 0.5, "ground": 0, "flying": 2, "dragon": 0.5},
	"grass":    {"fire": 0.5, "water": 2, "grass": 0.5, "poison": 0.5, "ground": 2, "flying": 0.5, "bug": 0.5, "rock": 2, "dragon": 0.5, "steel": 0.5},
	"ice":      {"fire": 0.5, "water": 0.5, "grass": 2, "ice": 0.5, "ground": 2, "flying": 2, "dragon": 2, "steel": 0.5},
	"fighting": {"normal": 2, "ice": 2, "poison": 0.5, "flying": 0.5, "psychic": 0.5, "bug": 0.5, "rock": 2, "ghost": 0, "dark": 2, "steel": 2, "fairy": 0.5},
	"poison":   {"grass": 2, "poison": 0.5, "ground": 0.5, "rock": 0.5, "ghost": 0.5, "steel": 0, "fairy": 2},
	"ground":   {"fire": 2, "electric": 2, "grass": 0.5, "poison": 2, "flying": 0, "bug": 0.5, "rock": 2, "steel": 2},
	"flying":   {"electric": 0.5, "grass": 2, "fighting": 2, "bug": 2, "rock": 0.5, "steel": 0.5},
	"psychic":  {"fighting": 2, "poison": 2, "psychic": 0.5, "dark": 0, "steel": 0.5},
	"bug":      {"fire": 0.5, "grass": 2, "fighting": 0.5, "poison": 0.5, "flying": 0.5, "psychic": 2, "ghost": 0.5, "dark": 2, "steel": 0.5, "fairy": 0.5},
	"rock":     {"fire": 2, "ice": 2, "fighting": 0.5, "ground": 0.5, "flying": 2, "bug": 2, "steel": 0.5},
	"ghost":    {"normal": 0, "psychic": 2, "ghost": 2, "dark": 0.5},
	"dragon":   {"dragon": 2, "steel": 0.5, "fairy": 0},
	"dark":     {"fighting": 0.5, "psychic": 2, "ghost": 2, "dark": 0.5, "fairy": 0.5},
	"steel":    {"fire": 0.5, "water": 0.5, "electric": 0.5, "ice": 2, "rock": 2, "steel": 0.5, "fairy": 2},
	"fairy":    {"fire": 0.5, "fighting": 2, "poison": 0.5, "dragon": 2, "dark": 2, "steel": 0.5},
}
