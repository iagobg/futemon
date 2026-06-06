package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const migrationSQL = `
CREATE TABLE IF NOT EXISTS pokemons (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
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
  gemini_api_key TEXT,
  role TEXT NOT NULL DEFAULT 'user',
  deleted_at TEXT
);

CREATE TABLE IF NOT EXISTS teams (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id),
  name TEXT NOT NULL,
  goalkeeper_id INTEGER NOT NULL REFERENCES pokemons(id),
  fixo_id INTEGER NOT NULL REFERENCES pokemons(id),
  ala_esquerda_id INTEGER NOT NULL REFERENCES pokemons(id),
  ala_direita_id INTEGER NOT NULL REFERENCES pokemons(id),
  pivo_id INTEGER NOT NULL REFERENCES pokemons(id),
  is_frozen INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  deleted_at TEXT
);

CREATE TABLE IF NOT EXISTS matches (
  id TEXT PRIMARY KEY,
  mode TEXT NOT NULL,
  team_a_id TEXT NOT NULL REFERENCES teams(id),
  team_b_id TEXT NOT NULL REFERENCES teams(id),
  score_a INTEGER,
  score_b INTEGER,
  raw_json_output TEXT NOT NULL DEFAULT '{}',
  start_time TEXT,
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
);`

type SQLiteStore struct {
	db     *sql.DB
	cipher *KeyCipher
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
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

	return store, nil
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
	return s.ensureColumn("teams", "deleted_at", "TEXT")
}

func (s *SQLiteStore) seedDemoData() error {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM pokemons").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, pokemon := range samplePokemon() {
		if _, err := tx.Exec(`
			INSERT INTO pokemons (
				id, name, type_1, type_2, hp, attack, defense, special_attack,
				special_defense, speed, description, abilities
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			pokemon.ID, pokemon.Name, pokemon.Type1, nullString(pokemon.Type2), pokemon.HP,
			pokemon.Attack, pokemon.Defense, pokemon.SpecialAttack, pokemon.SpecialDefense,
			pokemon.Speed, pokemon.Description, pokemon.Abilities,
		); err != nil {
			return err
		}
	}

	now := time.Now().UTC()
	if _, err := tx.Exec(`
		INSERT INTO users (id, google_id, display_name, email, role)
		VALUES
			('user-demo', 'demo-google-id', 'Treinador Demo', 'demo@futemon.local', 'admin'),
			('user-rival', 'rival-google-id', 'Rival Local', 'rival@futemon.local', 'user'),
			('user-misty', 'misty-google-id', 'Lider do Ginasio Azul', 'misty@futemon.local', 'user')`); err != nil {
		return err
	}

	teams := []struct {
		ID, UserID, Name                                string
		Goalkeeper, Fixo, AlaEsquerda, AlaDireita, Pivo int
		IsFrozen                                        bool
		CreatedAt                                       time.Time
	}{
		{"team-kanto-press", "user-demo", "Kanto Press", 9, 6, 25, 4, 68, false, now.Add(-72 * time.Hour)},
		{"team-paleta-bolada", "user-rival", "Paleta Bolada", 143, 95, 26, 7, 149, true, now.Add(-48 * time.Hour)},
		{"team-ginasio-azul", "user-misty", "Ginasio Azul FC", 130, 55, 121, 7, 131, false, now.Add(-24 * time.Hour)},
	}
	for _, team := range teams {
		if _, err := tx.Exec(`
			INSERT INTO teams (
				id, user_id, name, goalkeeper_id, fixo_id, ala_esquerda_id,
				ala_direita_id, pivo_id, is_frozen, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			team.ID, team.UserID, team.Name, team.Goalkeeper, team.Fixo, team.AlaEsquerda,
			team.AlaDireita, team.Pivo, boolInt(team.IsFrozen), formatTime(team.CreatedAt),
		); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(`
		INSERT INTO tournaments (id, name, status, created_by, created_at)
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
			INSERT INTO tournament_registrations (id, tournament_id, team_id)
			VALUES (?, ?, ?)`,
			registration.ID, registration.TournamentID, registration.TeamID,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) Pokemon() []Pokemon {
	rows, err := s.db.Query(`
		SELECT id, name, type_1, COALESCE(type_2, ''), hp, attack, defense,
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

	apiKey := user.GeminiAPIKey
	if input.ClearAPIKey {
		apiKey = ""
	} else if strings.TrimSpace(input.GeminiAPIKey) != "" {
		if s.cipher == nil {
			return User{}, ErrEncryptionKeyRequired
		}
		encrypted, err := s.cipher.Encrypt(strings.TrimSpace(input.GeminiAPIKey))
		if err != nil {
			return User{}, err
		}
		apiKey = encrypted
	}

	if _, err := s.db.Exec(
		"UPDATE users SET display_name = ?, gemini_api_key = ? WHERE id = ?",
		input.DisplayName, nullString(apiKey), input.UserID,
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
		SELECT id, google_id, display_name, email, COALESCE(gemini_api_key, ''), role
		FROM users
		WHERE id = ? AND deleted_at IS NULL`, id).Scan(
		&user.ID, &user.GoogleID, &user.DisplayName, &user.Email, &user.GeminiAPIKey, &user.Role,
	)
	if err != nil {
		return User{}, false
	}
	user.HasGeminiAPIKey = user.GeminiAPIKey != ""
	return user, true
}

func (s *SQLiteStore) MyTeams() []Team {
	return s.teamsWhere("t.user_id = ?", "user-demo")
}

func (s *SQLiteStore) GlobalTeams() []Team {
	return s.teamsWhere("")
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

func (s *SQLiteStore) findTeam(id string, includeDeleted bool) (Team, bool) {
	teams := s.teamsWhereWithDeleted(includeDeleted, "t.id = ?", id)
	if len(teams) == 0 {
		return Team{}, false
	}
	return teams[0], true
}

func (s *SQLiteStore) SaveTeam(input TeamInput) (Team, error) {
	if input.UserID == "" {
		input.UserID = demoUserID
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return Team{}, ErrInvalidTeam
	}
	if err := s.validatePokemonIDs(input); err != nil {
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
			id, user_id, name, goalkeeper_id, fixo_id, ala_esquerda_id,
			ala_direita_id, pivo_id, is_frozen, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?)`,
		input.ID, input.UserID, input.Name, input.GoalkeeperID, input.FixoID,
		input.AlaEsquerdaID, input.AlaDireitaID, input.PivoID, formatTime(time.Now().UTC()),
	)
	if err != nil {
		return Team{}, err
	}
	team, ok := s.FindTeam(input.ID)
	if !ok {
		return Team{}, ErrTeamNotFound
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

	result, err := s.db.Exec(`
		UPDATE teams
		SET name = ?,
			goalkeeper_id = ?,
			fixo_id = ?,
			ala_esquerda_id = ?,
			ala_direita_id = ?,
			pivo_id = ?
		WHERE id = ? AND user_id = ?`,
		input.Name, input.GoalkeeperID, input.FixoID, input.AlaEsquerdaID,
		input.AlaDireitaID, input.PivoID, input.ID, input.UserID,
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
	return team, nil
}

func (s *SQLiteStore) validatePokemonIDs(input TeamInput) error {
	ids := []int{input.GoalkeeperID, input.FixoID, input.AlaEsquerdaID, input.AlaDireitaID, input.PivoID}
	for _, id := range ids {
		var exists int
		if err := s.db.QueryRow("SELECT 1 FROM pokemons WHERE id = ?", id).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
			return ErrPokemonNotFound
		} else if err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) LatestMatch() (MatchResult, bool) {
	var match MatchResult
	var teamAID, teamBID, rawJSON, startTime string
	err := s.db.QueryRow(`
		SELECT id, team_a_id, team_b_id, score_a, score_b, raw_json_output, COALESCE(start_time, '')
		FROM matches
		ORDER BY created_at DESC
		LIMIT 1`).Scan(&match.ID, &teamAID, &teamBID, &match.ScoreTeamA, &match.ScoreTeamB, &rawJSON, &startTime)
	if err != nil {
		return MatchResult{}, false
	}

	teamA, okA := s.findTeam(teamAID, true)
	teamB, okB := s.findTeam(teamBID, true)
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
	return match, true
}

func (s *SQLiteStore) SetLatestMatch(match MatchResult) {
	match.ScoreTeamA, match.ScoreTeamB = match.Score()
	payload, err := json.Marshal(match)
	if err != nil {
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		INSERT INTO matches (
			id, mode, team_a_id, team_b_id, score_a, score_b,
			raw_json_output, start_time, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		match.ID, "duel_random", match.TeamA.ID, match.TeamB.ID, match.ScoreTeamA, match.ScoreTeamB,
		string(payload), formatTime(match.StartTime), formatTime(time.Now().UTC()),
	); err != nil {
		return
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
			return
		}
	}
	_ = tx.Commit()
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
			t.id, t.user_id, t.name, t.is_frozen, t.created_at,
			g.id, g.name, g.type_1, COALESCE(g.type_2, ''), g.hp, g.attack, g.defense, g.special_attack, g.special_defense, g.speed, g.description, g.abilities,
			f.id, f.name, f.type_1, COALESCE(f.type_2, ''), f.hp, f.attack, f.defense, f.special_attack, f.special_defense, f.speed, f.description, f.abilities,
			ae.id, ae.name, ae.type_1, COALESCE(ae.type_2, ''), ae.hp, ae.attack, ae.defense, ae.special_attack, ae.special_defense, ae.speed, ae.description, ae.abilities,
			ad.id, ad.name, ad.type_1, COALESCE(ad.type_2, ''), ad.hp, ad.attack, ad.defense, ad.special_attack, ad.special_defense, ad.speed, ad.description, ad.abilities,
			p.id, p.name, p.type_1, COALESCE(p.type_2, ''), p.hp, p.attack, p.defense, p.special_attack, p.special_defense, p.speed, p.description, p.abilities
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
	return teams
}

type pokemonScanner interface {
	Scan(dest ...any) error
}

func scanPokemon(scanner pokemonScanner) (Pokemon, error) {
	var pokemon Pokemon
	err := scanner.Scan(
		&pokemon.ID, &pokemon.Name, &pokemon.Type1, &pokemon.Type2, &pokemon.HP,
		&pokemon.Attack, &pokemon.Defense, &pokemon.SpecialAttack,
		&pokemon.SpecialDefense, &pokemon.Speed, &pokemon.Description, &pokemon.Abilities,
	)
	return pokemon, err
}

func scanTeam(rows *sql.Rows) (Team, error) {
	var team Team
	var frozen int
	var createdAt string
	err := rows.Scan(
		&team.ID, &team.UserID, &team.Name, &frozen, &createdAt,
		&team.Goalkeeper.ID, &team.Goalkeeper.Name, &team.Goalkeeper.Type1, &team.Goalkeeper.Type2, &team.Goalkeeper.HP, &team.Goalkeeper.Attack, &team.Goalkeeper.Defense, &team.Goalkeeper.SpecialAttack, &team.Goalkeeper.SpecialDefense, &team.Goalkeeper.Speed, &team.Goalkeeper.Description, &team.Goalkeeper.Abilities,
		&team.Fixo.ID, &team.Fixo.Name, &team.Fixo.Type1, &team.Fixo.Type2, &team.Fixo.HP, &team.Fixo.Attack, &team.Fixo.Defense, &team.Fixo.SpecialAttack, &team.Fixo.SpecialDefense, &team.Fixo.Speed, &team.Fixo.Description, &team.Fixo.Abilities,
		&team.AlaEsquerda.ID, &team.AlaEsquerda.Name, &team.AlaEsquerda.Type1, &team.AlaEsquerda.Type2, &team.AlaEsquerda.HP, &team.AlaEsquerda.Attack, &team.AlaEsquerda.Defense, &team.AlaEsquerda.SpecialAttack, &team.AlaEsquerda.SpecialDefense, &team.AlaEsquerda.Speed, &team.AlaEsquerda.Description, &team.AlaEsquerda.Abilities,
		&team.AlaDireita.ID, &team.AlaDireita.Name, &team.AlaDireita.Type1, &team.AlaDireita.Type2, &team.AlaDireita.HP, &team.AlaDireita.Attack, &team.AlaDireita.Defense, &team.AlaDireita.SpecialAttack, &team.AlaDireita.SpecialDefense, &team.AlaDireita.Speed, &team.AlaDireita.Description, &team.AlaDireita.Abilities,
		&team.Pivo.ID, &team.Pivo.Name, &team.Pivo.Type1, &team.Pivo.Type2, &team.Pivo.HP, &team.Pivo.Attack, &team.Pivo.Defense, &team.Pivo.SpecialAttack, &team.Pivo.SpecialDefense, &team.Pivo.Speed, &team.Pivo.Description, &team.Pivo.Abilities,
	)
	if err != nil {
		return Team{}, err
	}
	team.IsFrozen = frozen == 1
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

func countRows(db *sql.DB, table string) (int, error) {
	var count int
	if err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
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
