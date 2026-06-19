package app

import (
	"fmt"
	"math"
	"strings"
)

type ServerMatchContext struct {
	Analysis MatchAnalysis `json:"analysis"`
}

type MatchAnalysis struct {
	Overall     OverallMatchAnalysis `json:"overall"`
	KeyMatchups []KeyMatchupAnalysis `json:"key_matchups"`
}

type OverallMatchAnalysis struct {
	TeamAPower float64 `json:"team_a_power"`
	TeamBPower float64 `json:"team_b_power"`
	Favorite   string  `json:"favorite"`
	Confidence string  `json:"confidence"`
	Summary    string  `json:"summary"`
}

type KeyMatchupAnalysis struct {
	Label          string  `json:"label"`
	TeamARef       string  `json:"team_a_ref"`
	TeamBRef       string  `json:"team_b_ref"`
	TeamAPokemon   string  `json:"team_a_pokemon"`
	TeamBPokemon   string  `json:"team_b_pokemon"`
	TeamAStatScore float64 `json:"team_a_stat_score"`
	TeamBStatScore float64 `json:"team_b_stat_score"`
	TeamATypeEdge  float64 `json:"team_a_type_edge"`
	TeamBTypeEdge  float64 `json:"team_b_type_edge"`
	Edge           string  `json:"edge"`
	Summary        string  `json:"summary"`
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
		KeyMatchups: []KeyMatchupAnalysis{
			comparePlayers("Ala esquerda team_a vs ala direita team_b", "team_a.ala_esquerda", teamA.AlaEsquerda, "team_b.ala_direita", teamB.AlaDireita, "wing"),
			comparePlayers("Ala direita team_a vs ala esquerda team_b", "team_a.ala_direita", teamA.AlaDireita, "team_b.ala_esquerda", teamB.AlaEsquerda, "wing"),
			comparePlayers("Pivo team_a vs fixo team_b", "team_a.pivo", teamA.Pivo, "team_b.fixo", teamB.Fixo, "pivot_vs_fixo"),
			comparePlayers("Pivo team_b vs fixo team_a", "team_b.pivo", teamB.Pivo, "team_a.fixo", teamA.Fixo, "pivot_vs_fixo"),
			compareGoalkeeper("Goleiro team_a vs finalizadores team_b", "team_a.goleiro", teamA.Goalkeeper, "team_b.finalizadores", []Pokemon{teamB.AlaEsquerda, teamB.AlaDireita, teamB.Pivo}),
			compareGoalkeeper("Goleiro team_b vs finalizadores team_a", "team_b.goleiro", teamB.Goalkeeper, "team_a.finalizadores", []Pokemon{teamA.AlaEsquerda, teamA.AlaDireita, teamA.Pivo}),
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

func comparePlayers(label string, refA string, pokemonA Pokemon, refB string, pokemonB Pokemon, role string) KeyMatchupAnalysis {
	scoreA := positionPower(rolePowerRole(refA, role), pokemonA)
	scoreB := positionPower(rolePowerRole(refB, role), pokemonB)
	typeA := typeEffectiveness(pokemonA, pokemonB)
	typeB := typeEffectiveness(pokemonB, pokemonA)
	edge := matchupEdge(scoreA, scoreB, typeA, typeB)
	return KeyMatchupAnalysis{
		Label:          label,
		TeamARef:       refA,
		TeamBRef:       refB,
		TeamAPokemon:   pokemonDisplayName(pokemonA.Name),
		TeamBPokemon:   pokemonDisplayName(pokemonB.Name),
		TeamAStatScore: round1(scoreA),
		TeamBStatScore: round1(scoreB),
		TeamATypeEdge:  typeA,
		TeamBTypeEdge:  typeB,
		Edge:           edge,
		Summary:        matchupSummary(pokemonA, pokemonB, refA, refB, scoreA, scoreB, typeA, typeB, edge),
	}
}

func compareGoalkeeper(label string, keeperRef string, keeper Pokemon, attackersRef string, attackers []Pokemon) KeyMatchupAnalysis {
	keeperScore := positionPower("goalkeeper", keeper)
	var attackScore float64
	var typePressure float64
	var names []string
	for _, attacker := range attackers {
		attackScore += positionPower("pivo", attacker)
		typePressure += typeEffectiveness(attacker, keeper)
		names = append(names, pokemonDisplayName(attacker.Name))
	}
	attackScore = attackScore / float64(len(attackers))
	typePressure = round2(typePressure / float64(len(attackers)))
	keeperEdge := round2(typeEffectiveness(keeper, attackers[0]))
	edge := matchupEdge(keeperScore, attackScore, keeperEdge, typePressure)
	summary := fmt.Sprintf("%s encara media ofensiva de %s; defesa/leitura %.1f contra pressao %.1f, pressao de tipo media %.2fx.", pokemonDisplayName(keeper.Name), strings.Join(names, ", "), keeperScore, attackScore, typePressure)
	return KeyMatchupAnalysis{
		Label:          label,
		TeamARef:       keeperRef,
		TeamBRef:       attackersRef,
		TeamAPokemon:   pokemonDisplayName(keeper.Name),
		TeamBPokemon:   strings.Join(names, ", "),
		TeamAStatScore: round1(keeperScore),
		TeamBStatScore: round1(attackScore),
		TeamATypeEdge:  keeperEdge,
		TeamBTypeEdge:  typePressure,
		Edge:           edge,
		Summary:        summary,
	}
}

func rolePowerRole(ref string, fallback string) string {
	switch {
	case strings.Contains(ref, ".pivo"):
		return "pivo"
	case strings.Contains(ref, ".fixo"):
		return "fixo"
	case strings.Contains(ref, ".goleiro"):
		return "goalkeeper"
	default:
		return fallback
	}
}

func matchupEdge(scoreA float64, scoreB float64, typeA float64, typeB float64) string {
	valueA := scoreA * (0.85 + (typeA * 0.15))
	valueB := scoreB * (0.85 + (typeB * 0.15))
	diff := valueA - valueB
	threshold := math.Max(valueA, valueB) * 0.06
	if math.Abs(diff) <= threshold {
		return "neutral"
	}
	if diff > 0 {
		return "team_a"
	}
	return "team_b"
}

func matchupSummary(pokemonA Pokemon, pokemonB Pokemon, refA string, refB string, scoreA float64, scoreB float64, typeA float64, typeB float64, edge string) string {
	return fmt.Sprintf("%s (%s) contra %s (%s): stats %.1f vs %.1f, tipos %.2fx vs %.2fx, vantagem %s.", pokemonDisplayName(pokemonA.Name), refA, pokemonDisplayName(pokemonB.Name), refB, scoreA, scoreB, typeA, typeB, edge)
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
