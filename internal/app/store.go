package app

import (
	"errors"
	"strings"
	"time"
)

const demoUserID = "user-demo"

var (
	ErrTeamLimitReached = errors.New("team limit reached")
	ErrTeamFrozen       = errors.New("team is frozen")
	ErrTeamNotFound     = errors.New("team not found")
	ErrPokemonNotFound  = errors.New("pokemon not found")
	ErrInvalidTeam      = errors.New("invalid team")
	ErrUserNotFound     = errors.New("user not found")
	ErrInvalidAccount   = errors.New("invalid account")
)

type Store interface {
	CurrentUser() (User, bool)
	UpdateAccount(input AccountInput) (User, error)
	Pokemon() []Pokemon
	MyTeams() []Team
	GlobalTeams() []Team
	Tournaments() []Tournament
	FindTeam(id string) (Team, bool)
	SaveTeam(input TeamInput) (Team, error)
	DeleteTeam(id string, userID string) error
	LatestMatch() (MatchResult, bool)
	SetLatestMatch(match MatchResult)
}

type MemoryStore struct {
	pokemon     []Pokemon
	myTeams     []Team
	globalTeams []Team
	tournaments []Tournament
	latestMatch *MatchResult
	user        User
}

func NewMemoryStore() *MemoryStore {
	pokemon := samplePokemon()
	teamA := Team{
		ID:          "team-kanto-press",
		UserID:      "user-demo",
		Name:        "Kanto Press",
		Goalkeeper:  pokemon[9],
		Fixo:        pokemon[6],
		AlaEsquerda: pokemon[25],
		AlaDireita:  pokemon[4],
		Pivo:        pokemon[68],
		CreatedAt:   time.Now().Add(-72 * time.Hour),
	}
	teamB := Team{
		ID:          "team-paleta-bolada",
		UserID:      "user-rival",
		Name:        "Paleta Bolada",
		Goalkeeper:  pokemon[143],
		Fixo:        pokemon[95],
		AlaEsquerda: pokemon[26],
		AlaDireita:  pokemon[7],
		Pivo:        pokemon[149],
		IsFrozen:    true,
		CreatedAt:   time.Now().Add(-48 * time.Hour),
	}
	teamC := Team{
		ID:          "team-ginasio-azul",
		UserID:      "user-misty",
		Name:        "Ginasio Azul FC",
		Goalkeeper:  pokemon[130],
		Fixo:        pokemon[55],
		AlaEsquerda: pokemon[121],
		AlaDireita:  pokemon[7],
		Pivo:        pokemon[131],
		CreatedAt:   time.Now().Add(-24 * time.Hour),
	}

	return &MemoryStore{
		user:        User{ID: demoUserID, GoogleID: "demo-google-id", DisplayName: "Treinador Demo", Email: "demo@futemon.local", Role: "admin"},
		pokemon:     values(pokemon),
		myTeams:     []Team{teamA},
		globalTeams: []Team{teamA, teamB, teamC},
		tournaments: []Tournament{
			{ID: "tourn-001", Name: "Copa Professor Carvalho", Status: "registration", Teams: []Team{teamA, teamC}},
			{ID: "tourn-002", Name: "Liga dos Centros Pokemon", Status: "active", Teams: []Team{teamB, teamC}},
		},
	}
}

func (s *MemoryStore) CurrentUser() (User, bool) {
	return s.user, true
}

func (s *MemoryStore) UpdateAccount(input AccountInput) (User, error) {
	if strings.TrimSpace(input.DisplayName) == "" {
		return User{}, ErrInvalidAccount
	}
	s.user.DisplayName = strings.TrimSpace(input.DisplayName)
	if input.ClearAPIKey {
		s.user.GeminiAPIKey = ""
		s.user.HasGeminiAPIKey = false
	} else if input.GeminiAPIKey != "" {
		s.user.GeminiAPIKey = input.GeminiAPIKey
		s.user.HasGeminiAPIKey = true
	}
	return s.user, nil
}

func (s *MemoryStore) Pokemon() []Pokemon {
	return s.pokemon
}

func (s *MemoryStore) MyTeams() []Team {
	return s.myTeams
}

func (s *MemoryStore) GlobalTeams() []Team {
	return s.globalTeams
}

func (s *MemoryStore) Tournaments() []Tournament {
	return s.tournaments
}

func (s *MemoryStore) FindTeam(id string) (Team, bool) {
	for _, team := range s.globalTeams {
		if strings.EqualFold(team.ID, id) {
			return team, true
		}
	}
	return Team{}, false
}

func (s *MemoryStore) SaveTeam(input TeamInput) (Team, error) {
	if strings.TrimSpace(input.Name) == "" {
		return Team{}, ErrInvalidTeam
	}
	if input.UserID == "" {
		input.UserID = demoUserID
	}
	pokemonByID := make(map[int]Pokemon, len(s.pokemon))
	for _, pokemon := range s.pokemon {
		pokemonByID[pokemon.ID] = pokemon
	}
	team, err := teamFromInput(input, pokemonByID)
	if err != nil {
		return Team{}, err
	}

	for i, existing := range s.myTeams {
		if existing.ID != input.ID {
			continue
		}
		if existing.IsFrozen {
			return Team{}, ErrTeamFrozen
		}
		team.CreatedAt = existing.CreatedAt
		s.myTeams[i] = team
		for j, global := range s.globalTeams {
			if global.ID == team.ID {
				s.globalTeams[j] = team
			}
		}
		return team, nil
	}

	if len(s.myTeams) >= 6 {
		return Team{}, ErrTeamLimitReached
	}
	team.ID = newID("team")
	team.CreatedAt = time.Now().UTC()
	s.myTeams = append([]Team{team}, s.myTeams...)
	s.globalTeams = append([]Team{team}, s.globalTeams...)
	return team, nil
}

func (s *MemoryStore) DeleteTeam(id string, userID string) error {
	for i, team := range s.myTeams {
		if team.ID != id || team.UserID != userID {
			continue
		}
		if team.IsFrozen {
			return ErrTeamFrozen
		}
		s.myTeams = append(s.myTeams[:i], s.myTeams[i+1:]...)
		for j, global := range s.globalTeams {
			if global.ID == id {
				s.globalTeams = append(s.globalTeams[:j], s.globalTeams[j+1:]...)
				break
			}
		}
		return nil
	}
	return ErrTeamNotFound
}

func (s *MemoryStore) LatestMatch() (MatchResult, bool) {
	if s.latestMatch == nil {
		return MatchResult{}, false
	}
	return *s.latestMatch, true
}

func teamFromInput(input TeamInput, pokemonByID map[int]Pokemon) (Team, error) {
	goalkeeper, ok := pokemonByID[input.GoalkeeperID]
	if !ok {
		return Team{}, ErrPokemonNotFound
	}
	fixo, ok := pokemonByID[input.FixoID]
	if !ok {
		return Team{}, ErrPokemonNotFound
	}
	alaEsquerda, ok := pokemonByID[input.AlaEsquerdaID]
	if !ok {
		return Team{}, ErrPokemonNotFound
	}
	alaDireita, ok := pokemonByID[input.AlaDireitaID]
	if !ok {
		return Team{}, ErrPokemonNotFound
	}
	pivo, ok := pokemonByID[input.PivoID]
	if !ok {
		return Team{}, ErrPokemonNotFound
	}

	return Team{
		ID:          input.ID,
		UserID:      input.UserID,
		Name:        strings.TrimSpace(input.Name),
		Goalkeeper:  goalkeeper,
		Fixo:        fixo,
		AlaEsquerda: alaEsquerda,
		AlaDireita:  alaDireita,
		Pivo:        pivo,
	}, nil
}

func (s *MemoryStore) SetLatestMatch(match MatchResult) {
	s.latestMatch = &match
}

func values(roster map[int]Pokemon) []Pokemon {
	out := make([]Pokemon, 0, len(roster))
	for _, pokemon := range roster {
		out = append(out, pokemon)
	}
	return out
}

func samplePokemon() map[int]Pokemon {
	return map[int]Pokemon{
		4:   {ID: 4, Name: "Charmander", Type1: "Fire", HP: 39, Attack: 52, Defense: 43, SpecialAttack: 60, SpecialDefense: 50, Speed: 65, Description: "Leve, rapido e perigosamente inflamavel em quadra."},
		6:   {ID: 6, Name: "Charizard", Type1: "Fire", Type2: "Flying", HP: 78, Attack: 84, Defense: 78, SpecialAttack: 109, SpecialDefense: 85, Speed: 100, Description: "Aereo demais para respeitar linhas laterais."},
		7:   {ID: 7, Name: "Squirtle", Type1: "Water", HP: 44, Attack: 48, Defense: 65, SpecialAttack: 50, SpecialDefense: 64, Speed: 43, Description: "Baixo centro de gravidade e casco util em bloqueios."},
		9:   {ID: 9, Name: "Blastoise", Type1: "Water", HP: 79, Attack: 83, Defense: 100, SpecialAttack: 85, SpecialDefense: 105, Speed: 78, Description: "Goleiro de area ampla com canhoes anti-chute."},
		25:  {ID: 25, Name: "Pikachu", Type1: "Electric", HP: 35, Attack: 55, Defense: 40, SpecialAttack: 50, SpecialDefense: 50, Speed: 90, Description: "Aceleracao curta e tomada eletrica de decisao."},
		26:  {ID: 26, Name: "Raichu", Type1: "Electric", HP: 60, Attack: 90, Defense: 55, SpecialAttack: 90, SpecialDefense: 80, Speed: 110, Description: "Ala velocista com chute de media distancia."},
		55:  {ID: 55, Name: "Golduck", Type1: "Water", HP: 80, Attack: 82, Defense: 78, SpecialAttack: 95, SpecialDefense: 80, Speed: 85, Description: "Boa leitura de jogo e serenidade suspeita."},
		68:  {ID: 68, Name: "Machamp", Type1: "Fighting", HP: 90, Attack: 130, Defense: 80, SpecialAttack: 65, SpecialDefense: 85, Speed: 55, Description: "Quatro bracos, uma interpretacao elastica da regra de mao."},
		95:  {ID: 95, Name: "Onix", Type1: "Rock", Type2: "Ground", HP: 35, Attack: 45, Defense: 160, SpecialAttack: 30, SpecialDefense: 45, Speed: 70, Description: "Defende por obstrucao geologica."},
		121: {ID: 121, Name: "Starmie", Type1: "Water", Type2: "Psychic", HP: 60, Attack: 75, Defense: 85, SpecialAttack: 100, SpecialDefense: 85, Speed: 115, Description: "Sem pe, mas com geometria ofensiva elite."},
		130: {ID: 130, Name: "Gyarados", Type1: "Water", Type2: "Flying", HP: 95, Attack: 125, Defense: 79, SpecialAttack: 60, SpecialDefense: 100, Speed: 81, Description: "Intimida atacantes e tambem a arbitragem."},
		131: {ID: 131, Name: "Lapras", Type1: "Water", Type2: "Ice", HP: 130, Attack: 85, Defense: 80, SpecialAttack: 85, SpecialDefense: 95, Speed: 60, Description: "Pivo de parede, literalmente."},
		143: {ID: 143, Name: "Snorlax", Type1: "Normal", HP: 160, Attack: 110, Defense: 65, SpecialAttack: 65, SpecialDefense: 110, Speed: 30, Description: "Ocupa o gol e parte do conceito de gol."},
		149: {ID: 149, Name: "Dragonite", Type1: "Dragon", Type2: "Flying", HP: 91, Attack: 134, Defense: 95, SpecialAttack: 100, SpecialDefense: 100, Speed: 80, Description: "Craque completo, embora excessivamente gentil."},
	}
}
