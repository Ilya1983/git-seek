package store

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/user/git-seek/internal/git"
	_ "modernc.org/sqlite"
)

const (
	indexDir = ".git/semantic-index"
	dbFile   = "embeddings.db"
)

// SQLiteStore implements Store using SQLite for persistence.
type SQLiteStore struct {
	db   *sql.DB
	path string
}

// NewSQLiteStore creates a new SQLite-based store.
// repoPath should be the root of the git repository.
func NewSQLiteStore(repoPath string) (*SQLiteStore, error) {
	indexPath := filepath.Join(repoPath, indexDir)
	if err := os.MkdirAll(indexPath, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(indexPath, dbFile)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	store := &SQLiteStore{db: db, path: dbPath}
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SQLiteStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS commits (
		hash         TEXT PRIMARY KEY,
		message      TEXT NOT NULL,
		author       TEXT NOT NULL,
		author_email TEXT NOT NULL,
		date         INTEGER NOT NULL,
		files        TEXT,
		branches     TEXT,
		embedding    BLOB NOT NULL,
		created_at   INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_commits_date ON commits(date);
	`
	_, err := s.db.Exec(schema)
	return err
}

// Save stores a single commit with its embedding.
func (s *SQLiteStore) Save(commit git.Commit, embedding []float32) error {
	return s.SaveBatch([]git.Commit{commit}, [][]float32{embedding})
}

// SaveBatch stores multiple commits with embeddings efficiently.
func (s *SQLiteStore) SaveBatch(commits []git.Commit, embeddings [][]float32) error {
	if len(commits) != len(embeddings) {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO commits
		(hash, message, author, author_email, date, files, branches, embedding, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for i, commit := range commits {
		filesJSON, _ := json.Marshal(commit.Files)
		branchesJSON, _ := json.Marshal(commit.Branches)
		embeddingBytes := embeddingToBytes(embeddings[i])

		_, err := stmt.Exec(
			commit.Hash,
			commit.Message,
			commit.Author,
			commit.AuthorEmail,
			commit.Date.Unix(),
			string(filesJSON),
			string(branchesJSON),
			embeddingBytes,
			now,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Search finds commits similar to the query vector, ordered by similarity.
func (s *SQLiteStore) Search(queryVec []float32, limit int) ([]SearchResult, error) {
	rows, err := s.db.Query(`
		SELECT hash, message, author, author_email, date, files, branches, embedding
		FROM commits
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult

	for rows.Next() {
		var (
			hash, message, author, authorEmail string
			dateUnix                           int64
			filesJSON, branchesJSON            string
			embeddingBytes                     []byte
		)

		err := rows.Scan(&hash, &message, &author, &authorEmail, &dateUnix, &filesJSON, &branchesJSON, &embeddingBytes)
		if err != nil {
			continue
		}

		var files, branches []string
		json.Unmarshal([]byte(filesJSON), &files)
		json.Unmarshal([]byte(branchesJSON), &branches)

		embedding := bytesToEmbedding(embeddingBytes)
		score := cosineSimilarity(queryVec, embedding)

		results = append(results, SearchResult{
			Commit: git.Commit{
				Hash:        hash,
				Message:     message,
				Author:      author,
				AuthorEmail: authorEmail,
				Date:        time.Unix(dateUnix, 0),
				Files:       files,
				Branches:    branches,
			},
			Score: score,
		})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Limit results
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// GetLastIndexedCommit returns the most recently indexed commit hash.
func (s *SQLiteStore) GetLastIndexedCommit() (string, error) {
	var hash string
	err := s.db.QueryRow(`
		SELECT hash FROM commits ORDER BY created_at DESC, date DESC LIMIT 1
	`).Scan(&hash)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return hash, err
}

// Count returns the number of indexed commits.
func (s *SQLiteStore) Count() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM commits`).Scan(&count)
	return count, err
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
