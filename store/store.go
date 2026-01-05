package store

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS published (
			guid TEXT PRIMARY KEY,
			published_at INTEGER NOT NULL
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_published_at ON published(published_at)`)
	if err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) IsPublished(guid string) bool {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM published WHERE guid = ?", guid).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

func (s *Store) MarkPublished(guid string, timestamp int64) error {
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO published (guid, published_at) VALUES (?, ?)",
		guid, timestamp,
	)
	return err
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Cleanup(olderThan int64) (int64, error) {
	result, err := s.db.Exec("DELETE FROM published WHERE published_at < ?", olderThan)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
