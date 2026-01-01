package store

import "github.com/user/git-seek/internal/git"

// SearchResult represents a search match with similarity score.
type SearchResult struct {
	Commit git.Commit
	Score  float32 // Cosine similarity (0.0 - 1.0)
}

// Store defines the interface for commit+embedding storage.
type Store interface {
	// Save stores a commit with its embedding.
	Save(commit git.Commit, embedding []float32) error

	// SaveBatch stores multiple commits with embeddings efficiently.
	SaveBatch(commits []git.Commit, embeddings [][]float32) error

	// Search finds commits similar to the query vector, ordered by similarity.
	Search(queryVec []float32, limit int) ([]SearchResult, error)

	// GetLastIndexedCommit returns the most recent indexed commit hash.
	GetLastIndexedCommit() (string, error)

	// Count returns the number of indexed commits.
	Count() (int, error)

	// Close closes the store.
	Close() error
}
