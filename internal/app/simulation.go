package app

import (
	"fmt"
	"time"
)

func SimulateMatch(teamA Team, teamB Team) MatchResult {
	aPower := teamPower(teamA)
	bPower := teamPower(teamB)
	scoreA := 1 + (aPower % 3)
	scoreB := bPower % 3

	if scoreA == scoreB {
		if teamA.Pivo.Attack+teamA.AlaDireita.Speed >= teamB.Pivo.Attack+teamB.AlaDireita.Speed {
			scoreA++
		} else {
			scoreB++
		}
	}

	events := []MatchEvent{
		{Minute: 0, Type: "kickoff", DramaticPauseSeconds: 1, Narrative: fmt.Sprintf("Apita o arbitro. %s e %s entram em quadra como se isso fosse perfeitamente normal.", teamA.Name, teamB.Name)},
		{Minute: 7, Type: "close_chance", DramaticPauseSeconds: 2, TeamID: teamA.ID, PokemonID: teamA.AlaDireita.ID, Narrative: fmt.Sprintf("%s acelera pela ala e descobre que futsal tambem exige freio.", teamA.AlaDireita.Name)},
		{Minute: 14, Type: "foul", DramaticPauseSeconds: 2, TeamID: teamB.ID, PokemonID: teamB.Fixo.ID, Narrative: fmt.Sprintf("%s aplica uma marcacao que o VAR classificou como fenomeno natural.", teamB.Fixo.Name)},
		{Minute: 20, Type: "halftime", DramaticPauseSeconds: 5, Narrative: fmt.Sprintf("Intervalo no ginasio. %s tenta organizar a saida de bola, enquanto %s discute seriamente se anatomia deveria constar na sumula.", teamA.Name, teamB.Name)},
	}
	events = append(events, goalEvents(teamA, teamB, scoreA, scoreB)...)
	events = append(events,
		MatchEvent{Minute: 31, Type: "injury", DramaticPauseSeconds: 3, TeamID: loserID(teamA, teamB, scoreA, scoreB), PokemonID: trailingFixo(teamA, teamB, scoreA, scoreB).ID, Narrative: fmt.Sprintf("%s sente o ritmo. O departamento medico recomenda agua, gelo e uma conversa franca com a PokeAPI.", trailingFixo(teamA, teamB, scoreA, scoreB).Name)},
		MatchEvent{Minute: 40, Type: "fulltime", DramaticPauseSeconds: 4, Narrative: fmt.Sprintf("Fim de jogo. Uma partida taticamente discutivel e espiritualmente impecavel.")},
	)

	match := MatchResult{
		ID:        fmt.Sprintf("match-%d", time.Now().Unix()),
		TeamA:     teamA,
		TeamB:     teamB,
		StartTime: time.Now(),
		Events:    events,
		Consequences: []MatchConsequence{
			{PokemonID: trailingFixo(teamA, teamB, scoreA, scoreB).ID, TeamID: loserID(teamA, teamB, scoreA, scoreB), EffectDescription: "Chega ao proximo jogo com leve desconfianca de divididas."},
		},
	}
	match.ScoreTeamA, match.ScoreTeamB = match.Score()
	return match
}

func teamPower(team Team) int {
	return team.Goalkeeper.Defense + team.Fixo.Defense + team.AlaEsquerda.Speed + team.AlaDireita.Speed + team.Pivo.Attack + team.Pivo.SpecialAttack
}

func goalEvents(teamA Team, teamB Team, scoreA int, scoreB int) []MatchEvent {
	events := make([]MatchEvent, 0, scoreA+scoreB)
	minutes := []int{23, 27, 34, 36, 38, 39}
	scorersA := []Pokemon{teamA.Pivo, teamA.AlaDireita, teamA.AlaEsquerda}
	scorersB := []Pokemon{teamB.Pivo, teamB.AlaDireita, teamB.AlaEsquerda}
	index := 0
	for i := 0; i < scoreA; i++ {
		scorer := scorersA[i%len(scorersA)]
		events = append(events, MatchEvent{Minute: minutes[index%len(minutes)], Type: "goal", DramaticPauseSeconds: 5, TeamID: teamA.ID, PokemonID: scorer.ID, Narrative: fmt.Sprintf("Gol do %s. %s finaliza com a conviccao de quem leu o regulamento e discordou.", teamA.Name, scorer.Name)})
		index++
	}
	for i := 0; i < scoreB; i++ {
		scorer := scorersB[i%len(scorersB)]
		events = append(events, MatchEvent{Minute: minutes[index%len(minutes)], Type: "goal", DramaticPauseSeconds: 5, TeamID: teamB.ID, PokemonID: scorer.ID, Narrative: fmt.Sprintf("Gol do %s. %s acha um espaco que a defesa jurava nao existir.", teamB.Name, scorer.Name)})
		index++
	}
	return events
}

func winnerName(teamA Team, teamB Team, scoreA int, scoreB int) string {
	if scoreA >= scoreB {
		return teamA.Name
	}
	return teamB.Name
}

func loserID(teamA Team, teamB Team, scoreA int, scoreB int) string {
	if scoreA >= scoreB {
		return teamB.ID
	}
	return teamA.ID
}

func leadingPivo(teamA Team, teamB Team, scoreA int, scoreB int) Pokemon {
	if scoreA >= scoreB {
		return teamA.Pivo
	}
	return teamB.Pivo
}

func trailingFixo(teamA Team, teamB Team, scoreA int, scoreB int) Pokemon {
	if scoreA >= scoreB {
		return teamB.Fixo
	}
	return teamA.Fixo
}
