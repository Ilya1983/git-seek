package store

import (
	"time"

	"github.com/user/git-seek/internal/git"
)

// SearchFilters contains optional filters for search queries.
type SearchFilters struct {
	Author string    // Substring match on author name or email
	Since  time.Time // Commits after this date (zero = no filter)
	Until  time.Time // Commits before this date (zero = no filter)
	Path   string    // Glob pattern for file paths
	Branch string    // Exact match on branch name
}

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
	// Filters are applied to narrow results before similarity ranking.
	Search(queryVec []float32, limit int, filters SearchFilters) ([]SearchResult, error)

	// GetLastIndexedCommit returns the most recent indexed commit hash.
	GetLastIndexedCommit() (string, error)

	// Count returns the number of indexed commits.
	Count() (int, error)

	// Close closes the store.
	Close() error
}
