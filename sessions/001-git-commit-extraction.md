# Session 001: Git Commit Extraction

**Date:** 2026-01-01

## Summary

Built the foundation for git-seek: a CLI that extracts git commits with full metadata using go-git.

---

## What Was Built

### Project Structure

```
git-seek/
├── go.mod                      # Go module (github.com/user/git-seek)
├── go.sum                      # Dependency checksums
├── main.go                     # Entry point
├── cmd/
│   └── root.go                 # Cobra CLI setup
└── internal/
    └── git/
        └── commits.go          # Commit extraction logic
```

### Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/spf13/cobra` | v1.8.1 | CLI framework |
| `github.com/go-git/go-git/v5` | v5.12.0 | Pure Go git implementation |

### Commit Data Structure

```go
type Commit struct {
    Hash        string    // Short hash (7 chars)
    Message     string    // First line of commit message
    Author      string    // Author name
    AuthorEmail string    // Author email
    Date        time.Time // Commit date
    Files       []string  // Changed files in this commit
    Branches    []string  // Branches containing this commit
}
```

### Functions Implemented

| Function | Location | Purpose |
|----------|----------|---------|
| `OpenRepository(path)` | `internal/git/commits.go` | Opens a git repository |
| `GetAllCommits(repo)` | `internal/git/commits.go` | Walks all commits, extracts metadata |
| `getCommitFiles(commit)` | `internal/git/commits.go` | Gets changed files via tree diff |
| `buildBranchMap(repo)` | `internal/git/commits.go` | Maps commit hashes to branch names |
| `firstLine(s)` | `internal/git/commits.go` | Extracts first line of commit message |

### CLI Commands

```bash
./git-seek --debug    # Test mode: shows commit extraction results
./git-seek --help     # Show help
./git-seek "query"    # Placeholder for future search
```

---

## Verified Output

Running `./git-seek --debug` on this repository:

```
Opening repository at: /workspace
Fetching commits...

Found 1 commits

─────────────────────────────────────────
Hash:     9342359
Author:   Ilya1983 <47712289+Ilya1983@users.noreply.github.com>
Date:     2026-01-01 13:45:51
Message:  Initial commit
Files:    [.gitignore LICENSE]
Branches: [main]
```

---

## Next Steps

### Phase 2: Embedding Generation

1. **Add embedding model** - Integrate `all-MiniLM-L6-v2` via ONNX Runtime
   - Create `internal/embedding/` package
   - Function: `GenerateEmbedding(text string) ([]float32, error)`
   - Batch processing for efficiency

2. **Decide what to embed**
   - Option A: Commit message only (simpler, smaller index)
   - Option B: Message + file paths (richer matching)

### Phase 3: Vector Storage

1. **Set up SQLite storage** - Create `internal/store/` package
   - Store embeddings with commit metadata
   - Location: `.git/semantic-index/embeddings.db`
   - Create `meta.json` for index metadata

2. **Implement storage functions**
   - `SaveCommit(commit, embedding)`
   - `GetAllEmbeddings()`
   - `GetLastIndexedCommit()`

### Phase 4: Search Implementation

1. **Cosine similarity search**
   - Embed query text
   - Compare against stored embeddings
   - Rank by similarity score

2. **Wire up CLI**
   - `git-seek "query"` performs search
   - `git-seek --index` builds/updates index
   - `git-seek --status` shows index info

### Phase 5: Filters & Polish

1. **Add filters** - `--author`, `--since`, `--until`, `--path`
2. **Output formats** - `--json`, `--short`
3. **Progress bar** - For indexing large repos
4. **Incremental indexing** - Only embed new commits

---

## Technical Decisions Made

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Git library | go-git | Pure Go, no external git binary needed |
| CLI framework | Cobra | Standard for Go CLIs, good UX |
| Hash format | 7-char short hash | Matches git's default short hash |
| Branch mapping | Pre-built map | Efficient lookup during commit walk |
| File diff | Tree comparison | Accurate list of changed files |

---

## Build & Run

```bash
# Build
go build -o git-seek .

# Run debug mode
./git-seek --debug

# Show help
./git-seek --help
```
