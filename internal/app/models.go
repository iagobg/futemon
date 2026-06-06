package app

import "time"

type Pokemon struct {
	ID             int
	Name           string
	Type1          string
	Type2          string
	HP             int
	Attack         int
	Defense        int
	SpecialAttack  int
	SpecialDefense int
	Speed          int
	Description    string
	Abilities      string
}

type User struct {
	ID              string
	GoogleID        string
	DisplayName     string
	Email           string
	GeminiAPIKey    string
	Role            string
	HasGeminiAPIKey bool
}

type AccountInput struct {
	UserID       string
	DisplayName  string
	GeminiAPIKey string
	ClearAPIKey  bool
}

type Team struct {
	ID          string
	UserID      string
	Name        string
	Goalkeeper  Pokemon
	Fixo        Pokemon
	AlaEsquerda Pokemon
	AlaDireita  Pokemon
	Pivo        Pokemon
	IsFrozen    bool
	CreatedAt   time.Time
}

type TeamInput struct {
	ID            string
	UserID        string
	Name          string
	GoalkeeperID  int
	FixoID        int
	AlaEsquerdaID int
	AlaDireitaID  int
	PivoID        int
}

func (t Team) Roster() []PositionedPokemon {
	return []PositionedPokemon{
		{Position: "Goleiro", Pokemon: t.Goalkeeper},
		{Position: "Fixo", Pokemon: t.Fixo},
		{Position: "Ala Esquerda", Pokemon: t.AlaEsquerda},
		{Position: "Ala Direita", Pokemon: t.AlaDireita},
		{Position: "Pivo", Pokemon: t.Pivo},
	}
}

type PositionedPokemon struct {
	Position string
	Pokemon  Pokemon
}

type Tournament struct {
	ID     string
	Name   string
	Status string
	Teams  []Team
}

type MatchEvent struct {
	Minute               int    `json:"minute"`
	Type                 string `json:"type"`
	Narrative            string `json:"narrative"`
	DramaticPauseSeconds int    `json:"dramatic_pause_seconds"`
	TeamID               string `json:"team_id,omitempty"`
	PokemonID            int    `json:"pokemon_id,omitempty"`
}

type MatchConsequence struct {
	PokemonID         int    `json:"pokemon_id"`
	TeamID            string `json:"team_id"`
	EffectDescription string `json:"effect_description"`
}

type MatchResult struct {
	ID           string             `json:"-"`
	TeamA        Team               `json:"-"`
	TeamB        Team               `json:"-"`
	ScoreTeamA   int                `json:"-"`
	ScoreTeamB   int                `json:"-"`
	Events       []MatchEvent       `json:"events"`
	Consequences []MatchConsequence `json:"consequences"`
	StartTime    time.Time          `json:"-"`
}

func (m MatchResult) Score() (int, int) {
	var teamA int
	var teamB int
	for _, event := range m.Events {
		if event.Type != "goal" {
			continue
		}
		switch event.TeamID {
		case m.TeamA.ID:
			teamA++
		case m.TeamB.ID:
			teamB++
		}
	}
	return teamA, teamB
}
