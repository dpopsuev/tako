package cerebrum

import "github.com/jmoiron/sqlx"

func MigrateSchema(db *sqlx.DB) error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS pipes (
			name VARCHAR(255) PRIMARY KEY,
			description TEXT,
			embedding JSON,
			replays INT DEFAULT 0,
			usage_count INT DEFAULT 0,
			last_played DATETIME NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS pipe_steps (
			pipe_name VARCHAR(255),
			step_id VARCHAR(255),
			call_name VARCHAR(255),
			args JSON,
			depends_on JSON,
			expected_hash VARBINARY(32),
			confidence DOUBLE DEFAULT 0.6,
			step_order INT DEFAULT 0,
			PRIMARY KEY (pipe_name, step_id)
		)`,
		`CREATE TABLE IF NOT EXISTS turn_records (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			molecule_id VARCHAR(255),
			turn INT,
			phase VARCHAR(100),
			gear VARCHAR(20),
			domain VARCHAR(50),
			model_name VARCHAR(255),
			tokens_in INT,
			tokens_out INT,
			tool_calls INT,
			distance DOUBLE,
			delta_distance DOUBLE,
			momentum DOUBLE,
			unmet_count INT,
			reflex_hits INT,
			elapsed_ms BIGINT,
			navigator_decision VARCHAR(255),
			regulator_depth VARCHAR(255),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_turn_molecule (molecule_id)
		)`,
		`CREATE TABLE IF NOT EXISTS session_summaries (
			molecule_id VARCHAR(255) PRIMARY KEY,
			total_turns INT,
			total_tokens_in INT,
			total_tokens_out INT,
			total_tool_calls INT,
			oae DOUBLE,
			gear_novel_pct DOUBLE,
			gear_familiar_pct DOUBLE,
			gear_intuition_pct DOUBLE,
			gear_reflex_pct DOUBLE,
			reflex_hits INT,
			reflex_coverage DOUBLE,
			llm_calls INT,
			reflex_fires INT,
			avg_turn_ms BIGINT,
			sealed BOOLEAN,
			final_distance DOUBLE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, ddl := range tables {
		if _, err := db.Exec(ddl); err != nil {
			return err
		}
	}
	return nil
}
