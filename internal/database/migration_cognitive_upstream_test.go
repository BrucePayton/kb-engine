package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPostgresCognitiveMigrationAfterUpstream137(t *testing.T) {
	dsn := os.Getenv("WEKNORA_MIGRATION_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set WEKNORA_MIGRATION_TEST_POSTGRES_DSN to run PostgreSQL migration tests")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	root := sqliteRepoRoot(t)
	read := func(name string) string {
		script, err := os.ReadFile(filepath.Join(root, "migrations", "versioned", name))
		require.NoError(t, err)
		return string(script)
	}
	migration := read("000138_cognitive_paths.up.sql")
	for _, legacy := range []bool{false, true} {
		name := "fresh"
		if legacy {
			name = "legacy_cognitive_137"
		}
		t.Run(name, func(t *testing.T) {
			tx, err := db.Begin()
			require.NoError(t, err)
			t.Cleanup(func() { _ = tx.Rollback() })
			exec := func(script string) { _, err := tx.Exec(script); require.NoError(t, err) }
			exec(`CREATE TEMP TABLE im_channel_sessions (
    id TEXT PRIMARY KEY, platform TEXT, user_id TEXT, chat_id TEXT,
    tenant_id BIGINT, agent_id TEXT, thread_id TEXT DEFAULT '', deleted_at TIMESTAMPTZ
   )`)
			// Keep all tables and indexes isolated even against a nonempty test database.
			exec("SET LOCAL search_path TO pg_temp")
			exec(`CREATE UNIQUE INDEX idx_channel_lookup
    ON im_channel_sessions (platform, user_id, chat_id, tenant_id, agent_id)
    WHERE deleted_at IS NULL`)
			exec(`INSERT INTO im_channel_sessions VALUES ('user', 'feishu', 'alice', 'chat', 1, 'agent', '', NULL)`)
			if legacy {
				old, _, found := strings.Cut(migration, "-- Older fork deployments")
				require.True(t, found)
				exec(old)
				exec(`INSERT INTO cognitive_paths (id, tenant_id, knowledge_base_id, path_key, fact_hash)
     VALUES ('retained', 1, 'kb', 'path', 'hash')`)
			} else {
				exec(read("000137_im_session_mode_indexes.up.sql"))
			}
			exec(migration)
			exec(migration) // Reapplying must preserve existing data and index semantics.
			exec(`INSERT INTO im_channel_sessions VALUES
    ('thread1', 'feishu', 'alice', 'chat', 1, 'agent', 'one', NULL),
    ('thread2', 'feishu', 'alice', 'chat', 1, 'agent', 'two', NULL)`)
			var count int
			require.NoError(t, tx.QueryRow("SELECT count(*) FROM im_channel_sessions").Scan(&count))
			require.Equal(t, 3, count)
			if legacy {
				row := tx.QueryRow("SELECT count(*) FROM cognitive_paths WHERE id = 'retained'")
				require.NoError(t, row.Scan(&count))
				require.Equal(t, 1, count)
			}
		})
	}
}
