-- PostgreSQL version. Run in the banting_raket_db database.
CREATE TABLE IF NOT EXISTS players (
  id VARCHAR(20) PRIMARY KEY,
  nama VARCHAR(100) NOT NULL,
  initial_level VARCHAR(2) NULL
);
CREATE TABLE IF NOT EXISTS sessions (
  id SERIAL PRIMARY KEY,
  play_date DATE NOT NULL UNIQUE,
  courts SMALLINT NOT NULL DEFAULT 2
);
CREATE TABLE IF NOT EXISTS session_players (
  session_id INT NOT NULL REFERENCES sessions(id),
  player_id VARCHAR(20) NOT NULL REFERENCES players(id),
  level VARCHAR(2) NULL,
  payment VARCHAR(5) NOT NULL DEFAULT 'belum' CHECK (payment IN ('qris','cash','belum')),
  PRIMARY KEY (session_id, player_id)
);
CREATE TABLE IF NOT EXISTS matches (
  id SERIAL PRIMARY KEY,
  session_id INT NOT NULL REFERENCES sessions(id),
  court SMALLINT NOT NULL,
  score1 SMALLINT NULL, score2 SMALLINT NULL,
  p1 VARCHAR(20), p2 VARCHAR(20), p3 VARCHAR(20), p4 VARCHAR(20), -- p1+p2 vs p3+p4
  finished_at TIMESTAMP NULL
);

ALTER TABLE session_players ADD COLUMN IF NOT EXISTS checked_in_at TIMESTAMP NOT NULL DEFAULT now();

-- v3: many games (sessions) per day, each with a name and a system
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_play_date_key;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS name VARCHAR(100) NOT NULL DEFAULT 'Open Play';
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS mode VARCHAR(10) NOT NULL DEFAULT 'regular'; -- regular | team | americano
CREATE TABLE IF NOT EXISTS teams (
  id SERIAL PRIMARY KEY, session_id INT NOT NULL,
  p1 VARCHAR(20) NOT NULL, p2 VARCHAR(20) NOT NULL,
  grp SMALLINT NOT NULL, category VARCHAR(10) NOT NULL DEFAULT 'Mixed'
);

-- v3b: guests (added by hand in team/americano games) live only in session_players
ALTER TABLE session_players DROP CONSTRAINT IF EXISTS session_players_player_id_fkey;
ALTER TABLE session_players ADD COLUMN IF NOT EXISTS guest_name VARCHAR(100) NULL;

-- v4: login, share links, live snapshot for viewers
CREATE TABLE IF NOT EXISTS users (id SERIAL PRIMARY KEY, username VARCHAR(50) UNIQUE NOT NULL, password_hash VARCHAR(100) NOT NULL);
CREATE TABLE IF NOT EXISTS auth_tokens (token VARCHAR(64) PRIMARY KEY, user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE, created_at TIMESTAMP NOT NULL DEFAULT now());
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS share_token VARCHAR(32) NULL;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS live TEXT NOT NULL DEFAULT '{}';

-- v6: how long a match took
ALTER TABLE matches ADD COLUMN IF NOT EXISTS duration_min SMALLINT NULL;

-- v10: live matches (ongoing + upcoming) are saved so a refresh or another admin sees them
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS state TEXT NOT NULL DEFAULT '{}';
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS state_v INT NOT NULL DEFAULT 0;

-- v11: see and end logins per device
ALTER TABLE auth_tokens ADD COLUMN IF NOT EXISTS ua VARCHAR(200) NOT NULL DEFAULT '';
ALTER TABLE auth_tokens ADD COLUMN IF NOT EXISTS last_seen TIMESTAMP NOT NULL DEFAULT now();

-- v12: custom group names for team games (shown as "Group <name>")
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS group_a VARCHAR(40) NOT NULL DEFAULT 'A';
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS group_b VARCHAR(40) NOT NULL DEFAULT 'B';
