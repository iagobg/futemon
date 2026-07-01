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
	Overall       OverallMatchAnalysis   `json:"geral"`
	PhaseMatchups []PhaseMatchupAnalysis `json:"confrontos"`
}

type OverallMatchAnalysis struct {
	TeamAPower      float64 `json:"forca_time_da_casa"`
	TeamBPower      float64 `json:"forca_time_visitante"`
	TeamAPowerIndex float64 `json:"indice_time_da_casa"`
	TeamBPowerIndex float64 `json:"indice_time_visitante"`
	PowerGapPercent float64 `json:"diferenca_percentual_forca"`
	Favorite        string  `json:"favorito_no_papel"`
	Confidence      string  `json:"confianca"`
	Summary         string  `json:"resumo"`
}

type PhaseMatchupAnalysis struct {
	Label        string  `json:"rotulo"`
	AttackRef    string  `json:"ataque_ref"`
	DefenseRef   string  `json:"defesa_ref"`
	AttackScore  float64 `json:"nota_ataque"`
	DefenseScore float64 `json:"nota_defesa"`
	Advantage    string  `json:"vantagem"`
	Summary      string  `json:"resumo"`
}

func BuildServerMatchContext(teamA Team, teamB Team) ServerMatchContext {
	return ServerMatchContext{
		Analysis: AnalyzeMatch(teamA, teamB),
	}
}

func AnalyzeMatch(teamA Team, teamB Team) MatchAnalysis {
	teamAPower := teamPower(teamA)
	teamBPower := teamPower(teamB)
	teamAPowerIndex, teamBPowerIndex := powerIndexes(teamAPower, teamBPower)
	gapPercent := powerGapPercent(teamAPower, teamBPower)
	favorite := "empate"
	if teamAPower > teamBPower {
		favorite = "time_da_casa"
	}
	if teamBPower > teamAPower {
		favorite = "time_visitante"
	}
	confidence := confidenceLabel(teamAPower, teamBPower)
	return MatchAnalysis{
		Overall: OverallMatchAnalysis{
			TeamAPower:      round1(teamAPower),
			TeamBPower:      round1(teamBPower),
			TeamAPowerIndex: round1(teamAPowerIndex),
			TeamBPowerIndex: round1(teamBPowerIndex),
			PowerGapPercent: round1(gapPercent),
			Favorite:        favorite,
			Confidence:      confidence,
			Summary:         overallSummary(teamA, teamB, teamAPower, teamBPower, teamAPowerIndex, teamBPowerIndex, gapPercent, favorite, confidence),
		},
		PhaseMatchups: []PhaseMatchupAnalysis{
			comparePhaseMatchup("Ataque do Time da Casa vs defesa do Time Visitante", "ataque_time_da_casa", teamA, "defesa_time_visitante", teamB),
			comparePhaseMatchup("Ataque do Time Visitante vs defesa do Time da Casa", "ataque_time_visitante", teamB, "defesa_time_da_casa", teamA),
		},
	}
}

func teamPower(team Team) float64 {
	return positionPower("goleiro", team.Goalkeeper) +
		positionPower("fixo", team.Fixo) +
		positionPower("wing", team.AlaEsquerda) +
		positionPower("wing", team.AlaDireita) +
		positionPower("pivo", team.Pivo)
}

func positionPower(role string, pokemon Pokemon) float64 {
	switch role {
	case "goleiro":
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
		{role: "goleiro", pokemon: defenseTeam.Goalkeeper},
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
		return "neutro"
	}
	if diff > 0 {
		return "ataque"
	}
	return "defesa"
}

func phaseSummary(attackRef string, defenseRef string, attackScore float64, defenseScore float64, advantage string) string {
	return fmt.Sprintf("%s contra %s: ataque %.1f vs defesa %.1f, vantagem %s.", attackRef, defenseRef, attackScore, defenseScore, advantage)
}

func overallSummary(teamA Team, teamB Team, powerA float64, powerB float64, indexA float64, indexB float64, gapPercent float64, favorite string, confidence string) string {
	return fmt.Sprintf("Time da Casa %.1f (indice %.1f) vs %.1f (indice %.1f) Time Visitante; diferenca %.1f%%; favorito no papel: %s (%s).", powerA, indexA, powerB, indexB, gapPercent, favorite, confidence)
}

func powerIndexes(powerA float64, powerB float64) (float64, float64) {
	average := (powerA + powerB) / 2
	if average <= 0 {
		return 100, 100
	}
	return (powerA / average) * 100, (powerB / average) * 100
}

func powerGapPercent(powerA float64, powerB float64) float64 {
	lower := math.Min(powerA, powerB)
	if lower <= 0 {
		return 0
	}
	return (math.Abs(powerA-powerB) / lower) * 100
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
