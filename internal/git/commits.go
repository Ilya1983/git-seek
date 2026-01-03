package git

import (
	"fmt"
	"os"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

// Debug enables verbose logging for git operations.
var Debug bool

// ShortHashLength is the number of characters used for abbreviated commit hashes.
const ShortHashLength = 7

// ShortenHash returns the first ShortHashLength characters of a hash string.
func ShortenHash(hash string) string {
	if len(hash) >= ShortHashLength {
		return hash[:ShortHashLength]
	}
	return hash
}

// Commit represents a git commit with its metadata.
type Commit struct {
	Hash        string // Full 40-character hash
	Message     string
	Author      string
	AuthorEmail string
	Date        time.Time
	Files       []string
	Branches    []string
}

// ShortHash returns the abbreviated commit hash for display.
func (c Commit) ShortHash() string {
	return ShortenHash(c.Hash)
}

// OpenRepository opens a git repository at the given path.
func OpenRepository(path string) (*git.Repository, error) {
	return git.PlainOpen(path)
}

// GetCommits retrieves commits from the repository.
// If sinceHash is empty, returns all commits.
// If sinceHash is provided, returns only commits newer than that hash.
// Returns commits in reverse chronological order (newest first).
func GetCommits(repo *git.Repository, sinceHash string) ([]Commit, error) {
	// Build a map of commit hash -> branches for quick lookup
	branchMap, err := buildBranchMap(repo)
	if err != nil {
		return nil, fmt.Errorf("building branch map: %w", err)
	}

	// Get commit iterator starting from HEAD
	commitIter, err := repo.Log(&git.LogOptions{
		Order: git.LogOrderCommitterTime,
		All:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("reading git log: %w", err)
	}
	defer commitIter.Close()

	var commits []Commit

	err = commitIter.ForEach(func(c *object.Commit) error {
		hash := c.Hash.String()
		shortHash := ShortenHash(hash)

		// Stop when we reach the already-indexed commit (if sinceHash provided)
		if sinceHash != "" && (shortHash == sinceHash || hash == sinceHash) {
			return storer.ErrStop
		}

		files, err := getCommitFiles(c)
		if err != nil {
			if Debug {
				fmt.Fprintf(os.Stderr, "Warning: failed to get files for %s: %v\n", shortHash, err)
			}
			files = []string{}
		}

		commit := Commit{
			Hash:        hash, // Full hash
			Message:     firstLine(c.Message),
			Author:      c.Author.Name,
			AuthorEmail: c.Author.Email,
			Date:        c.Author.When,
			Files:       files,
			Branches:    branchMap[c.Hash],
		}

		commits = append(commits, commit)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("iterating commits: %w", err)
	}

	return commits, nil
}

// buildBranchMap creates a mapping from commit hash to branch names.
func buildBranchMap(repo *git.Repository) (map[plumbing.Hash][]string, error) {
	branchMap := make(map[plumbing.Hash][]string)

	// Get all branches
	branchIter, err := repo.Branches()
	if err != nil {
		return nil, fmt.Errorf("reading branches: %w", err)
	}
	defer branchIter.Close()

	err = branchIter.ForEach(func(ref *plumbing.Reference) error {
		branchName := ref.Name().Short()
		commitHash := ref.Hash()

		// Add this branch to the commit's branch list
		branchMap[commitHash] = append(branchMap[commitHash], branchName)

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("iterating branches: %w", err)
	}

	return branchMap, nil
}

// getCommitFiles returns the list of files changed in a commit.
func getCommitFiles(c *object.Commit) ([]string, error) {
	var files []string

	// Get the tree for this commit
	tree, err := c.Tree()
	if err != nil {
		return nil, fmt.Errorf("reading commit tree: %w", err)
	}

	// For the first commit, list all files in the tree
	if c.NumParents() == 0 {
		err = tree.Files().ForEach(func(f *object.File) error {
			files = append(files, f.Name)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("listing tree files: %w", err)
		}
		return files, nil
	}

	// Get parent commit for diff
	parent, err := c.Parent(0)
	if err != nil {
		return nil, fmt.Errorf("reading parent commit: %w", err)
	}

	parentTree, err := parent.Tree()
	if err != nil {
		return nil, fmt.Errorf("reading parent tree: %w", err)
	}

	// Get changes between parent and this commit
	changes, err := parentTree.Diff(tree)
	if err != nil {
		return nil, fmt.Errorf("computing tree diff: %w", err)
	}

	for _, change := range changes {
		// Get the file path (prefer "To" for additions/modifications, "From" for deletions)
		name := change.To.Name
		if name == "" {
			name = change.From.Name
		}
		if name != "" {
			files = append(files, name)
		}
	}

	return files, nil
}

// firstLine returns the first line of a string.
func firstLine(s string) string {
	for i, c := range s {
		if c == '\n' || c == '\r' {
			return s[:i]
		}
	}
	return s
}
