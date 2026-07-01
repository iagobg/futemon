package app

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteStoreSeedsDemoData(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	pokemonCount, err := countRows(store.db, "pokemons")
	if err != nil {
		t.Fatal(err)
	}
	if pokemonCount != len(samplePokemon()) {
		t.Fatalf("pokemon count = %d, want %d", pokemonCount, len(samplePokemon()))
	}

	if got := len(store.MyTeams(demoUserID)); got != 1 {
		t.Fatalf("my teams = %d, want 1", got)
	}
	if got := len(store.GlobalTeams("recent")); got != 3 {
		t.Fatalf("global teams = %d, want 3", got)
	}
	if got := len(store.Tournaments()); got != 2 {
		t.Fatalf("tournaments = %d, want 2", got)
	}
}

func TestSQLiteStoreCreatesParentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app", "data", "futemon.db")
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if _, err := store.db.Exec("SELECT 1"); err != nil {
		t.Fatal(err)
	}
}

func TestSQLiteStoreSeedsDemoTeamsWhenPokemonAlreadyExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "futemon.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(migrationSQL); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO pokemons (
			id, name, artwork_url, local_artwork_url, type_1, type_2, hp, attack, defense,
			special_attack, special_defense, speed, description, abilities
		) VALUES (1, 'bulbasaur', '', '', 'grass', 'poison', 45, 49, 49, 65, 65, 45, '', '[]')`,
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if got := len(store.GlobalTeams("recent")); got != 3 {
		t.Fatalf("global teams = %d, want 3", got)
	}
	if got := len(store.TeamHistory("team-kanto-press")); got == 0 {
		t.Fatalf("expected demo match history for kanto")
	}
}

func TestSQLiteStorePokemonIncludeArtwork(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	pokemon := store.Pokemon()
	if len(pokemon) == 0 {
		t.Fatal("expected seeded Pokemon")
	}
	if pokemon[0].ArtworkURL == "" {
		t.Fatalf("pokemon missing external artwork: %+v", pokemon[0])
	}
}

func TestSQLiteStorePersistsLocalArtworkURL(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	pokemon := Pokemon{ID: 1, Name: "bulbasaur", ArtworkURL: "https://img.example/bulbasaur.png", LocalArtworkURL: "/static/pokemon-artwork/1.png", Type1: "grass"}
	if err := store.UpsertPokemon(pokemon); err != nil {
		t.Fatal(err)
	}
	got := store.Pokemon()[0]
	if got.LocalArtworkURL != "/static/pokemon-artwork/1.png" {
		t.Fatalf("local artwork url = %q", got.LocalArtworkURL)
	}
	if got.DisplayArtworkURL() != got.LocalArtworkURL {
		t.Fatalf("display artwork = %q, want local", got.DisplayArtworkURL())
	}
}

func TestSQLiteStoreUpsertsGoogleUser(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	created, err := store.UpsertGoogleUser(GoogleProfile{GoogleID: "google-123", DisplayName: "Ash Ketchum", Email: "ash@example.com", PictureURL: "https://img.example/ash.png"})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Role != "user" {
		t.Fatalf("created user = %+v", created)
	}
	if created.PictureURL != "https://img.example/ash.png" {
		t.Fatalf("picture url = %q", created.PictureURL)
	}

	updated, err := store.UpsertGoogleUser(GoogleProfile{GoogleID: "google-123", DisplayName: "Ash Campeao", Email: "ash@league.example", PictureURL: "https://img.example/champion.png"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != created.ID {
		t.Fatalf("updated id = %q, want %q", updated.ID, created.ID)
	}
	if updated.DisplayName != "Ash Campeao" || updated.Email != "ash@league.example" {
		t.Fatalf("updated user = %+v", updated)
	}
	if updated.PictureURL != "https://img.example/champion.png" {
		t.Fatalf("updated picture url = %q", updated.PictureURL)
	}
	if _, ok := store.UserByID(updated.ID); !ok {
		t.Fatal("upserted user was not found by session lookup")
	}
}

func TestSQLiteStorePersistsMatch(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	teamA, ok := store.FindTeam("team-kanto-press")
	if !ok {
		t.Fatal("team A not found")
	}
	teamB, ok := store.FindTeam("team-paleta-bolada")
	if !ok {
		t.Fatal("team B not found")
	}

	match := SimulateMatch(teamA, teamB)
	if err := store.SaveMatch(match); err != nil {
		t.Fatal(err)
	}

	got, ok := store.MatchByID(match.ID)
	if !ok {
		t.Fatal("match not found")
	}
	if got.ID != match.ID {
		t.Fatalf("match id = %q, want %q", got.ID, match.ID)
	}
	if got.TeamA.ID != teamA.ID || got.TeamB.ID != teamB.ID {
		t.Fatalf("teams = %q/%q, want %q/%q", got.TeamA.ID, got.TeamB.ID, teamA.ID, teamB.ID)
	}
	if got.ScoreTeamA != match.ScoreTeamA || got.ScoreTeamB != match.ScoreTeamB {
		t.Fatalf("score = %d-%d, want %d-%d", got.ScoreTeamA, got.ScoreTeamB, match.ScoreTeamA, match.ScoreTeamB)
	}
	if len(got.Events) != len(match.Events) {
		t.Fatalf("events = %d, want %d", len(got.Events), len(match.Events))
	}
	var eventCount int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM match_events WHERE match_id = ?", match.ID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != len(match.Events) {
		t.Fatalf("persisted events = %d, want %d", eventCount, len(match.Events))
	}
	goal := firstGoal(got.Events)
	if goal.TeamID == "" || goal.PokemonID == 0 {
		t.Fatalf("goal attribution was not restored: %+v", goal)
	}
	if got.StartTime.IsZero() {
		t.Fatal("start time was not restored")
	}
}

func TestSQLiteStoreTeamHistoryUsesCompletedMatchesGoalsAndSnapshots(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	teamA, _ := store.FindTeam("team-kanto-press")
	teamB, _ := store.FindTeam("team-paleta-bolada")
	match := completedTestMatch("match-history", teamA, teamB, []MatchEvent{
		{Minute: 3, Type: "goal", TeamID: teamA.ID, PokemonID: teamA.Pivo.ID, Narrative: "Machamp abre o placar."},
		{Minute: 9, Type: "goal", TeamID: teamB.ID, PokemonID: teamB.Pivo.ID, Narrative: "Dragonite empata."},
		{Minute: 18, Type: "goal", TeamID: teamA.ID, PokemonID: teamA.AlaEsquerda.ID, Narrative: "Pikachu decide."},
		{Minute: 40, Type: "fulltime", Narrative: "Fim."},
	})
	store.SaveMatch(match)

	_, err := store.SaveTeam(TeamInput{
		ID:                 teamA.ID,
		UserID:             teamA.UserID,
		Name:               "Kanto Editado",
		GoalkeeperID:       teamA.Goalkeeper.ID,
		GoalkeeperAbility:  teamA.GoalkeeperAbility,
		FixoID:             teamA.Fixo.ID,
		FixoAbility:        teamA.FixoAbility,
		AlaEsquerdaID:      teamA.AlaEsquerda.ID,
		AlaEsquerdaAbility: teamA.AlaEsquerdaAbility,
		AlaDireitaID:       teamA.AlaDireita.ID,
		AlaDireitaAbility:  teamA.AlaDireitaAbility,
		PivoID:             teamA.Pivo.ID,
		PivoAbility:        teamA.PivoAbility,
	})
	if err != nil {
		t.Fatal(err)
	}

	history := store.TeamHistory(teamA.ID)
	var summary MatchSummary
	for _, item := range history {
		if item.ID == match.ID {
			summary = item
			break
		}
	}
	if summary.ID == "" {
		t.Fatalf("history did not include %s: %+v", match.ID, history)
	}
	if summary.TeamAName != "Kanto Press" {
		t.Fatalf("history used edited team name = %q, want snapshot", summary.TeamAName)
	}
	if len(summary.GoalsTeamA) != 2 || len(summary.GoalsTeamB) != 1 {
		t.Fatalf("goal summary = A:%+v B:%+v", summary.GoalsTeamA, summary.GoalsTeamB)
	}
	if summary.GoalsTeamA[0].PokemonName == "" {
		t.Fatalf("goal scorer missing: %+v", summary.GoalsTeamA[0])
	}

	got, ok := store.MatchByID(match.ID)
	if !ok {
		t.Fatal("match not found")
	}
	if got.TeamA.Name != "Kanto Press" {
		t.Fatalf("match replay used edited team name = %q, want snapshot", got.TeamA.Name)
	}
}

func TestSQLiteStoreGlobalTeamsBestSortsByWeightedRecord(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	teamA, _ := store.FindTeam("team-kanto-press")
	teamB, _ := store.FindTeam("team-paleta-bolada")
	teamC, _ := store.FindTeam("team-ginasio-azul")
	store.SaveMatch(completedTestMatch("match-win-1", teamA, teamB, []MatchEvent{
		{Minute: 5, Type: "goal", TeamID: teamA.ID, PokemonID: teamA.Pivo.ID, Narrative: "Gol."},
		{Minute: 40, Type: "fulltime", Narrative: "Fim."},
	}))
	store.SaveMatch(completedTestMatch("match-win-2", teamA, teamC, []MatchEvent{
		{Minute: 8, Type: "goal", TeamID: teamA.ID, PokemonID: teamA.AlaEsquerda.ID, Narrative: "Gol."},
		{Minute: 12, Type: "goal", TeamID: teamA.ID, PokemonID: teamA.Pivo.ID, Narrative: "Gol."},
		{Minute: 40, Type: "fulltime", Narrative: "Fim."},
	}))

	best := store.GlobalTeams("best")
	if len(best) == 0 || best[0].ID != teamA.ID {
		t.Fatalf("best teams first = %+v, want %s", best, teamA.ID)
	}
	if best[0].Record.Wins < 2 || best[0].Record.Played < 2 {
		t.Fatalf("record = %+v, want at least 2 wins in seeded/demo games", best[0].Record)
	}
}

func TestSQLiteStoreCreatesUpdatesAndDeletesTeam(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	created, err := store.SaveTeam(validAbilityInput(TeamInput{
		UserID:            demoUserID,
		Name:              "Teste de Quadra",
		GoalkeeperID:      9,
		GoalkeeperAbility: "rain-dish",
		FixoID:            6,
		AlaEsquerdaID:     25,
		AlaDireitaID:      4,
		PivoID:            68,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" {
		t.Fatal("created team has empty id")
	}
	if created.Name != "Teste de Quadra" {
		t.Fatalf("created team name = %q", created.Name)
	}
	if created.GoalkeeperAbility != "rain-dish" || created.FixoAbility == "" {
		t.Fatalf("created abilities were not normalized: %+v", created.Roster())
	}

	updated, err := store.SaveTeam(TeamInput{
		ID:                 created.ID,
		UserID:             demoUserID,
		Name:               "Teste Editado",
		GoalkeeperID:       143,
		GoalkeeperAbility:  "thick-fat",
		FixoID:             created.Fixo.ID,
		FixoAbility:        created.FixoAbility,
		AlaEsquerdaID:      created.AlaEsquerda.ID,
		AlaEsquerdaAbility: "static",
		AlaDireitaID:       created.AlaDireita.ID,
		AlaDireitaAbility:  created.AlaDireitaAbility,
		PivoID:             created.Pivo.ID,
		PivoAbility:        created.PivoAbility,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Teste Editado" || updated.Goalkeeper.ID != 143 || updated.GoalkeeperAbility != "thick-fat" || updated.FixoAbility == "" {
		t.Fatalf("updated team = %+v", updated)
	}

	if err := store.DeleteTeam(created.ID, demoUserID); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.FindTeam(created.ID); ok {
		t.Fatal("deleted team still found")
	}
}

func TestSQLiteStoreRetiredTeamsStayHistoricalButLeaveCompetitiveLists(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	created, err := store.SaveTeam(validAbilityInput(TeamInput{
		UserID:        demoUserID,
		Name:          "Time Aposentavel",
		GoalkeeperID:  9,
		FixoID:        6,
		AlaEsquerdaID: 25,
		AlaDireitaID:  4,
		PivoID:        68,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteTeam(created.ID, demoUserID); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.FindTeam(created.ID); ok {
		t.Fatal("retired team should not be found as an active team")
	}
	retired, ok := store.FindTeamIncludingRetired(created.ID)
	if !ok || !retired.IsRetired {
		t.Fatalf("retired lookup = %+v, ok %v", retired, ok)
	}
	for _, team := range store.GlobalTeams("recent") {
		if team.ID == created.ID {
			t.Fatal("retired team appeared in global teams")
		}
	}
	for _, team := range store.RetiredTeams(demoUserID) {
		if team.ID == created.ID {
			return
		}
	}
	t.Fatal("retired team did not appear in retired team history")
}

func TestSQLiteStoreRecordsOriginalFormationAndLimitsWeeklyPokemonTransfer(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	team, ok := store.FindTeam("team-kanto-press")
	if !ok {
		t.Fatal("team not found")
	}
	transfers := store.TeamTransfers(team.ID)
	if len(transfers) == 0 || transfers[0].Kind != "formation_created" {
		t.Fatalf("initial transfers = %+v, want original formation", transfers)
	}
	if store.TransferWindow(team.ID).Used {
		t.Fatal("fresh transfer window should not be used")
	}

	_, err := store.SaveTeam(TeamInput{
		ID:                 team.ID,
		UserID:             team.UserID,
		Name:               team.Name,
		GoalkeeperID:       team.Goalkeeper.ID,
		GoalkeeperAbility:  team.GoalkeeperAbility,
		FixoID:             95,
		FixoAbility:        "sturdy",
		AlaEsquerdaID:      team.AlaEsquerda.ID,
		AlaEsquerdaAbility: team.AlaEsquerdaAbility,
		AlaDireitaID:       team.AlaDireita.ID,
		AlaDireitaAbility:  team.AlaDireitaAbility,
		PivoID:             149,
		PivoAbility:        "inner-focus",
	})
	if !errors.Is(err, ErrTransferTooLarge) {
		t.Fatalf("multi Pokemon transfer err = %v, want ErrTransferTooLarge", err)
	}
	if store.TransferWindow(team.ID).Used {
		t.Fatal("rejected transfer should not consume transfer window")
	}

	updated, err := store.SaveTeam(TeamInput{
		ID:                 team.ID,
		UserID:             team.UserID,
		Name:               team.Name,
		GoalkeeperID:       team.Goalkeeper.ID,
		GoalkeeperAbility:  team.GoalkeeperAbility,
		FixoID:             team.Fixo.ID,
		FixoAbility:        team.FixoAbility,
		AlaEsquerdaID:      team.AlaEsquerda.ID,
		AlaEsquerdaAbility: team.AlaEsquerdaAbility,
		AlaDireitaID:       team.AlaDireita.ID,
		AlaDireitaAbility:  team.AlaDireitaAbility,
		PivoID:             149,
		PivoAbility:        "inner-focus",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !store.TransferWindow(team.ID).Used {
		t.Fatal("transfer window should be used after Pokemon swap")
	}
	transfers = store.TeamTransfers(team.ID)
	last := transfers[len(transfers)-1]
	if last.Kind != "pokemon_transfer" || len(last.ChangedPlayers) != 1 || last.ChangedPlayers[0].Position != "Pivo" {
		t.Fatalf("transfer history = %+v", last)
	}

	_, err = store.SaveTeam(TeamInput{
		ID:                 updated.ID,
		UserID:             updated.UserID,
		Name:               updated.Name,
		GoalkeeperID:       updated.Goalkeeper.ID,
		GoalkeeperAbility:  updated.GoalkeeperAbility,
		FixoID:             95,
		FixoAbility:        "sturdy",
		AlaEsquerdaID:      updated.AlaEsquerda.ID,
		AlaEsquerdaAbility: updated.AlaEsquerdaAbility,
		AlaDireitaID:       updated.AlaDireita.ID,
		AlaDireitaAbility:  updated.AlaDireitaAbility,
		PivoID:             updated.Pivo.ID,
		PivoAbility:        updated.PivoAbility,
	})
	if !errors.Is(err, ErrTransferLimit) {
		t.Fatalf("second Pokemon transfer err = %v, want ErrTransferLimit", err)
	}

	renamed, err := store.SaveTeam(TeamInput{
		ID:                 updated.ID,
		UserID:             updated.UserID,
		Name:               "Kanto Nome Novo",
		GoalkeeperID:       updated.Goalkeeper.ID,
		GoalkeeperAbility:  updated.GoalkeeperAbility,
		FixoID:             updated.Fixo.ID,
		FixoAbility:        updated.FixoAbility,
		AlaEsquerdaID:      updated.AlaEsquerda.ID,
		AlaEsquerdaAbility: updated.AlaEsquerdaAbility,
		AlaDireitaID:       updated.AlaDireita.ID,
		AlaDireitaAbility:  updated.AlaDireitaAbility,
		PivoID:             updated.Pivo.ID,
		PivoAbility:        updated.PivoAbility,
	})
	if err != nil {
		t.Fatalf("non-Pokemon edit should be allowed after transfer: %v", err)
	}
	if renamed.Name != "Kanto Nome Novo" {
		t.Fatalf("renamed team = %q", renamed.Name)
	}
}

func TestSQLiteStoreRejectsDuplicatePokemonAndInvalidAbility(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	_, err := store.SaveTeam(TeamInput{
		UserID:            demoUserID,
		Name:              "Time Repetido",
		GoalkeeperID:      9,
		GoalkeeperAbility: "torrent",
		FixoID:            9,
		AlaEsquerdaID:     25,
		AlaDireitaID:      4,
		PivoID:            68,
	})
	if !errors.Is(err, ErrDuplicatePokemon) {
		t.Fatalf("duplicate err = %v, want ErrDuplicatePokemon", err)
	}

	_, err = store.SaveTeam(TeamInput{
		UserID:             demoUserID,
		Name:               "Habilidade Invalida",
		GoalkeeperID:       9,
		GoalkeeperAbility:  "levitate",
		FixoID:             6,
		FixoAbility:        "blaze",
		AlaEsquerdaID:      25,
		AlaEsquerdaAbility: "static",
		AlaDireitaID:       4,
		AlaDireitaAbility:  "blaze",
		PivoID:             68,
		PivoAbility:        "no-guard",
	})
	if !errors.Is(err, ErrInvalidAbility) {
		t.Fatalf("ability err = %v, want ErrInvalidAbility", err)
	}
}

func TestSQLiteStoreTeamLimit(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	for i := 0; i < 5; i++ {
		_, err := store.SaveTeam(validAbilityInput(TeamInput{
			UserID:        demoUserID,
			Name:          "Time Extra",
			GoalkeeperID:  9,
			FixoID:        6,
			AlaEsquerdaID: 25,
			AlaDireitaID:  4,
			PivoID:        68,
		}))
		if err != nil {
			t.Fatal(err)
		}
	}

	_, err := store.SaveTeam(validAbilityInput(TeamInput{
		UserID:        demoUserID,
		Name:          "Time Sete",
		GoalkeeperID:  9,
		FixoID:        6,
		AlaEsquerdaID: 25,
		AlaDireitaID:  4,
		PivoID:        68,
	}))
	if !errors.Is(err, ErrTeamLimitReached) {
		t.Fatalf("err = %v, want ErrTeamLimitReached", err)
	}
}

func TestSQLiteStoreRejectsFrozenTeamMutations(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	_, err := store.SaveTeam(validAbilityInput(TeamInput{
		ID:            "team-paleta-bolada",
		UserID:        "user-rival",
		Name:          "Tentativa Congelada",
		GoalkeeperID:  9,
		FixoID:        6,
		AlaEsquerdaID: 25,
		AlaDireitaID:  4,
		PivoID:        68,
	}))
	if !errors.Is(err, ErrTeamFrozen) {
		t.Fatalf("update err = %v, want ErrTeamFrozen", err)
	}

	err = store.DeleteTeam("team-paleta-bolada", "user-rival")
	if !errors.Is(err, ErrTeamFrozen) {
		t.Fatalf("delete err = %v, want ErrTeamFrozen", err)
	}
}

func TestSQLiteStoreUpdatesAccountAndEncryptsAPIKey(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()
	cipher, err := NewKeyCipher([]byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatal(err)
	}
	store.WithCipher(cipher)

	user, err := store.UpdateAccount(AccountInput{
		UserID:           demoUserID,
		DisplayName:      "Treinador Atualizado",
		AvatarIcon:       12,
		OpenRouterAPIKey: "openrouter-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.DisplayName != "Treinador Atualizado" {
		t.Fatalf("display name = %q", user.DisplayName)
	}
	if user.AvatarIcon != 12 {
		t.Fatalf("avatar icon = %d", user.AvatarIcon)
	}
	if !user.HasOpenRouterAPIKey {
		t.Fatal("expected OpenRouter key flag")
	}
	if user.OpenRouterAPIKey == "openrouter-secret" {
		t.Fatal("API key was stored in plaintext")
	}
	decrypted, err := cipher.Decrypt(user.OpenRouterAPIKey)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "openrouter-secret" {
		t.Fatalf("decrypted key = %q", decrypted)
	}
	apiKey, ok, err := store.UserAPIKey(demoUserID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || apiKey != "openrouter-secret" {
		t.Fatalf("user api key = %q, ok %v", apiKey, ok)
	}
}

func TestSQLiteStoreRejectsAPIKeyWithoutCipher(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	_, err := store.UpdateAccount(AccountInput{
		UserID:           demoUserID,
		DisplayName:      "Treinador Demo",
		OpenRouterAPIKey: "openrouter-secret",
	})
	if !errors.Is(err, ErrEncryptionKeyRequired) {
		t.Fatalf("err = %v, want ErrEncryptionKeyRequired", err)
	}
}

func TestSQLiteStorePersistsDailyDuelUsage(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	date := "2026-06-19"
	count, err := store.DailyDuelCount(demoUserID, date)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("initial duel usage = %d, want 0", count)
	}

	if err := store.RecordDailyDuel(demoUserID, date); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordDailyDuel(demoUserID, date); err != nil {
		t.Fatal(err)
	}
	count, err = store.DailyDuelCount(demoUserID, date)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("duel usage = %d, want 2", count)
	}
	if err := store.RefundDailyDuel(demoUserID, date); err != nil {
		t.Fatal(err)
	}
	count, err = store.DailyDuelCount(demoUserID, date)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("duel usage after refund = %d, want 1", count)
	}
	if err := store.RefundDailyDuel(demoUserID, date); err != nil {
		t.Fatal(err)
	}
	if err := store.RefundDailyDuel(demoUserID, date); err != nil {
		t.Fatal(err)
	}
	count, err = store.DailyDuelCount(demoUserID, date)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("duel usage after extra refund = %d, want 0", count)
	}
}

func validAbilityInput(input TeamInput) TeamInput {
	if input.GoalkeeperAbility == "" {
		input.GoalkeeperAbility = "torrent"
	}
	if input.FixoAbility == "" {
		input.FixoAbility = "blaze"
	}
	if input.AlaEsquerdaAbility == "" {
		input.AlaEsquerdaAbility = "static"
	}
	if input.AlaDireitaAbility == "" {
		input.AlaDireitaAbility = "blaze"
	}
	if input.PivoAbility == "" {
		input.PivoAbility = "no-guard"
	}
	return input
}

func newTestSQLiteStore(t *testing.T) *SQLiteStore {
	t.Helper()

	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "futemon.db"))
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func firstGoal(events []MatchEvent) MatchEvent {
	for _, event := range events {
		if event.Type == "goal" {
			return event
		}
	}
	return MatchEvent{}
}

func completedTestMatch(id string, teamA Team, teamB Team, events []MatchEvent) MatchResult {
	start := time.Now().Add(-2 * time.Hour)
	match := MatchResult{
		ID:        id,
		TeamA:     teamA,
		TeamB:     teamB,
		Events:    events,
		StartTime: start,
		EndTime:   start.Add(time.Hour),
	}
	match.ScoreTeamA, match.ScoreTeamB = match.Score()
	return match
}
