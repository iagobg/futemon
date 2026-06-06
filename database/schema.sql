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
);
