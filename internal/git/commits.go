package git

import (
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

// Commit represents a git commit with its metadata.
type Commit struct {
	Hash        string
	Message     string
	Author      string
	AuthorEmail string
	Date        time.Time
	Files       []string
	Branches    []string
}

// OpenRepository opens a git repository at the given path.
func OpenRepository(path string) (*git.Repository, error) {
	return git.PlainOpen(path)
}

// GetAllCommits retrieves all commits from the repository.
func GetAllCommits(repo *git.Repository) ([]Commit, error) {
	// Build a map of commit hash -> branches for quick lookup
	branchMap, err := buildBranchMap(repo)
	if err != nil {
		return nil, err
	}

	// Get commit iterator starting from HEAD
	commitIter, err := repo.Log(&git.LogOptions{
		Order: git.LogOrderCommitterTime,
		All:   true,
	})
	if err != nil {
		return nil, err
	}
	defer commitIter.Close()

	var commits []Commit

	err = commitIter.ForEach(func(c *object.Commit) error {
		files, err := getCommitFiles(c)
		if err != nil {
			// Don't fail on file extraction errors, just skip files
			files = []string{}
		}

		hash := c.Hash.String()
		commit := Commit{
			Hash:        hash[:7], // Short hash
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
		return nil, err
	}

	return commits, nil
}

// GetCommitsSince retrieves commits newer than the given hash.
// Returns commits in reverse chronological order (newest first).
// If sinceHash is not found, returns all commits.
func GetCommitsSince(repo *git.Repository, sinceHash string) ([]Commit, error) {
	// Build a map of commit hash -> branches for quick lookup
	branchMap, err := buildBranchMap(repo)
	if err != nil {
		return nil, err
	}

	// Get commit iterator
	commitIter, err := repo.Log(&git.LogOptions{
		Order: git.LogOrderCommitterTime,
		All:   true,
	})
	if err != nil {
		return nil, err
	}
	defer commitIter.Close()

	var commits []Commit

	err = commitIter.ForEach(func(c *object.Commit) error {
		hash := c.Hash.String()
		shortHash := hash[:7]

		// Stop when we reach the already-indexed commit
		if shortHash == sinceHash || hash == sinceHash {
			return storer.ErrStop
		}

		files, err := getCommitFiles(c)
		if err != nil {
			files = []string{}
		}

		commit := Commit{
			Hash:        shortHash,
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
		return nil, err
	}

	return commits, nil
}

// buildBranchMap creates a mapping from commit hash to branch names.
func buildBranchMap(repo *git.Repository) (map[plumbing.Hash][]string, error) {
	branchMap := make(map[plumbing.Hash][]string)

	// Get all branches
	branchIter, err := repo.Branches()
	if err != nil {
		return nil, err
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
		return nil, err
	}

	return branchMap, nil
}

// getCommitFiles returns the list of files changed in a commit.
func getCommitFiles(c *object.Commit) ([]string, error) {
	var files []string

	// Get the tree for this commit
	tree, err := c.Tree()
	if err != nil {
		return nil, err
	}

	// For the first commit, list all files in the tree
	if c.NumParents() == 0 {
		err = tree.Files().ForEach(func(f *object.File) error {
			files = append(files, f.Name)
			return nil
		})
		return files, err
	}

	// Get parent commit for diff
	parent, err := c.Parent(0)
	if err != nil {
		return nil, err
	}

	parentTree, err := parent.Tree()
	if err != nil {
		return nil, err
	}

	// Get changes between parent and this commit
	changes, err := parentTree.Diff(tree)
	if err != nil {
		return nil, err
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
