package app

import (
	"errors"
	"path/filepath"
	"testing"
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

	if got := len(store.MyTeams()); got != 1 {
		t.Fatalf("my teams = %d, want 1", got)
	}
	if got := len(store.GlobalTeams()); got != 3 {
		t.Fatalf("global teams = %d, want 3", got)
	}
	if got := len(store.Tournaments()); got != 2 {
		t.Fatalf("tournaments = %d, want 2", got)
	}
}

func TestSQLiteStorePersistsLatestMatch(t *testing.T) {
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
	store.SetLatestMatch(match)

	got, ok := store.LatestMatch()
	if !ok {
		t.Fatal("latest match not found")
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
	eventCount, err := countRows(store.db, "match_events")
	if err != nil {
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

func TestSQLiteStoreCreatesUpdatesAndDeletesTeam(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	created, err := store.SaveTeam(TeamInput{
		UserID:        demoUserID,
		Name:          "Teste de Quadra",
		GoalkeeperID:  9,
		FixoID:        6,
		AlaEsquerdaID: 25,
		AlaDireitaID:  4,
		PivoID:        68,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" {
		t.Fatal("created team has empty id")
	}
	if created.Name != "Teste de Quadra" {
		t.Fatalf("created team name = %q", created.Name)
	}

	updated, err := store.SaveTeam(TeamInput{
		ID:            created.ID,
		UserID:        demoUserID,
		Name:          "Teste Editado",
		GoalkeeperID:  143,
		FixoID:        95,
		AlaEsquerdaID: 26,
		AlaDireitaID:  7,
		PivoID:        149,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Teste Editado" || updated.Goalkeeper.ID != 143 {
		t.Fatalf("updated team = %+v", updated)
	}

	if err := store.DeleteTeam(created.ID, demoUserID); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.FindTeam(created.ID); ok {
		t.Fatal("deleted team still found")
	}
}

func TestSQLiteStoreTeamLimit(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	for i := 0; i < 5; i++ {
		_, err := store.SaveTeam(TeamInput{
			UserID:        demoUserID,
			Name:          "Time Extra",
			GoalkeeperID:  9,
			FixoID:        6,
			AlaEsquerdaID: 25,
			AlaDireitaID:  4,
			PivoID:        68,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	_, err := store.SaveTeam(TeamInput{
		UserID:        demoUserID,
		Name:          "Time Sete",
		GoalkeeperID:  9,
		FixoID:        6,
		AlaEsquerdaID: 25,
		AlaDireitaID:  4,
		PivoID:        68,
	})
	if !errors.Is(err, ErrTeamLimitReached) {
		t.Fatalf("err = %v, want ErrTeamLimitReached", err)
	}
}

func TestSQLiteStoreRejectsFrozenTeamMutations(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	_, err := store.SaveTeam(TeamInput{
		ID:            "team-paleta-bolada",
		UserID:        "user-rival",
		Name:          "Tentativa Congelada",
		GoalkeeperID:  9,
		FixoID:        6,
		AlaEsquerdaID: 25,
		AlaDireitaID:  4,
		PivoID:        68,
	})
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
		UserID:       demoUserID,
		DisplayName:  "Treinador Atualizado",
		GeminiAPIKey: "gemini-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.DisplayName != "Treinador Atualizado" {
		t.Fatalf("display name = %q", user.DisplayName)
	}
	if !user.HasGeminiAPIKey {
		t.Fatal("expected Gemini key flag")
	}
	if user.GeminiAPIKey == "gemini-secret" {
		t.Fatal("API key was stored in plaintext")
	}
	decrypted, err := cipher.Decrypt(user.GeminiAPIKey)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "gemini-secret" {
		t.Fatalf("decrypted key = %q", decrypted)
	}
}

func TestSQLiteStoreRejectsAPIKeyWithoutCipher(t *testing.T) {
	store := newTestSQLiteStore(t)
	defer store.Close()

	_, err := store.UpdateAccount(AccountInput{
		UserID:       demoUserID,
		DisplayName:  "Treinador Demo",
		GeminiAPIKey: "gemini-secret",
	})
	if !errors.Is(err, ErrEncryptionKeyRequired) {
		t.Fatalf("err = %v, want ErrEncryptionKeyRequired", err)
	}
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
