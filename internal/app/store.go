package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
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
	ErrDuplicatePokemon = errors.New("duplicate pokemon")
	ErrInvalidAbility   = errors.New("invalid ability")
	ErrUserNotFound     = errors.New("user not found")
	ErrInvalidAccount   = errors.New("invalid account")
	ErrTransferLimit    = errors.New("weekly transfer limit reached")
)

type Store interface {
	CurrentUser() (User, bool)
	UserByID(id string) (User, bool)
	UpsertGoogleUser(profile GoogleProfile) (User, error)
	UpdateAccount(input AccountInput) (User, error)
	Pokemon() []Pokemon
	MyTeams(userID string) []Team
	RetiredTeams(userID string) []Team
	GlobalTeams(sortBy string) []Team
	Tournaments() []Tournament
	FindTeam(id string) (Team, bool)
	FindTeamIncludingRetired(id string) (Team, bool)
	TeamHistory(teamID string) []MatchSummary
	TeamTransfers(teamID string) []TeamTransfer
	TransferWindow(teamID string) TransferWindow
	SaveTeam(input TeamInput) (Team, error)
	DeleteTeam(id string, userID string) error
	LatestMatch() (MatchResult, bool)
	MatchByID(id string) (MatchResult, bool)
	SetLatestMatch(match MatchResult)
}

type MemoryStore struct {
	pokemon     []Pokemon
	myTeams     []Team
	globalTeams []Team
	tournaments []Tournament
	matches     []MatchResult
	transfers   []TeamTransfer
	latestMatch *MatchResult
	user        User
}

func NewMemoryStore() *MemoryStore {
	pokemon := samplePokemon()
	teamA := Team{
		ID:                 "team-kanto-press",
		UserID:             "user-demo",
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
		CreatedAt:          time.Now().Add(-72 * time.Hour),
	}
	teamB := Team{
		ID:                 "team-paleta-bolada",
		UserID:             "user-rival",
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
		IsFrozen:           true,
		CreatedAt:          time.Now().Add(-48 * time.Hour),
	}
	teamC := Team{
		ID:                 "team-ginasio-azul",
		UserID:             "user-misty",
		Name:               "Ginasio Azul FC",
		Goalkeeper:         pokemon[130],
		GoalkeeperAbility:  "intimidate",
		Fixo:               pokemon[55],
		FixoAbility:        "cloud-nine",
		AlaEsquerda:        pokemon[121],
		AlaEsquerdaAbility: "natural-cure",
		AlaDireita:         pokemon[7],
		AlaDireitaAbility:  "torrent",
		Pivo:               pokemon[131],
		PivoAbility:        "water-absorb",
		CreatedAt:          time.Now().Add(-24 * time.Hour),
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

func (s *MemoryStore) UserByID(id string) (User, bool) {
	if id == s.user.ID {
		return s.user, true
	}
	return User{}, false
}

func (s *MemoryStore) UpsertGoogleUser(profile GoogleProfile) (User, error) {
	s.user.GoogleID = profile.GoogleID
	s.user.DisplayName = profile.DisplayName
	s.user.Email = profile.Email
	s.user.PictureURL = profile.PictureURL
	return s.user, nil
}

func (s *MemoryStore) UpdateAccount(input AccountInput) (User, error) {
	if strings.TrimSpace(input.DisplayName) == "" {
		return User{}, ErrInvalidAccount
	}
	s.user.DisplayName = strings.TrimSpace(input.DisplayName)
	s.user.AvatarIcon = normalizeAvatarIcon(input.AvatarIcon)
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

func (s *MemoryStore) MyTeams(userID string) []Team {
	var teams []Team
	for _, team := range s.myTeams {
		if team.UserID == userID && !team.IsRetired {
			teams = append(teams, team)
		}
	}
	return s.withRecords(teams)
}

func (s *MemoryStore) RetiredTeams(userID string) []Team {
	var teams []Team
	for _, team := range s.globalTeams {
		if team.UserID == userID && team.IsRetired {
			teams = append(teams, team)
		}
	}
	return s.withRecords(teams)
}

func (s *MemoryStore) GlobalTeams(sortBy string) []Team {
	var active []Team
	for _, team := range s.globalTeams {
		if !team.IsRetired {
			active = append(active, team)
		}
	}
	teams := s.withRecords(active)
	sortTeams(teams, sortBy)
	return teams
}

func (s *MemoryStore) Tournaments() []Tournament {
	return s.tournaments
}

func (s *MemoryStore) FindTeam(id string) (Team, bool) {
	for _, team := range s.globalTeams {
		if strings.EqualFold(team.ID, id) && !team.IsRetired {
			return s.withRecord(team), true
		}
	}
	return Team{}, false
}

func (s *MemoryStore) FindTeamIncludingRetired(id string) (Team, bool) {
	for _, team := range s.globalTeams {
		if strings.EqualFold(team.ID, id) {
			return s.withRecord(team), true
		}
	}
	return Team{}, false
}

func (s *MemoryStore) TeamHistory(teamID string) []MatchSummary {
	now := time.Now()
	var summaries []MatchSummary
	for _, match := range s.matches {
		if match.TeamA.ID != teamID && match.TeamB.ID != teamID {
			continue
		}
		if !matchFinished(match, now) {
			continue
		}
		summaries = append(summaries, matchSummaryForTeam(match, teamID))
	}
	sort.SliceStable(summaries, func(i, j int) bool {
		return summaries[i].PlayedAt.After(summaries[j].PlayedAt)
	})
	return summaries
}

func (s *MemoryStore) TeamTransfers(teamID string) []TeamTransfer {
	var transfers []TeamTransfer
	for _, transfer := range s.transfers {
		if transfer.TeamID == teamID {
			transfers = append(transfers, transfer)
		}
	}
	sort.SliceStable(transfers, func(i, j int) bool {
		return transfers[i].CreatedAt.Before(transfers[j].CreatedAt)
	})
	return transfers
}

func (s *MemoryStore) TransferWindow(teamID string) TransferWindow {
	window := currentTransferWindow(time.Now())
	for _, transfer := range s.transfers {
		if transfer.TeamID == teamID && transfer.Kind == "pokemon_transfer" && transfer.WindowStart.Equal(window.Start) {
			window.Used = true
			window.Remaining = 0
			return window
		}
	}
	window.Remaining = 1
	return window
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
		if existing.IsRetired {
			return Team{}, ErrTeamNotFound
		}
		pokemonChanged := teamPokemonChanged(existing, team)
		if pokemonChanged && s.TransferWindow(existing.ID).Used {
			return Team{}, ErrTransferLimit
		}
		team.CreatedAt = existing.CreatedAt
		s.myTeams[i] = team
		for j, global := range s.globalTeams {
			if global.ID == team.ID {
				s.globalTeams[j] = team
			}
		}
		if pokemonChanged {
			s.recordTransfer(existing, team, "pokemon_transfer", time.Now())
		}
		return team, nil
	}

	activeTeams := 0
	for _, existing := range s.myTeams {
		if existing.UserID == input.UserID && !existing.IsRetired {
			activeTeams++
		}
	}
	if activeTeams >= 6 {
		return Team{}, ErrTeamLimitReached
	}
	team.ID = newID("team")
	team.CreatedAt = time.Now().UTC()
	s.myTeams = append([]Team{team}, s.myTeams...)
	s.globalTeams = append([]Team{team}, s.globalTeams...)
	s.recordTransfer(Team{}, team, "formation_created", team.CreatedAt)
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
		team.IsRetired = true
		s.myTeams[i] = team
		for j, global := range s.globalTeams {
			if global.ID == id {
				s.globalTeams[j].IsRetired = true
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

func (s *MemoryStore) MatchByID(id string) (MatchResult, bool) {
	for _, match := range s.matches {
		if match.ID == id {
			return match, true
		}
	}
	return MatchResult{}, false
}

func teamFromInput(input TeamInput, pokemonByID map[int]Pokemon) (Team, error) {
	input, err := normalizeTeamInput(input, pokemonByID)
	if err != nil {
		return Team{}, err
	}

	return Team{
		ID:                 input.ID,
		UserID:             input.UserID,
		Name:               strings.TrimSpace(input.Name),
		Goalkeeper:         pokemonByID[input.GoalkeeperID],
		GoalkeeperAbility:  input.GoalkeeperAbility,
		Fixo:               pokemonByID[input.FixoID],
		FixoAbility:        input.FixoAbility,
		AlaEsquerda:        pokemonByID[input.AlaEsquerdaID],
		AlaEsquerdaAbility: input.AlaEsquerdaAbility,
		AlaDireita:         pokemonByID[input.AlaDireitaID],
		AlaDireitaAbility:  input.AlaDireitaAbility,
		Pivo:               pokemonByID[input.PivoID],
		PivoAbility:        input.PivoAbility,
	}, nil
}

func normalizeTeamInput(input TeamInput, pokemonByID map[int]Pokemon) (TeamInput, error) {
	positions := []struct {
		id      int
		ability *string
	}{
		{input.GoalkeeperID, &input.GoalkeeperAbility},
		{input.FixoID, &input.FixoAbility},
		{input.AlaEsquerdaID, &input.AlaEsquerdaAbility},
		{input.AlaDireitaID, &input.AlaDireitaAbility},
		{input.PivoID, &input.PivoAbility},
	}
	seen := map[int]bool{}
	for _, position := range positions {
		pokemon, ok := pokemonByID[position.id]
		if !ok {
			return TeamInput{}, ErrPokemonNotFound
		}
		if seen[position.id] {
			return TeamInput{}, ErrDuplicatePokemon
		}
		seen[position.id] = true
		ability, ok := normalizePokemonAbility(pokemon, *position.ability)
		if !ok {
			return TeamInput{}, ErrInvalidAbility
		}
		*position.ability = ability
	}
	return input, nil
}

func (s *MemoryStore) SetLatestMatch(match MatchResult) {
	if match.EndTime.IsZero() {
		match.EndTime = match.StartTime.Add(matchDuration(match.Events))
	}
	s.latestMatch = &match
	s.matches = append(s.matches, match)
}

func (s *MemoryStore) withRecord(team Team) Team {
	team.Record = teamRecordFromMatches(team.ID, s.matches, time.Now())
	team.LeaderboardScore = leaderboardScore(team.Record)
	return team
}

func (s *MemoryStore) withRecords(teams []Team) []Team {
	for i := range teams {
		teams[i] = s.withRecord(teams[i])
	}
	return teams
}

func (s *MemoryStore) recordTransfer(before Team, after Team, kind string, now time.Time) {
	window := currentTransferWindow(now)
	transfer := TeamTransfer{
		ID:             newID("transfer"),
		TeamID:         after.ID,
		Kind:           kind,
		Before:         before,
		After:          after,
		WindowStart:    window.Start,
		CreatedAt:      now.UTC(),
		ChangedPlayers: changedPlayers(before, after),
	}
	transfer.Summary = transferSummary(transfer)
	s.transfers = append(s.transfers, transfer)
}

func values(roster map[int]Pokemon) []Pokemon {
	out := make([]Pokemon, 0, len(roster))
	for _, pokemon := range roster {
		out = append(out, ensurePokemonArtwork(pokemon))
	}
	return out
}

func sortTeams(teams []Team, sortBy string) {
	switch sortBy {
	case "", "best":
		sort.SliceStable(teams, func(i, j int) bool {
			if teams[i].LeaderboardScore == teams[j].LeaderboardScore {
				if teams[i].Record.Played == teams[j].Record.Played {
					return teams[i].CreatedAt.After(teams[j].CreatedAt)
				}
				return teams[i].Record.Played > teams[j].Record.Played
			}
			return teams[i].LeaderboardScore > teams[j].LeaderboardScore
		})
	default:
		sort.SliceStable(teams, func(i, j int) bool {
			return teams[i].CreatedAt.After(teams[j].CreatedAt)
		})
	}
}

func leaderboardScore(record TeamRecord) float64 {
	if record.Played == 0 {
		return 0
	}
	points := float64(record.Wins) + (0.5 * float64(record.Draws))
	rawRate := points / float64(record.Played)
	volumeWeight := float64(record.Played) / float64(record.Played+3)
	return rawRate * volumeWeight
}

func teamRecordFromMatches(teamID string, matches []MatchResult, now time.Time) TeamRecord {
	var record TeamRecord
	for _, match := range matches {
		if match.TeamA.ID != teamID && match.TeamB.ID != teamID {
			continue
		}
		if !matchFinished(match, now) {
			continue
		}
		record.Played++
		result := matchResultForTeam(match, teamID)
		switch result {
		case "win":
			record.Wins++
		case "draw":
			record.Draws++
		case "loss":
			record.Losses++
		}
	}
	return record
}

func matchFinished(match MatchResult, now time.Time) bool {
	end := match.EndTime
	if end.IsZero() {
		end = match.StartTime.Add(matchDuration(match.Events))
	}
	return !end.IsZero() && !now.Before(end)
}

func matchResultForTeam(match MatchResult, teamID string) string {
	scoreA, scoreB := match.Score()
	if scoreA == scoreB {
		return "draw"
	}
	switch teamID {
	case match.TeamA.ID:
		if scoreA > scoreB {
			return "win"
		}
	case match.TeamB.ID:
		if scoreB > scoreA {
			return "win"
		}
	}
	return "loss"
}

func matchSummaryForTeam(match MatchResult, teamID string) MatchSummary {
	scoreA, scoreB := match.Score()
	summary := MatchSummary{
		ID:         match.ID,
		TeamAName:  match.TeamA.Name,
		TeamBName:  match.TeamB.Name,
		ScoreTeamA: scoreA,
		ScoreTeamB: scoreB,
		PlayedAt:   match.EndTime,
		TeamResult: matchResultForTeam(match, teamID),
	}
	if summary.PlayedAt.IsZero() {
		summary.PlayedAt = match.StartTime.Add(matchDuration(match.Events))
	}
	if teamID == match.TeamB.ID {
		summary.TeamScoreLine = fmt.Sprintf("%d x %d", scoreB, scoreA)
	} else {
		summary.TeamScoreLine = fmt.Sprintf("%d x %d", scoreA, scoreB)
	}
	summary.GoalsTeamA, summary.GoalsTeamB = goalSummariesForMatch(match)
	return summary
}

func currentTransferWindow(now time.Time) TransferWindow {
	local := now.In(time.Local)
	year, month, day := local.Date()
	midnight := time.Date(year, month, day, 0, 0, 0, 0, local.Location())
	start := midnight.AddDate(0, 0, -int(local.Weekday()))
	return TransferWindow{
		Start:     start,
		End:       start.AddDate(0, 0, 7),
		Remaining: 1,
	}
}

func teamPokemonChanged(before Team, after Team) bool {
	return len(changedPlayers(before, after)) > 0
}

func changedPlayers(before Team, after Team) []PlayerTransfer {
	if before.ID == "" {
		return nil
	}
	beforeRoster := before.Roster()
	afterRoster := after.Roster()
	changes := make([]PlayerTransfer, 0, len(afterRoster))
	for i := range afterRoster {
		if i >= len(beforeRoster) {
			continue
		}
		if beforeRoster[i].Pokemon.ID == afterRoster[i].Pokemon.ID {
			continue
		}
		changes = append(changes, PlayerTransfer{
			Position: afterRoster[i].Position,
			From:     beforeRoster[i].Pokemon,
			To:       afterRoster[i].Pokemon,
		})
	}
	return changes
}

func transferSummary(transfer TeamTransfer) string {
	switch transfer.Kind {
	case "formation_created":
		return "Formacao original registrada."
	case "pokemon_transfer":
		if len(transfer.ChangedPlayers) == 0 {
			return "Elenco mantido."
		}
		parts := make([]string, 0, len(transfer.ChangedPlayers))
		for _, change := range transfer.ChangedPlayers {
			parts = append(parts, fmt.Sprintf("%s: %s por %s", change.Position, pokemonDisplayName(change.From.Name), pokemonDisplayName(change.To.Name)))
		}
		return strings.Join(parts, "; ")
	default:
		return "Alteracao registrada."
	}
}

func goalSummariesForMatch(match MatchResult) ([]MatchGoalSummary, []MatchGoalSummary) {
	var teamA []MatchGoalSummary
	var teamB []MatchGoalSummary
	for _, event := range match.Events {
		if event.Type != "goal" {
			continue
		}
		goal := MatchGoalSummary{
			Minute:      event.Minute,
			TeamID:      event.TeamID,
			PokemonID:   event.PokemonID,
			PokemonName: pokemonDisplayName(pokemonNameForEvent(match, event.PokemonID)),
		}
		switch event.TeamID {
		case match.TeamA.ID:
			goal.TeamName = match.TeamA.Name
			teamA = append(teamA, goal)
		case match.TeamB.ID:
			goal.TeamName = match.TeamB.Name
			teamB = append(teamB, goal)
		}
	}
	return teamA, teamB
}

func sampleAbilities(names ...string) string {
	abilities := make([]PokemonAbility, 0, len(names))
	for _, name := range names {
		abilities = append(abilities, PokemonAbility{Name: name})
	}
	payload, err := json.Marshal(abilities)
	if err != nil {
		return "[]"
	}
	return string(payload)
}

func samplePokemon() map[int]Pokemon {
	pokemon := map[int]Pokemon{
		4:   {ID: 4, Name: "Charmander", Type1: "Fire", HP: 39, Attack: 52, Defense: 43, SpecialAttack: 60, SpecialDefense: 50, Speed: 65, Description: "Leve, rapido e perigosamente inflamavel em quadra.", Abilities: sampleAbilities("blaze", "solar-power")},
		6:   {ID: 6, Name: "Charizard", Type1: "Fire", Type2: "Flying", HP: 78, Attack: 84, Defense: 78, SpecialAttack: 109, SpecialDefense: 85, Speed: 100, Description: "Aereo demais para respeitar linhas laterais.", Abilities: sampleAbilities("blaze", "solar-power")},
		7:   {ID: 7, Name: "Squirtle", Type1: "Water", HP: 44, Attack: 48, Defense: 65, SpecialAttack: 50, SpecialDefense: 64, Speed: 43, Description: "Baixo centro de gravidade e casco util em bloqueios.", Abilities: sampleAbilities("torrent", "rain-dish")},
		9:   {ID: 9, Name: "Blastoise", Type1: "Water", HP: 79, Attack: 83, Defense: 100, SpecialAttack: 85, SpecialDefense: 105, Speed: 78, Description: "Goleiro de area ampla com canhoes anti-chute.", Abilities: sampleAbilities("torrent", "rain-dish")},
		25:  {ID: 25, Name: "Pikachu", Type1: "Electric", HP: 35, Attack: 55, Defense: 40, SpecialAttack: 50, SpecialDefense: 50, Speed: 90, Description: "Aceleracao curta e tomada eletrica de decisao.", Abilities: sampleAbilities("static", "lightning-rod")},
		26:  {ID: 26, Name: "Raichu", Type1: "Electric", HP: 60, Attack: 90, Defense: 55, SpecialAttack: 90, SpecialDefense: 80, Speed: 110, Description: "Ala velocista com chute de media distancia.", Abilities: sampleAbilities("static", "lightning-rod")},
		55:  {ID: 55, Name: "Golduck", Type1: "Water", HP: 80, Attack: 82, Defense: 78, SpecialAttack: 95, SpecialDefense: 80, Speed: 85, Description: "Boa leitura de jogo e serenidade suspeita.", Abilities: sampleAbilities("damp", "cloud-nine", "swift-swim")},
		68:  {ID: 68, Name: "Machamp", Type1: "Fighting", HP: 90, Attack: 130, Defense: 80, SpecialAttack: 65, SpecialDefense: 85, Speed: 55, Description: "Quatro bracos, uma interpretacao elastica da regra de mao.", Abilities: sampleAbilities("guts", "no-guard", "steadfast")},
		95:  {ID: 95, Name: "Onix", Type1: "Rock", Type2: "Ground", HP: 35, Attack: 45, Defense: 160, SpecialAttack: 30, SpecialDefense: 45, Speed: 70, Description: "Defende por obstrucao geologica.", Abilities: sampleAbilities("rock-head", "sturdy", "weak-armor")},
		121: {ID: 121, Name: "Starmie", Type1: "Water", Type2: "Psychic", HP: 60, Attack: 75, Defense: 85, SpecialAttack: 100, SpecialDefense: 85, Speed: 115, Description: "Sem pe, mas com geometria ofensiva elite.", Abilities: sampleAbilities("illuminate", "natural-cure", "analytic")},
		130: {ID: 130, Name: "Gyarados", Type1: "Water", Type2: "Flying", HP: 95, Attack: 125, Defense: 79, SpecialAttack: 60, SpecialDefense: 100, Speed: 81, Description: "Intimida atacantes e tambem a arbitragem.", Abilities: sampleAbilities("intimidate", "moxie")},
		131: {ID: 131, Name: "Lapras", Type1: "Water", Type2: "Ice", HP: 130, Attack: 85, Defense: 80, SpecialAttack: 85, SpecialDefense: 95, Speed: 60, Description: "Pivo de parede, literalmente.", Abilities: sampleAbilities("water-absorb", "shell-armor", "hydration")},
		143: {ID: 143, Name: "Snorlax", Type1: "Normal", HP: 160, Attack: 110, Defense: 65, SpecialAttack: 65, SpecialDefense: 110, Speed: 30, Description: "Ocupa o gol e parte do conceito de gol.", Abilities: sampleAbilities("immunity", "thick-fat", "gluttony")},
		149: {ID: 149, Name: "Dragonite", Type1: "Dragon", Type2: "Flying", HP: 91, Attack: 134, Defense: 95, SpecialAttack: 100, SpecialDefense: 100, Speed: 80, Description: "Craque completo, embora excessivamente gentil.", Abilities: sampleAbilities("inner-focus", "multiscale")},
	}
	for id, item := range pokemon {
		pokemon[id] = ensurePokemonArtwork(item)
	}
	return pokemon
}
