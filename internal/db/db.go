package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/earworm/vibesboard/internal/model"
	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when a vibe does not exist.
var ErrNotFound = errors.New("vibe not found")

const createTableSQL = `
CREATE TABLE IF NOT EXISTS vibes (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    youtube_url   TEXT NOT NULL,
    video_title   TEXT DEFAULT '',
    thumbnail_url TEXT DEFAULT '',
    thought       TEXT NOT NULL,
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);`

// Store wraps a SQL database connection for vibe persistence.
type Store struct {
	db *sql.DB
}

// New opens a SQLite database at dbPath and ensures the vibes table exists.
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if _, err := db.Exec(createTableSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("create vibes table: %w", err)
	}

	// Migrate: add columns if they don't exist.
	migrations := []string{
		"ALTER TABLE vibes ADD COLUMN likes INTEGER DEFAULT 0",
		"ALTER TABLE vibes ADD COLUMN dislikes INTEGER DEFAULT 0",
		"ALTER TABLE vibes ADD COLUMN author TEXT DEFAULT ''",
	}
	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			// SQLite returns "duplicate column name" if column already exists — ignore it.
			if !strings.Contains(err.Error(), "duplicate column") {
				db.Close()
				return nil, fmt.Errorf("migration: %w", err)
			}
		}
	}

	return &Store{db: db}, nil
}

// Insert adds a new vibe to the database and populates its ID and CreatedAt fields.
func (s *Store) Insert(v *model.Vibe) error {
	res, err := s.db.Exec(
		"INSERT INTO vibes (youtube_url, video_title, thumbnail_url, thought, author) VALUES (?, ?, ?, ?, ?)",
		v.YouTubeURL, v.VideoTitle, v.ThumbnailURL, v.Thought, v.Author,
	)
	if err != nil {
		return fmt.Errorf("insert vibe: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}
	v.ID = id

	// Query back the created_at value set by the database default.
	var createdAtStr string
	err = s.db.QueryRow("SELECT created_at FROM vibes WHERE id = ?", id).Scan(&createdAtStr)
	if err != nil {
		return fmt.Errorf("query created_at: %w", err)
	}

	createdAt, err := parseTimestamp(createdAtStr)
	if err != nil {
		return fmt.Errorf("parse created_at: %w", err)
	}
	v.CreatedAt = createdAt
	v.Likes = 0
	v.Dislikes = 0
	v.NetScore = 0

	return nil
}

// ListRecent returns the most recent vibes ordered by created_at descending, limited to n rows.
func (s *Store) ListRecent(limit int) ([]model.Vibe, error) {
	rows, err := s.db.Query(
		"SELECT id, youtube_url, video_title, thumbnail_url, thought, likes, dislikes, (likes - dislikes) AS net_score, created_at, author FROM vibes ORDER BY created_at DESC, id DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query vibes: %w", err)
	}
	defer rows.Close()

	var vibes []model.Vibe
	for rows.Next() {
		var v model.Vibe
		var createdAtStr string
		if err := rows.Scan(&v.ID, &v.YouTubeURL, &v.VideoTitle, &v.ThumbnailURL, &v.Thought, &v.Likes, &v.Dislikes, &v.NetScore, &createdAtStr, &v.Author); err != nil {
			return nil, fmt.Errorf("scan vibe: %w", err)
		}
		createdAt, err := parseTimestamp(createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("parse created_at: %w", err)
		}
		v.CreatedAt = createdAt
		vibes = append(vibes, v)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate vibes: %w", err)
	}

	return vibes, nil
}

// Delete removes a vibe by ID. Returns false if no row was affected.
func (s *Store) Delete(id int64) (bool, error) {
	res, err := s.db.Exec("DELETE FROM vibes WHERE id = ?", id)
	if err != nil {
		return false, fmt.Errorf("delete vibe: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}

	return affected > 0, nil
}

// Vote atomically increments the likes or dislikes column for the given vibe ID.
// direction must be "up" (increments likes) or "down" (increments dislikes).
// Returns the updated Vibe with current counts, or ErrNotFound if the vibe doesn't exist.
func (s *Store) Vote(id int64, direction string) (*model.Vibe, error) {
	var col string
	switch direction {
	case "up":
		col = "likes"
	case "down":
		col = "dislikes"
	default:
		return nil, fmt.Errorf("invalid direction: %q", direction)
	}

	res, err := s.db.Exec(fmt.Sprintf("UPDATE vibes SET %s = %s + 1 WHERE id = ?", col, col), id)
	if err != nil {
		return nil, fmt.Errorf("update vote: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return nil, ErrNotFound
	}

	var v model.Vibe
	var createdAtStr string
	err = s.db.QueryRow(
		"SELECT id, youtube_url, video_title, thumbnail_url, thought, likes, dislikes, (likes - dislikes) AS net_score, created_at FROM vibes WHERE id = ?",
		id,
	).Scan(&v.ID, &v.YouTubeURL, &v.VideoTitle, &v.ThumbnailURL, &v.Thought, &v.Likes, &v.Dislikes, &v.NetScore, &createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("query updated vibe: %w", err)
	}

	createdAt, err := parseTimestamp(createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	v.CreatedAt = createdAt

	return &v, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// parseTimestamp attempts to parse a SQLite timestamp string in common formats.
func parseTimestamp(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("parsing time %q: unrecognized format", s)
}
