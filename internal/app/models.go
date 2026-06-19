package app

import (
	"fmt"
	"time"
)

type Pokemon struct {
	ID              int
	Name            string
	ArtworkURL      string
	LocalArtworkURL string
	Type1           string
	Type2           string
	HP              int
	Attack          int
	Defense         int
	SpecialAttack   int
	SpecialDefense  int
	Speed           int
	Description     string
	Abilities       string
}

func (p Pokemon) DisplayArtworkURL() string {
	if p.LocalArtworkURL != "" {
		return p.LocalArtworkURL
	}
	return p.ArtworkURL
}

type User struct {
	ID              string
	GoogleID        string
	DisplayName     string
	Email           string
	PictureURL      string
	AvatarIcon      int
	GeminiAPIKey    string
	Role            string
	HasGeminiAPIKey bool
}

type AccountInput struct {
	UserID       string
	DisplayName  string
	AvatarIcon   int
	GeminiAPIKey string
	ClearAPIKey  bool
}

type Team struct {
	ID                 string
	UserID             string
	Name               string
	Goalkeeper         Pokemon
	GoalkeeperAbility  string
	Fixo               Pokemon
	FixoAbility        string
	AlaEsquerda        Pokemon
	AlaEsquerdaAbility string
	AlaDireita         Pokemon
	AlaDireitaAbility  string
	Pivo               Pokemon
	PivoAbility        string
	IsFrozen           bool
	IsRetired          bool
	CreatedAt          time.Time
	Record             TeamRecord
	LeaderboardScore   float64
}

type TeamRecord struct {
	Wins   int
	Draws  int
	Losses int
	Played int
}

type TransferWindow struct {
	Start     time.Time
	End       time.Time
	Used      bool
	Remaining int
}

type TeamTransfer struct {
	ID             string
	TeamID         string
	Kind           string
	Summary        string
	Before         Team
	After          Team
	WindowStart    time.Time
	CreatedAt      time.Time
	ChangedPlayers []PlayerTransfer
}

type PlayerTransfer struct {
	Position string
	From     Pokemon
	To       Pokemon
}

func (r TeamRecord) Label() string {
	if r.Played == 0 {
		return "0J"
	}
	return fmt.Sprintf("%dV %dE %dD", r.Wins, r.Draws, r.Losses)
}

func (r TeamRecord) WinPercent() int {
	if r.Played == 0 {
		return 0
	}
	return (r.Wins * 100) / r.Played
}

type TeamInput struct {
	ID                 string
	UserID             string
	Name               string
	GoalkeeperID       int
	GoalkeeperAbility  string
	FixoID             int
	FixoAbility        string
	AlaEsquerdaID      int
	AlaEsquerdaAbility string
	AlaDireitaID       int
	AlaDireitaAbility  string
	PivoID             int
	PivoAbility        string
}

func (t Team) Roster() []PositionedPokemon {
	return []PositionedPokemon{
		{Position: "Goleiro", Pokemon: t.Goalkeeper, Ability: t.GoalkeeperAbility},
		{Position: "Fixo", Pokemon: t.Fixo, Ability: t.FixoAbility},
		{Position: "Ala Esquerda", Pokemon: t.AlaEsquerda, Ability: t.AlaEsquerdaAbility},
		{Position: "Ala Direita", Pokemon: t.AlaDireita, Ability: t.AlaDireitaAbility},
		{Position: "Pivo", Pokemon: t.Pivo, Ability: t.PivoAbility},
	}
}

type PositionedPokemon struct {
	Position string
	Pokemon  Pokemon
	Ability  string
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

type MatchSyncState struct {
	MatchID      string `json:"match_id"`
	MatchVersion string `json:"match_version"`
	Status       string `json:"status"`
	ServerNowMS  int64  `json:"server_now_ms"`
	StartTimeMS  int64  `json:"start_time_ms"`
	EndedAtMS    int64  `json:"ended_at_ms"`
}

type MatchSummary struct {
	ID            string
	TeamAName     string
	TeamBName     string
	ScoreTeamA    int
	ScoreTeamB    int
	PlayedAt      time.Time
	TeamResult    string
	TeamScoreLine string
	GoalsTeamA    []MatchGoalSummary
	GoalsTeamB    []MatchGoalSummary
}

type MatchGoalSummary struct {
	Minute      int
	TeamID      string
	TeamName    string
	PokemonID   int
	PokemonName string
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
	EndTime      time.Time          `json:"-"`
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
