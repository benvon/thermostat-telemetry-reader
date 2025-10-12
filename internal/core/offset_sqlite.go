package core

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// SQLiteOffsetStore implements OffsetStore using SQLite
// This provides persistent storage of polling offsets across restarts
type SQLiteOffsetStore struct {
	db *sql.DB
}

// NewSQLiteOffsetStore creates a new SQLite-based offset store
// The dbPath parameter specifies the path to the SQLite database file
func NewSQLiteOffsetStore(dbPath string) (*SQLiteOffsetStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite database: %w", err)
	}

	store := &SQLiteOffsetStore{db: db}
	if err := store.initSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing schema: %w", err)
	}

	return store, nil
}

// initSchema creates the necessary tables if they don't exist
func (s *SQLiteOffsetStore) initSchema() error {
	schema := `
		CREATE TABLE IF NOT EXISTS offset_tracking (
			thermostat_id TEXT PRIMARY KEY,
			last_runtime_time TEXT,
			last_snapshot_time TEXT,
			updated_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_updated_at ON offset_tracking(updated_at);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("creating schema: %w", err)
	}

	return nil
}

// GetLastRuntimeTime returns the last runtime timestamp for a thermostat
func (s *SQLiteOffsetStore) GetLastRuntimeTime(ctx context.Context, thermostatID string) (time.Time, error) {
	var timeStr sql.NullString
	query := `SELECT last_runtime_time FROM offset_tracking WHERE thermostat_id = ?`

	err := s.db.QueryRowContext(ctx, query, thermostatID).Scan(&timeStr)
	if err == sql.ErrNoRows {
		return time.Time{}, nil // Return zero time if not found
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("querying last runtime time: %w", err)
	}

	if !timeStr.Valid || timeStr.String == "" {
		return time.Time{}, nil
	}

	t, err := time.Parse(time.RFC3339, timeStr.String)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing timestamp: %w", err)
	}

	return t, nil
}

// SetLastRuntimeTime sets the last runtime timestamp for a thermostat
func (s *SQLiteOffsetStore) SetLastRuntimeTime(ctx context.Context, thermostatID string, timestamp time.Time) error {
	query := `
		INSERT INTO offset_tracking (thermostat_id, last_runtime_time, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(thermostat_id) DO UPDATE SET
			last_runtime_time = excluded.last_runtime_time,
			updated_at = excluded.updated_at
	`

	_, err := s.db.ExecContext(ctx, query, thermostatID, timestamp.Format(time.RFC3339), time.Now().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("setting last runtime time: %w", err)
	}

	return nil
}

// GetLastSnapshotTime returns the last snapshot timestamp for a thermostat
func (s *SQLiteOffsetStore) GetLastSnapshotTime(ctx context.Context, thermostatID string) (time.Time, error) {
	var timeStr sql.NullString
	query := `SELECT last_snapshot_time FROM offset_tracking WHERE thermostat_id = ?`

	err := s.db.QueryRowContext(ctx, query, thermostatID).Scan(&timeStr)
	if err == sql.ErrNoRows {
		return time.Time{}, nil // Return zero time if not found
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("querying last snapshot time: %w", err)
	}

	if !timeStr.Valid || timeStr.String == "" {
		return time.Time{}, nil
	}

	t, err := time.Parse(time.RFC3339, timeStr.String)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing timestamp: %w", err)
	}

	return t, nil
}

// SetLastSnapshotTime sets the last snapshot timestamp for a thermostat
func (s *SQLiteOffsetStore) SetLastSnapshotTime(ctx context.Context, thermostatID string, timestamp time.Time) error {
	query := `
		INSERT INTO offset_tracking (thermostat_id, last_snapshot_time, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(thermostat_id) DO UPDATE SET
			last_snapshot_time = excluded.last_snapshot_time,
			updated_at = excluded.updated_at
	`

	_, err := s.db.ExecContext(ctx, query, thermostatID, timestamp.Format(time.RFC3339), time.Now().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("setting last snapshot time: %w", err)
	}

	return nil
}

// Close closes the database connection
func (s *SQLiteOffsetStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
