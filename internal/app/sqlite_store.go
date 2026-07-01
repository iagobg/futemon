package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const migrationSQL = `
CREATE TABLE IF NOT EXISTS pokemons (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  artwork_url TEXT NOT NULL DEFAULT '',
  local_artwork_url TEXT NOT NULL DEFAULT '',
  type_1 TEXT NOT NULL,
  type_2 TEXT,
  hp INTEGER NOT NULL,
  attack INTEGER NOT NULL,
  defense INTEGER NOT NULL,
  special_attack INTEGER NOT NULL,
  special_defense INTEGER NOT NULL,
  speed INTEGER NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  abilities TEXT NOT NULL DEFAULT '[]'
);

CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  google_id TEXT UNIQUE NOT NULL,
  display_name TEXT NOT NULL,
  email TEXT NOT NULL,
  picture_url TEXT NOT NULL DEFAULT '',
  avatar_icon INTEGER NOT NULL DEFAULT 0,
  openrouter_api_key TEXT,
  role TEXT NOT NULL DEFAULT 'user',
  deleted_at TEXT
);

CREATE TABLE IF NOT EXISTS teams (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id),
  name TEXT NOT NULL,
  goalkeeper_id INTEGER NOT NULL REFERENCES pokemons(id),
  goalkeeper_ability TEXT NOT NULL DEFAULT '',
  fixo_id INTEGER NOT NULL REFERENCES pokemons(id),
  fixo_ability TEXT NOT NULL DEFAULT '',
  ala_esquerda_id INTEGER NOT NULL REFERENCES pokemons(id),
  ala_esquerda_ability TEXT NOT NULL DEFAULT '',
  ala_direita_id INTEGER NOT NULL REFERENCES pokemons(id),
  ala_direita_ability TEXT NOT NULL DEFAULT '',
  pivo_id INTEGER NOT NULL REFERENCES pokemons(id),
  pivo_ability TEXT NOT NULL DEFAULT '',
  is_frozen INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  deleted_at TEXT
);

CREATE TABLE IF NOT EXISTS matches (
  id TEXT PRIMARY KEY,
  mode TEXT NOT NULL,
  team_a_id TEXT NOT NULL REFERENCES teams(id),
  team_b_id TEXT NOT NULL REFERENCES teams(id),
  team_a_snapshot TEXT NOT NULL DEFAULT '{}',
  team_b_snapshot TEXT NOT NULL DEFAULT '{}',
  score_a INTEGER,
  score_b INTEGER,
  raw_json_output TEXT NOT NULL DEFAULT '{}',
  start_time TEXT,
  ended_at TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS match_events (
  id TEXT PRIMARY KEY,
  match_id TEXT NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
  sequence INTEGER NOT NULL,
  minute INTEGER NOT NULL,
  type TEXT NOT NULL,
  narrative TEXT NOT NULL,
  dramatic_pause_seconds INTEGER NOT NULL DEFAULT 0,
  team_id TEXT REFERENCES teams(id),
  pokemon_id INTEGER REFERENCES pokemons(id),
  UNIQUE(match_id, sequence)
);

CREATE INDEX IF NOT EXISTS idx_match_events_match_sequence
ON match_events(match_id, sequence);

CREATE TABLE IF NOT EXISTS tournaments (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  status TEXT NOT NULL,
  created_by TEXT NOT NULL REFERENCES users(id),
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tournament_registrations (
  id TEXT PRIMARY KEY,
  tournament_id TEXT NOT NULL REFERENCES tournaments(id),
  team_id TEXT NOT NULL REFERENCES teams(id),
  consequences_log TEXT NOT NULL DEFAULT '',
  UNIQUE(tournament_id, team_id)
);

CREATE TABLE IF NOT EXISTS team_transactions (
  id TEXT PRIMARY KEY,
  team_id TEXT NOT NULL REFERENCES teams(id),
  kind TEXT NOT NULL,
  before_snapshot TEXT NOT NULL DEFAULT '{}',
  after_snapshot TEXT NOT NULL DEFAULT '{}',
  summary TEXT NOT NULL DEFAULT '',
  window_start TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS duel_usage (
  user_id TEXT NOT NULL REFERENCES users(id),
  usage_date TEXT NOT NULL,
  count INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL,
  PRIMARY KEY(user_id, usage_date)
);`

type SQLiteStore struct {
	db     *sql.DB
	cipher *KeyCipher
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if err := ensureSQLiteDir(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.seedDemoData(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.ensureInitialTeamTransactions(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.seedDemoMatches(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func ensureSQLiteDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func (s *SQLiteStore) WithCipher(cipher KeyCipher) *SQLiteStore {
	s.cipher = &cipher
	return s
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) migrate() error {
	if _, err := s.db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return err
	}
	if _, err := s.db.Exec(migrationSQL); err != nil {
		return err
	}
	if err := s.ensureColumn("teams", "deleted_at", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn("users", "picture_url", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn("users", "avatar_icon", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn("users", "openrouter_api_key", "TEXT"); err != nil {
		return err
	}
	for _, column := range []string{"goalkeeper_ability", "fixo_ability", "ala_esquerda_ability", "ala_direita_ability", "pivo_ability"} {
		if err := s.ensureColumn("teams", column, "TEXT NOT NULL DEFAULT ''"); err != nil {
			return err
		}
	}
	if err := s.ensureColumn("pokemons", "artwork_url", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn("pokemons", "local_artwork_url", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn("matches", "ended_at", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn("matches", "team_a_snapshot", "TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}
	if err := s.ensureColumn("matches", "team_b_snapshot", "TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}
	if err := s.backfillPokemonArtworkURLs(); err != nil {
		return err
	}
	return s.ensureInitialTeamTransactions()
}

func (s *SQLiteStore) seedDemoData() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, pokemon := range samplePokemon() {
		if _, err := tx.Exec(`
			INSERT OR IGNORE INTO pokemons (
				id, name, artwork_url, local_artwork_url, type_1, type_2, hp, attack, defense, special_attack,
				special_defense, speed, description, abilities
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			pokemon.ID, pokemon.Name, pokemon.ArtworkURL, pokemon.LocalArtworkURL, pokemon.Type1, nullString(pokemon.Type2), pokemon.HP,
			pokemon.Attack, pokemon.Defense, pokemon.SpecialAttack, pokemon.SpecialDefense,
			pokemon.Speed, pokemon.Description, pokemon.Abilities,
		); err != nil {
			return err
		}
	}

	now := time.Now().UTC()
	if _, err := tx.Exec(`
		INSERT OR IGNORE INTO users (id, google_id, display_name, email, avatar_icon, role)
		VALUES
			('user-demo', 'demo-google-id', 'Treinador Demo', 'demo@futemon.local', 1, 'admin'),
			('user-rival', 'rival-google-id', 'Rival Local', 'rival@futemon.local', 2, 'user'),
			('user-misty', 'misty-google-id', 'Lider do Ginasio Azul', 'misty@futemon.local', 3, 'user')`); err != nil {
		return err
	}

	teams := []struct {
		ID, UserID, Name   string
		Goalkeeper         int
		GoalkeeperAbility  string
		Fixo               int
		FixoAbility        string
		AlaEsquerda        int
		AlaEsquerdaAbility string
		AlaDireita         int
		AlaDireitaAbility  string
		Pivo               int
		PivoAbility        string
		IsFrozen           bool
		CreatedAt          time.Time
	}{
		{"team-kanto-press", "user-demo", "Kanto Press", 9, "torrent", 6, "blaze", 25, "static", 4, "blaze", 68, "no-guard", false, now.Add(-72 * time.Hour)},
		{"team-paleta-bolada", "user-rival", "Paleta Bolada", 143, "thick-fat", 95, "sturdy", 26, "static", 7, "torrent", 149, "inner-focus", true, now.Add(-48 * time.Hour)},
		{"team-ginasio-azul", "user-misty", "Ginasio Azul FC", 130, "intimidate", 55, "cloud-nine", 121, "natural-cure", 7, "torrent", 131, "water-absorb", false, now.Add(-24 * time.Hour)},
	}
	for _, team := range teams {
		if _, err := tx.Exec(`
			INSERT OR IGNORE INTO teams (
				id, user_id, name, goalkeeper_id, goalkeeper_ability, fixo_id, fixo_ability,
				ala_esquerda_id, ala_esquerda_ability, ala_direita_id, ala_direita_ability,
				pivo_id, pivo_ability, is_frozen, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			team.ID, team.UserID, team.Name, team.Goalkeeper, team.GoalkeeperAbility, team.Fixo, team.FixoAbility,
			team.AlaEsquerda, team.AlaEsquerdaAbility, team.AlaDireita, team.AlaDireitaAbility,
			team.Pivo, team.PivoAbility, boolInt(team.IsFrozen), formatTime(team.CreatedAt),
		); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(`
		INSERT OR IGNORE INTO tournaments (id, name, status, created_by, created_at)
		VALUES
			('tourn-001', 'Copa Professor Carvalho', 'registration', 'user-demo', ?),
			('tourn-002', 'Liga dos Centros Pokemon', 'active', 'user-demo', ?)`,
		formatTime(now.Add(-12*time.Hour)), formatTime(now.Add(-6*time.Hour)),
	); err != nil {
		return err
	}

	registrations := []struct {
		ID, TournamentID, TeamID string
	}{
		{"reg-001", "tourn-001", "team-kanto-press"},
		{"reg-002", "tourn-001", "team-ginasio-azul"},
		{"reg-003", "tourn-002", "team-paleta-bolada"},
		{"reg-004", "tourn-002", "team-ginasio-azul"},
	}
	for _, registration := range registrations {
		if _, err := tx.Exec(`
			INSERT OR IGNORE INTO tournament_registrations (id, tournament_id, team_id)
			VALUES (?, ?, ?)`,
			registration.ID, registration.TournamentID, registration.TeamID,
		); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) seedDemoMatches() error {
	count, err := countRows(s.db, "matches")
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	kanto, okA := s.FindTeam("team-kanto-press")
	paleta, okB := s.FindTeam("team-paleta-bolada")
	ginasio, okC := s.FindTeam("team-ginasio-azul")
	if !okA || !okB || !okC {
		return nil
	}
	now := time.Now().Add(-5 * 24 * time.Hour)
	matches := []MatchResult{
		demoMatch("match-demo-001", kanto, paleta, now, []demoGoal{{3, "a", kanto.AlaEsquerda.ID}, {14, "b", paleta.Pivo.ID}, {31, "a", kanto.Pivo.ID}}),
		demoMatch("match-demo-002", ginasio, kanto, now.Add(18*time.Hour), []demoGoal{{9, "a", ginasio.Pivo.ID}, {19, "a", ginasio.AlaDireita.ID}, {33, "b", kanto.AlaDireita.ID}}),
		demoMatch("match-demo-003", paleta, ginasio, now.Add(36*time.Hour), []demoGoal{{8, "a", paleta.Pivo.ID}, {16, "b", ginasio.Pivo.ID}, {27, "a", paleta.AlaEsquerda.ID}}),
		demoMatch("match-demo-004", kanto, ginasio, now.Add(54*time.Hour), []demoGoal{{5, "a", kanto.Pivo.ID}, {22, "b", ginasio.Pivo.ID}, {38, "b", ginasio.AlaEsquerda.ID}}),
		demoMatch("match-demo-005", paleta, kanto, now.Add(72*time.Hour), []demoGoal{{12, "b", kanto.AlaEsquerda.ID}}),
	}
	for _, match := range matches {
		if err := s.SaveMatch(match); err != nil {
			return err
		}
	}
	return nil
}

type demoGoal struct {
	minute    int
	side      string
	pokemonID int
}

func demoMatch(id string, teamA Team, teamB Team, start time.Time, goals []demoGoal) MatchResult {
	events := []MatchEvent{
		{Minute: 0, Type: "kickoff", Narrative: fmt.Sprintf("%s e %s entram em quadra para uma partida de teste.", teamA.Name, teamB.Name)},
	}
	for _, goal := range goals {
		team := teamA
		teamID := teamA.ID
		if goal.side == "b" {
			team = teamB
			teamID = teamB.ID
		}
		events = append(events, MatchEvent{
			Minute:    goal.minute,
			Type:      "goal",
			TeamID:    teamID,
			PokemonID: goal.pokemonID,
			Narrative: fmt.Sprintf("%s encontra espaco e marca para %s.", pokemonNameForEvent(MatchResult{TeamA: teamA, TeamB: teamB}, goal.pokemonID), team.Name),
		})
	}
	events = append(events, MatchEvent{Minute: 40, Type: "fulltime", Narrative: "Fim de jogo no historico de demonstracao."})
	match := MatchResult{ID: id, TeamA: teamA, TeamB: teamB, Events: events, StartTime: start, EndTime: start.Add(time.Hour)}
	match.ScoreTeamA, match.ScoreTeamB = match.Score()
	return match
}

func (s *SQLiteStore) Pokemon() []Pokemon {
	rows, err := s.db.Query(`
		SELECT id, name, COALESCE(artwork_url, ''), COALESCE(local_artwork_url, ''), type_1, COALESCE(type_2, ''), hp, attack, defense,
			special_attack, special_defense, speed, description, abilities
		FROM pokemons
		ORDER BY id`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var pokemon []Pokemon
	for rows.Next() {
		item, err := scanPokemon(rows)
		if err != nil {
			return nil
		}
		pokemon = append(pokemon, item)
	}
	return pokemon
}

func (s *SQLiteStore) CurrentUser() (User, bool) {
	return s.findUser(demoUserID)
}

func (s *SQLiteStore) UserByID(id string) (User, bool) {
	return s.findUser(id)
}

func (s *SQLiteStore) UserAPIKey(userID string) (string, bool, error) {
	user, ok := s.findUser(userID)
	if !ok {
		return "", false, ErrUserNotFound
	}
	if strings.TrimSpace(user.OpenRouterAPIKey) == "" {
		return "", false, nil
	}
	if s.cipher == nil {
		return "", false, ErrEncryptionKeyRequired
	}
	apiKey, err := s.cipher.Decrypt(user.OpenRouterAPIKey)
	if err != nil {
		return "", false, err
	}
	return apiKey, strings.TrimSpace(apiKey) != "", nil
}

func (s *SQLiteStore) DailyDuelCount(userID string, date string) (int, error) {
	var count int
	err := s.db.QueryRow(
		"SELECT count FROM duel_usage WHERE user_id = ? AND usage_date = ?",
		userID, date,
	).Scan(&count)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return count, err
}

func (s *SQLiteStore) RecordDailyDuel(userID string, date string) error {
	_, err := s.db.Exec(`
		INSERT INTO duel_usage (user_id, usage_date, count, updated_at)
		VALUES (?, ?, 1, ?)
		ON CONFLICT(user_id, usage_date) DO UPDATE SET
			count = count + 1,
			updated_at = excluded.updated_at`,
		userID, date, formatTime(time.Now().UTC()),
	)
	return err
}

func (s *SQLiteStore) RefundDailyDuel(userID string, date string) error {
	_, err := s.db.Exec(`
		UPDATE duel_usage
		SET count = MAX(count - 1, 0),
			updated_at = ?
		WHERE user_id = ? AND usage_date = ?`,
		formatTime(time.Now().UTC()), userID, date,
	)
	return err
}

func (s *SQLiteStore) UpsertGoogleUser(profile GoogleProfile) (User, error) {
	profile.DisplayName = strings.TrimSpace(profile.DisplayName)
	profile.Email = strings.TrimSpace(profile.Email)
	if profile.GoogleID == "" || profile.Email == "" {
		return User{}, ErrInvalidAccount
	}
	if profile.DisplayName == "" {
		profile.DisplayName = profile.Email
	}

	var id string
	err := s.db.QueryRow("SELECT id FROM users WHERE google_id = ? AND deleted_at IS NULL", profile.GoogleID).Scan(&id)
	switch {
	case err == nil:
		if _, err := s.db.Exec(
			"UPDATE users SET display_name = ?, email = ?, picture_url = ? WHERE id = ?",
			profile.DisplayName, profile.Email, profile.PictureURL, id,
		); err != nil {
			return User{}, err
		}
	case errors.Is(err, sql.ErrNoRows):
		id = newID("user")
		if _, err := s.db.Exec(
			"INSERT INTO users (id, google_id, display_name, email, picture_url, role) VALUES (?, ?, ?, ?, ?, 'user')",
			id, profile.GoogleID, profile.DisplayName, profile.Email, profile.PictureURL,
		); err != nil {
			return User{}, err
		}
	default:
		return User{}, err
	}

	user, ok := s.findUser(id)
	if !ok {
		return User{}, ErrUserNotFound
	}
	return user, nil
}

func (s *SQLiteStore) UpdateAccount(input AccountInput) (User, error) {
	if input.UserID == "" {
		input.UserID = demoUserID
	}
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if input.DisplayName == "" {
		return User{}, ErrInvalidAccount
	}

	user, ok := s.findUser(input.UserID)
	if !ok {
		return User{}, ErrUserNotFound
	}

	apiKey := user.OpenRouterAPIKey
	if input.ClearAPIKey {
		apiKey = ""
	} else if strings.TrimSpace(input.OpenRouterAPIKey) != "" {
		if s.cipher == nil {
			return User{}, ErrEncryptionKeyRequired
		}
		encrypted, err := s.cipher.Encrypt(strings.TrimSpace(input.OpenRouterAPIKey))
		if err != nil {
			return User{}, err
		}
		apiKey = encrypted
	}

	if _, err := s.db.Exec(
		"UPDATE users SET display_name = ?, avatar_icon = ?, openrouter_api_key = ? WHERE id = ?",
		input.DisplayName, normalizeAvatarIcon(input.AvatarIcon), nullString(apiKey), input.UserID,
	); err != nil {
		return User{}, err
	}
	updated, ok := s.findUser(input.UserID)
	if !ok {
		return User{}, ErrUserNotFound
	}
	return updated, nil
}

func (s *SQLiteStore) findUser(id string) (User, bool) {
	var user User
	err := s.db.QueryRow(`
		SELECT id, google_id, display_name, email, COALESCE(picture_url, ''), COALESCE(avatar_icon, 0), COALESCE(openrouter_api_key, ''), role
		FROM users
		WHERE id = ? AND deleted_at IS NULL`, id).Scan(
		&user.ID, &user.GoogleID, &user.DisplayName, &user.Email, &user.PictureURL, &user.AvatarIcon, &user.OpenRouterAPIKey, &user.Role,
	)
	if err != nil {
		return User{}, false
	}
	user.HasOpenRouterAPIKey = user.OpenRouterAPIKey != ""
	return user, true
}

func (s *SQLiteStore) MyTeams(userID string) []Team {
	if userID == "" {
		return nil
	}
	return s.teamsWhere("t.user_id = ?", userID)
}

func (s *SQLiteStore) RetiredTeams(userID string) []Team {
	if userID == "" {
		return nil
	}
	return s.teamsWhereWithDeleted(true, "t.user_id = ? AND t.deleted_at IS NOT NULL", userID)
}

func (s *SQLiteStore) GlobalTeams(sortBy string) []Team {
	teams := s.teamsWhere("")
	sortTeams(teams, sortBy)
	return teams
}

func (s *SQLiteStore) Tournaments() []Tournament {
	rows, err := s.db.Query("SELECT id, name, status FROM tournaments ORDER BY created_at DESC")
	if err != nil {
		return nil
	}
	defer rows.Close()

	var tournaments []Tournament
	for rows.Next() {
		var tournament Tournament
		if err := rows.Scan(&tournament.ID, &tournament.Name, &tournament.Status); err != nil {
			return nil
		}
		tournament.Teams = s.teamsForTournament(tournament.ID)
		tournaments = append(tournaments, tournament)
	}
	return tournaments
}

func (s *SQLiteStore) FindTeam(id string) (Team, bool) {
	return s.findTeam(id, false)
}

func (s *SQLiteStore) FindTeamIncludingRetired(id string) (Team, bool) {
	return s.findTeam(id, true)
}

func (s *SQLiteStore) findTeam(id string, includeDeleted bool) (Team, bool) {
	teams := s.teamsWhereWithDeleted(includeDeleted, "t.id = ?", id)
	if len(teams) == 0 {
		return Team{}, false
	}
	return teams[0], true
}

func (s *SQLiteStore) TeamHistory(teamID string) []MatchSummary {
	rows, err := s.db.Query(`
		SELECT id, team_a_id, team_b_id, COALESCE(team_a_snapshot, '{}'), COALESCE(team_b_snapshot, '{}'), COALESCE(score_a, 0), COALESCE(score_b, 0), COALESCE(ended_at, '')
		FROM matches
		WHERE (team_a_id = ? OR team_b_id = ?)
			AND COALESCE(ended_at, '') != ''
			AND ended_at <= ?
		ORDER BY ended_at DESC`,
		teamID, teamID, formatTime(time.Now().UTC()),
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var summaries []MatchSummary
	for rows.Next() {
		var id, teamAID, teamBID, teamASnapshot, teamBSnapshot, endedAt string
		var scoreA, scoreB int
		if err := rows.Scan(&id, &teamAID, &teamBID, &teamASnapshot, &teamBSnapshot, &scoreA, &scoreB, &endedAt); err != nil {
			return nil
		}
		teamA, okA := s.teamFromSnapshot(teamASnapshot, teamAID)
		teamB, okB := s.teamFromSnapshot(teamBSnapshot, teamBID)
		if !okA || !okB {
			continue
		}
		match := MatchResult{
			ID:         id,
			TeamA:      teamA,
			TeamB:      teamB,
			ScoreTeamA: scoreA,
			ScoreTeamB: scoreB,
			EndTime:    parseTimeOrZero(endedAt),
		}
		if events, ok := s.matchEvents(id); ok {
			match.Events = events
		}
		summaries = append(summaries, matchSummaryForTeam(match, teamID))
	}
	return summaries
}

func (s *SQLiteStore) TeamTransfers(teamID string) []TeamTransfer {
	rows, err := s.db.Query(`
		SELECT id, team_id, kind, COALESCE(before_snapshot, '{}'), COALESCE(after_snapshot, '{}'), summary, COALESCE(window_start, ''), created_at
		FROM team_transactions
		WHERE team_id = ?
		ORDER BY created_at ASC`, teamID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var transfers []TeamTransfer
	for rows.Next() {
		var transfer TeamTransfer
		var beforeSnapshot, afterSnapshot, windowStart, createdAt string
		if err := rows.Scan(&transfer.ID, &transfer.TeamID, &transfer.Kind, &beforeSnapshot, &afterSnapshot, &transfer.Summary, &windowStart, &createdAt); err != nil {
			return nil
		}
		transfer.Before = decodeTeamSnapshot(beforeSnapshot)
		transfer.After = decodeTeamSnapshot(afterSnapshot)
		transfer.WindowStart = parseTimeOrZero(windowStart)
		transfer.CreatedAt = parseTimeOrZero(createdAt)
		transfer.ChangedPlayers = changedPlayers(transfer.Before, transfer.After)
		if transfer.Summary == "" {
			transfer.Summary = transferSummary(transfer)
		}
		transfers = append(transfers, transfer)
	}
	return transfers
}

func (s *SQLiteStore) TransferWindow(teamID string) TransferWindow {
	window := currentTransferWindow(time.Now())
	var count int
	_ = s.db.QueryRow(`
		SELECT COUNT(*)
		FROM team_transactions
		WHERE team_id = ?
			AND kind = 'pokemon_transfer'
			AND window_start = ?`,
		teamID, formatTime(window.Start),
	).Scan(&count)
	if count > 0 {
		window.Used = true
		window.Remaining = 0
	}
	return window
}

func (s *SQLiteStore) SaveTeam(input TeamInput) (Team, error) {
	if input.UserID == "" {
		input.UserID = demoUserID
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return Team{}, ErrInvalidTeam
	}
	var err error
	input, err = s.normalizeTeamInput(input)
	if err != nil {
		return Team{}, err
	}

	if input.ID == "" {
		return s.createTeam(input)
	}
	return s.updateTeam(input)
}

func (s *SQLiteStore) DeleteTeam(id string, userID string) error {
	var frozen int
	err := s.db.QueryRow(
		"SELECT is_frozen FROM teams WHERE id = ? AND user_id = ? AND deleted_at IS NULL",
		id, userID,
	).Scan(&frozen)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTeamNotFound
	}
	if err != nil {
		return err
	}
	if frozen == 1 {
		return ErrTeamFrozen
	}

	result, err := s.db.Exec(
		"UPDATE teams SET deleted_at = ? WHERE id = ? AND user_id = ? AND deleted_at IS NULL",
		formatTime(time.Now().UTC()), id, userID,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrTeamNotFound
	}
	return nil
}

func (s *SQLiteStore) createTeam(input TeamInput) (Team, error) {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM teams WHERE user_id = ? AND deleted_at IS NULL", input.UserID).Scan(&count); err != nil {
		return Team{}, err
	}
	if count >= 6 {
		return Team{}, ErrTeamLimitReached
	}

	input.ID = newID("team")
	_, err := s.db.Exec(`
		INSERT INTO teams (
			id, user_id, name, goalkeeper_id, goalkeeper_ability, fixo_id, fixo_ability,
			ala_esquerda_id, ala_esquerda_ability, ala_direita_id, ala_direita_ability,
			pivo_id, pivo_ability, is_frozen, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?)`,
		input.ID, input.UserID, input.Name, input.GoalkeeperID, input.GoalkeeperAbility, input.FixoID, input.FixoAbility,
		input.AlaEsquerdaID, input.AlaEsquerdaAbility, input.AlaDireitaID, input.AlaDireitaAbility,
		input.PivoID, input.PivoAbility, formatTime(time.Now().UTC()),
	)
	if err != nil {
		return Team{}, err
	}
	team, ok := s.FindTeam(input.ID)
	if !ok {
		return Team{}, ErrTeamNotFound
	}
	if err := s.recordTeamTransaction(Team{}, team, "formation_created", team.CreatedAt); err != nil {
		return Team{}, err
	}
	return team, nil
}

func (s *SQLiteStore) updateTeam(input TeamInput) (Team, error) {
	var frozen int
	err := s.db.QueryRow(
		"SELECT is_frozen FROM teams WHERE id = ? AND user_id = ? AND deleted_at IS NULL",
		input.ID, input.UserID,
	).Scan(&frozen)
	if errors.Is(err, sql.ErrNoRows) {
		return Team{}, ErrTeamNotFound
	}
	if err != nil {
		return Team{}, err
	}
	if frozen == 1 {
		return Team{}, ErrTeamFrozen
	}
	before, ok := s.FindTeam(input.ID)
	if !ok {
		return Team{}, ErrTeamNotFound
	}
	after, err := teamFromInput(input, pokemonMapByID(s.Pokemon()))
	if err != nil {
		return Team{}, err
	}
	after.CreatedAt = before.CreatedAt
	playerChanges := changedPlayers(before, after)
	if len(playerChanges) > 1 {
		return Team{}, ErrTransferTooLarge
	}
	pokemonChanged := len(playerChanges) > 0
	if pokemonChanged && s.TransferWindow(input.ID).Used {
		return Team{}, ErrTransferLimit
	}

	result, err := s.db.Exec(`
		UPDATE teams
		SET name = ?,
			goalkeeper_id = ?,
			goalkeeper_ability = ?,
			fixo_id = ?,
			fixo_ability = ?,
			ala_esquerda_id = ?,
			ala_esquerda_ability = ?,
			ala_direita_id = ?,
			ala_direita_ability = ?,
			pivo_id = ?,
			pivo_ability = ?
		WHERE id = ? AND user_id = ?`,
		input.Name, input.GoalkeeperID, input.GoalkeeperAbility, input.FixoID, input.FixoAbility,
		input.AlaEsquerdaID, input.AlaEsquerdaAbility, input.AlaDireitaID, input.AlaDireitaAbility,
		input.PivoID, input.PivoAbility, input.ID, input.UserID,
	)
	if err != nil {
		return Team{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Team{}, err
	}
	if affected == 0 {
		return Team{}, ErrTeamNotFound
	}

	team, ok := s.FindTeam(input.ID)
	if !ok {
		return Team{}, ErrTeamNotFound
	}
	if pokemonChanged {
		if err := s.recordTeamTransaction(before, team, "pokemon_transfer", time.Now()); err != nil {
			return Team{}, err
		}
	}
	return team, nil
}

func (s *SQLiteStore) normalizeTeamInput(input TeamInput) (TeamInput, error) {
	return normalizeTeamInput(input, pokemonMapByID(s.Pokemon()))
}

func pokemonMapByID(pokemon []Pokemon) map[int]Pokemon {
	pokemonByID := make(map[int]Pokemon, len(pokemon))
	for _, item := range pokemon {
		pokemonByID[item.ID] = item
	}
	return pokemonByID
}

func decodeTeamSnapshot(snapshot string) Team {
	var team Team
	_ = json.Unmarshal([]byte(snapshot), &team)
	return team
}

func (s *SQLiteStore) recordTeamTransaction(before Team, after Team, kind string, at time.Time) error {
	if after.ID == "" {
		return nil
	}
	beforePayload := "{}"
	if before.ID != "" {
		payload, err := json.Marshal(before)
		if err != nil {
			return err
		}
		beforePayload = string(payload)
	}
	afterBytes, err := json.Marshal(after)
	if err != nil {
		return err
	}
	transfer := TeamTransfer{
		TeamID:         after.ID,
		Kind:           kind,
		Before:         before,
		After:          after,
		WindowStart:    currentTransferWindow(at).Start,
		CreatedAt:      at.UTC(),
		ChangedPlayers: changedPlayers(before, after),
	}
	_, err = s.db.Exec(`
		INSERT INTO team_transactions (
			id, team_id, kind, before_snapshot, after_snapshot, summary, window_start, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		newID("transaction"), after.ID, kind, beforePayload, string(afterBytes), transferSummary(transfer),
		formatTime(transfer.WindowStart), formatTime(transfer.CreatedAt),
	)
	return err
}

func (s *SQLiteStore) ensureInitialTeamTransactions() error {
	teams := s.teamsWhereWithDeleted(true, "")
	for _, team := range teams {
		var count int
		if err := s.db.QueryRow(
			"SELECT COUNT(*) FROM team_transactions WHERE team_id = ? AND kind = 'formation_created'",
			team.ID,
		).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if err := s.recordTeamTransaction(Team{}, team, "formation_created", team.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) MatchByID(id string) (MatchResult, bool) {
	return s.matchByQuery(`
		SELECT id, team_a_id, team_b_id, COALESCE(team_a_snapshot, '{}'), COALESCE(team_b_snapshot, '{}'), score_a, score_b, raw_json_output, COALESCE(start_time, ''), COALESCE(ended_at, '')
		FROM matches
		WHERE id = ?
		LIMIT 1`, id)
}

func (s *SQLiteStore) matchByQuery(query string, args ...any) (MatchResult, bool) {
	var match MatchResult
	var teamAID, teamBID, teamASnapshot, teamBSnapshot, rawJSON, startTime, endedAt string
	err := s.db.QueryRow(query, args...).Scan(&match.ID, &teamAID, &teamBID, &teamASnapshot, &teamBSnapshot, &match.ScoreTeamA, &match.ScoreTeamB, &rawJSON, &startTime, &endedAt)
	if err != nil {
		return MatchResult{}, false
	}

	teamA, okA := s.teamFromSnapshot(teamASnapshot, teamAID)
	teamB, okB := s.teamFromSnapshot(teamBSnapshot, teamBID)
	if !okA || !okB {
		return MatchResult{}, false
	}
	if err := json.Unmarshal([]byte(rawJSON), &match); err != nil {
		return MatchResult{}, false
	}
	match.TeamA = teamA
	match.TeamB = teamB
	if events, ok := s.matchEvents(match.ID); ok {
		match.Events = events
	}
	match.ScoreTeamA, match.ScoreTeamB = match.Score()
	if parsed, err := time.Parse(time.RFC3339Nano, startTime); err == nil {
		match.StartTime = parsed
	}
	if parsed, err := time.Parse(time.RFC3339Nano, endedAt); err == nil {
		match.EndTime = parsed
	}
	if match.EndTime.IsZero() && !match.StartTime.IsZero() {
		match.EndTime = match.StartTime.Add(matchDuration(match.Events))
	}
	return match, true
}

func (s *SQLiteStore) teamFromSnapshot(snapshot string, fallbackID string) (Team, bool) {
	var team Team
	if snapshot != "" && snapshot != "{}" {
		if err := json.Unmarshal([]byte(snapshot), &team); err == nil && team.ID != "" {
			return team, true
		}
	}
	return s.findTeam(fallbackID, true)
}

func (s *SQLiteStore) SaveMatch(match MatchResult) error {
	match.ScoreTeamA, match.ScoreTeamB = match.Score()
	if match.EndTime.IsZero() {
		match.EndTime = match.StartTime.Add(matchDuration(match.Events))
	}
	payload, err := json.Marshal(match)
	if err != nil {
		return err
	}
	teamASnapshot, err := json.Marshal(match.TeamA)
	if err != nil {
		return err
	}
	teamBSnapshot, err := json.Marshal(match.TeamB)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		INSERT INTO matches (
			id, mode, team_a_id, team_b_id, team_a_snapshot, team_b_snapshot, score_a, score_b,
			raw_json_output, start_time, ended_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		match.ID, "duel_random", match.TeamA.ID, match.TeamB.ID, string(teamASnapshot), string(teamBSnapshot), match.ScoreTeamA, match.ScoreTeamB,
		string(payload), formatTime(match.StartTime), formatTime(match.EndTime), formatTime(time.Now().UTC()),
	); err != nil {
		return err
	}
	for sequence, event := range match.Events {
		if _, err := tx.Exec(`
			INSERT INTO match_events (
				id, match_id, sequence, minute, type, narrative,
				dramatic_pause_seconds, team_id, pokemon_id
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			newID("event"), match.ID, sequence, event.Minute, event.Type, event.Narrative,
			event.DramaticPauseSeconds, nullString(event.TeamID), nullInt(event.PokemonID),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) matchEvents(matchID string) ([]MatchEvent, bool) {
	rows, err := s.db.Query(`
		SELECT minute, type, narrative, dramatic_pause_seconds, COALESCE(team_id, ''), COALESCE(pokemon_id, 0)
		FROM match_events
		WHERE match_id = ?
		ORDER BY sequence`, matchID)
	if err != nil {
		return nil, false
	}
	defer rows.Close()

	var events []MatchEvent
	for rows.Next() {
		var event MatchEvent
		if err := rows.Scan(
			&event.Minute,
			&event.Type,
			&event.Narrative,
			&event.DramaticPauseSeconds,
			&event.TeamID,
			&event.PokemonID,
		); err != nil {
			return nil, false
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, false
	}
	return events, len(events) > 0
}

func (s *SQLiteStore) teamsForTournament(tournamentID string) []Team {
	rows, err := s.db.Query(`
		SELECT team_id
		FROM tournament_registrations
		WHERE tournament_id = ?
		ORDER BY id`, tournamentID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var teams []Team
	for rows.Next() {
		var teamID string
		if err := rows.Scan(&teamID); err != nil {
			return nil
		}
		team, ok := s.FindTeam(teamID)
		if ok {
			teams = append(teams, team)
		}
	}
	return teams
}

func (s *SQLiteStore) teamsWhere(where string, args ...any) []Team {
	return s.teamsWhereWithDeleted(false, where, args...)
}

func (s *SQLiteStore) teamsWhereWithDeleted(includeDeleted bool, where string, args ...any) []Team {
	filter := where
	if !includeDeleted {
		if filter == "" {
			filter = "t.deleted_at IS NULL"
		} else {
			filter = "t.deleted_at IS NULL AND (" + filter + ")"
		}
	}
	whereSQL := ""
	if filter != "" {
		whereSQL = "WHERE " + filter
	}

	query := `
		SELECT
			t.id, t.user_id, t.name, t.is_frozen, t.created_at, COALESCE(t.deleted_at, ''),
			COALESCE(t.goalkeeper_ability, ''), COALESCE(t.fixo_ability, ''), COALESCE(t.ala_esquerda_ability, ''), COALESCE(t.ala_direita_ability, ''), COALESCE(t.pivo_ability, ''),
			g.id, g.name, COALESCE(g.artwork_url, ''), COALESCE(g.local_artwork_url, ''), g.type_1, COALESCE(g.type_2, ''), g.hp, g.attack, g.defense, g.special_attack, g.special_defense, g.speed, g.description, g.abilities,
			f.id, f.name, COALESCE(f.artwork_url, ''), COALESCE(f.local_artwork_url, ''), f.type_1, COALESCE(f.type_2, ''), f.hp, f.attack, f.defense, f.special_attack, f.special_defense, f.speed, f.description, f.abilities,
			ae.id, ae.name, COALESCE(ae.artwork_url, ''), COALESCE(ae.local_artwork_url, ''), ae.type_1, COALESCE(ae.type_2, ''), ae.hp, ae.attack, ae.defense, ae.special_attack, ae.special_defense, ae.speed, ae.description, ae.abilities,
			ad.id, ad.name, COALESCE(ad.artwork_url, ''), COALESCE(ad.local_artwork_url, ''), ad.type_1, COALESCE(ad.type_2, ''), ad.hp, ad.attack, ad.defense, ad.special_attack, ad.special_defense, ad.speed, ad.description, ad.abilities,
			p.id, p.name, COALESCE(p.artwork_url, ''), COALESCE(p.local_artwork_url, ''), p.type_1, COALESCE(p.type_2, ''), p.hp, p.attack, p.defense, p.special_attack, p.special_defense, p.speed, p.description, p.abilities
		FROM teams t
		JOIN pokemons g ON g.id = t.goalkeeper_id
		JOIN pokemons f ON f.id = t.fixo_id
		JOIN pokemons ae ON ae.id = t.ala_esquerda_id
		JOIN pokemons ad ON ad.id = t.ala_direita_id
		JOIN pokemons p ON p.id = t.pivo_id
		` + whereSQL + `
		ORDER BY t.created_at DESC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var teams []Team
	for rows.Next() {
		team, err := scanTeam(rows)
		if err != nil {
			return nil
		}
		teams = append(teams, team)
	}
	s.attachTeamRecords(teams)
	return teams
}

func (s *SQLiteStore) attachTeamRecords(teams []Team) {
	for i := range teams {
		teams[i].Record = s.teamRecord(teams[i].ID)
		teams[i].LeaderboardScore = leaderboardScore(teams[i].Record)
	}
}

func (s *SQLiteStore) teamRecord(teamID string) TeamRecord {
	rows, err := s.db.Query(`
		SELECT team_a_id, team_b_id, COALESCE(score_a, 0), COALESCE(score_b, 0)
		FROM matches
		WHERE (team_a_id = ? OR team_b_id = ?)
			AND COALESCE(ended_at, '') != ''
			AND ended_at <= ?`,
		teamID, teamID, formatTime(time.Now().UTC()),
	)
	if err != nil {
		return TeamRecord{}
	}
	defer rows.Close()

	var record TeamRecord
	for rows.Next() {
		var teamAID, teamBID string
		var scoreA, scoreB int
		if err := rows.Scan(&teamAID, &teamBID, &scoreA, &scoreB); err != nil {
			return TeamRecord{}
		}
		record.Played++
		switch {
		case scoreA == scoreB:
			record.Draws++
		case teamID == teamAID && scoreA > scoreB:
			record.Wins++
		case teamID == teamBID && scoreB > scoreA:
			record.Wins++
		default:
			record.Losses++
		}
	}
	return record
}

type pokemonScanner interface {
	Scan(dest ...any) error
}

func scanPokemon(scanner pokemonScanner) (Pokemon, error) {
	var pokemon Pokemon
	err := scanner.Scan(
		&pokemon.ID, &pokemon.Name, &pokemon.ArtworkURL, &pokemon.LocalArtworkURL, &pokemon.Type1, &pokemon.Type2, &pokemon.HP,
		&pokemon.Attack, &pokemon.Defense, &pokemon.SpecialAttack,
		&pokemon.SpecialDefense, &pokemon.Speed, &pokemon.Description, &pokemon.Abilities,
	)
	return ensurePokemonArtwork(pokemon), err
}

func scanTeam(rows *sql.Rows) (Team, error) {
	var team Team
	var frozen int
	var createdAt string
	var deletedAt string
	err := rows.Scan(
		&team.ID, &team.UserID, &team.Name, &frozen, &createdAt, &deletedAt,
		&team.GoalkeeperAbility, &team.FixoAbility, &team.AlaEsquerdaAbility, &team.AlaDireitaAbility, &team.PivoAbility,
		&team.Goalkeeper.ID, &team.Goalkeeper.Name, &team.Goalkeeper.ArtworkURL, &team.Goalkeeper.LocalArtworkURL, &team.Goalkeeper.Type1, &team.Goalkeeper.Type2, &team.Goalkeeper.HP, &team.Goalkeeper.Attack, &team.Goalkeeper.Defense, &team.Goalkeeper.SpecialAttack, &team.Goalkeeper.SpecialDefense, &team.Goalkeeper.Speed, &team.Goalkeeper.Description, &team.Goalkeeper.Abilities,
		&team.Fixo.ID, &team.Fixo.Name, &team.Fixo.ArtworkURL, &team.Fixo.LocalArtworkURL, &team.Fixo.Type1, &team.Fixo.Type2, &team.Fixo.HP, &team.Fixo.Attack, &team.Fixo.Defense, &team.Fixo.SpecialAttack, &team.Fixo.SpecialDefense, &team.Fixo.Speed, &team.Fixo.Description, &team.Fixo.Abilities,
		&team.AlaEsquerda.ID, &team.AlaEsquerda.Name, &team.AlaEsquerda.ArtworkURL, &team.AlaEsquerda.LocalArtworkURL, &team.AlaEsquerda.Type1, &team.AlaEsquerda.Type2, &team.AlaEsquerda.HP, &team.AlaEsquerda.Attack, &team.AlaEsquerda.Defense, &team.AlaEsquerda.SpecialAttack, &team.AlaEsquerda.SpecialDefense, &team.AlaEsquerda.Speed, &team.AlaEsquerda.Description, &team.AlaEsquerda.Abilities,
		&team.AlaDireita.ID, &team.AlaDireita.Name, &team.AlaDireita.ArtworkURL, &team.AlaDireita.LocalArtworkURL, &team.AlaDireita.Type1, &team.AlaDireita.Type2, &team.AlaDireita.HP, &team.AlaDireita.Attack, &team.AlaDireita.Defense, &team.AlaDireita.SpecialAttack, &team.AlaDireita.SpecialDefense, &team.AlaDireita.Speed, &team.AlaDireita.Description, &team.AlaDireita.Abilities,
		&team.Pivo.ID, &team.Pivo.Name, &team.Pivo.ArtworkURL, &team.Pivo.LocalArtworkURL, &team.Pivo.Type1, &team.Pivo.Type2, &team.Pivo.HP, &team.Pivo.Attack, &team.Pivo.Defense, &team.Pivo.SpecialAttack, &team.Pivo.SpecialDefense, &team.Pivo.Speed, &team.Pivo.Description, &team.Pivo.Abilities,
	)
	if err != nil {
		return Team{}, err
	}
	team.Goalkeeper = ensurePokemonArtwork(team.Goalkeeper)
	team.Fixo = ensurePokemonArtwork(team.Fixo)
	team.AlaEsquerda = ensurePokemonArtwork(team.AlaEsquerda)
	team.AlaDireita = ensurePokemonArtwork(team.AlaDireita)
	team.Pivo = ensurePokemonArtwork(team.Pivo)
	team.IsFrozen = frozen == 1
	team.IsRetired = deletedAt != ""
	if parsed, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		team.CreatedAt = parsed
	}
	return team, nil
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullInt(value int) any {
	if value == 0 {
		return nil
	}
	return value
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTimeOrZero(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func countRows(db *sql.DB, table string) (int, error) {
	var count int
	if err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *SQLiteStore) backfillPokemonArtworkURLs() error {
	_, err := s.db.Exec("UPDATE pokemons SET artwork_url = 'https://raw.githubusercontent.com/PokeAPI/sprites/master/sprites/pokemon/other/official-artwork/' || id || '.png' WHERE artwork_url IS NULL OR artwork_url = ''")
	return err
}

func (s *SQLiteStore) ensureColumn(table string, column string, definition string) error {
	rows, err := s.db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue any
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec("ALTER TABLE " + table + " ADD COLUMN " + column + " " + definition)
	return err
}
