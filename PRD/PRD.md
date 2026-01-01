# PRD: Semantic Git Search CLI

## Product Name
`git-seek` (working title — short, memorable, verb-like)

---

## Problem Statement

Developers frequently need to find commits related to a concept, but can't remember the exact words used in commit messages.

**Git's native search is limited:**
- `git log --grep="auth"` misses commits that say "login", "session", "JWT", "credentials"
- Combining multiple greps with regex is tedious and still misses synonyms
- No way to search by *meaning*, only by exact text

**The workaround today:**
- Run multiple greps with different keywords
- Scroll through hundreds of commits manually
- Ask a colleague who might remember (if they haven't left)

---

## Target Users

| User | Situation | What they need |
|------|-----------|----------------|
| Developer debugging | "When did we change how tokens refresh?" | Find related commits without knowing exact terms |
| Developer onboarding | "What commits touched the billing system?" | Explore unfamiliar codebase by concept |
| Tech lead reviewing | "Show me security-related changes this quarter" | Semantic search + date filtering |
| Anyone doing git archaeology | "Did we ever have retry logic here?" | Find historical context |

---

## Core Value Proposition

**Semantic understanding + filtering power.**

Find commits by *meaning*, not just keywords. Combine semantic search with the filters you expect: author, date, path, branch.

```bash
git-seek "payment processing" --author=alice --since="2024-01-01"
```

---

## What This Tool IS and IS NOT

### IS:
- Semantic search for finding *related* commits
- A way to bridge synonyms ("auth" finds "login", "session", "JWT")
- Combinable with traditional filters (author, date, path)
- Fast, offline, single-binary CLI

### IS NOT:
- Documentation or code explanation ("how does billing work" won't get good results)
- A replacement for good commit messages
- An AI chat interface (no LLM reasoning, just embedding similarity)

---

## Feature Requirements

### 1. Indexing

**Command:**
```bash
git-seek --index
```

**Behavior:**
- Walks all commits in the repository
- Extracts commit message + optionally changed file paths
- Generates embeddings using local MiniLM model
- Stores embeddings + metadata in `.git/semantic-index/`
- Shows progress bar for large repos

**Performance target:**
- 10K commits in <60 seconds
- Index size: ~10-20 MB for 10K commits

**Incremental updates:**
- On subsequent runs, only embed new commits since last index
- `--reindex` flag to force full rebuild

---

### 2. Semantic Search

**Command:**
```bash
git-seek "error handling in API layer"
```

**Output:**
```
Found 5 commits (search took 45ms)

abc123f  0.82  Add retry logic for failed API calls
         2 months ago · alice@company.com
         pkg/api/client.go, pkg/api/retry.go

def456a  0.76  Improve error messages in REST endpoints  
         3 months ago · bob@company.com
         pkg/api/handlers.go

...
```

**Output fields:**
- Commit hash (short)
- Similarity score (0.00 - 1.00)
- Commit message (first line)
- Relative date
- Author
- Changed files (abbreviated if many)

---

### 3. Filters

All filters narrow results *after* semantic matching:

| Filter | Flag | Example |
|--------|------|---------|
| Author | `--author` | `--author=alice` or `--author="alice@company.com"` |
| Since | `--since` | `--since="2024-01-01"` or `--since="3 months ago"` |
| Until | `--until` | `--until="2024-06-01"` |
| Path | `--path` | `--path="pkg/api/*"` (glob supported) |
| Branch | `--branch` | `--branch=main` |

**Combining filters:**
```bash
git-seek "database optimization" --author=alice --since="2024-01-01" --path="pkg/db/*"
```

---

### 4. Output Formats

| Format | Flag | Use case |
|--------|------|----------|
| Human (default) | (none) | Terminal inspection |
| Short | `--short` | Just hashes, one per line, for piping |
| JSON | `--json` | Scripting, further processing |

**Short format (for piping):**
```bash
git-seek "auth changes" --short | xargs git show
```

**JSON format:**
```json
[
  {
    "hash": "abc123f",
    "message": "Add retry logic for failed API calls",
    "author": "alice@company.com",
    "date": "2024-10-15T14:32:00Z",
    "files": ["pkg/api/client.go", "pkg/api/retry.go"],
    "score": 0.82
  }
]
```

---

### 5. Status / Info

**Command:**
```bash
git-seek --status
```

**Output:**
```
Index: .git/semantic-index/
Commits indexed: 8,432
Last updated: 2 hours ago
Index size: 12 MB
Model: all-MiniLM-L6-v2 (384 dimensions)
```

---

## CLI Interface Summary

```bash
git-seek [query]              # search (auto-indexes if needed)
git-seek --index              # build/update index
git-seek --reindex            # force full rebuild
git-seek --status             # show index info

# Filters
--author=<pattern>            # filter by author
--since=<date>                # commits after date
--until=<date>                # commits before date  
--path=<glob>                 # filter by file path
--branch=<name>               # filter by branch

# Output control
-n, --limit=<N>               # max results (default: 10)
--short                       # output hashes only
--json                        # output as JSON
--no-color                    # disable colored output

# Other
--help                        # show help
--version                     # show version
```

---

## Technical Architecture

### Components

```
┌─────────────────────────────────────────────────────┐
│                     CLI Layer                        │
│  (cobra: argument parsing, flags, help)             │
└─────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────┐
│                   Search Engine                      │
│  - Query embedding                                  │
│  - Cosine similarity                                │
│  - Filter application                               │
│  - Result ranking                                   │
└─────────────────────────────────────────────────────┘
                          │
          ┌───────────────┴───────────────┐
          ▼                               ▼
┌──────────────────┐           ┌──────────────────────┐
│   Git Interface  │           │   Embedding Store    │
│  (go-git)        │           │  (chromem-go or      │
│  - Walk commits  │           │   custom + SQLite)   │
│  - Read metadata │           │  - Store vectors     │
│  - Filter by     │           │  - Similarity search │
│    path/branch   │           │  - Persistence       │
└──────────────────┘           └──────────────────────┘
                                         │
                                         ▼
                               ┌──────────────────────┐
                               │   Embedding Model    │
                               │  (all-MiniLM-L6-v2   │
                               │   via ONNX Runtime)  │
                               └──────────────────────┘
```

### Key Dependencies

| Component | Library | Why |
|-----------|---------|-----|
| CLI | `spf13/cobra` | Standard for Go CLIs |
| Git | `go-git/go-git` | Pure Go, no git binary needed |
| Embeddings | `clems4ever/all-minilm-l6-v2-go` or `kelindar/search` | Local inference, no API |
| Vector store | `philippgille/chromem-go` or SQLite + custom | Embedded, no external DB |
| Progress | `schollz/progressbar` | User feedback during indexing |

### Storage Format

Location: `.git/semantic-index/`

```
.git/semantic-index/
├── meta.json          # index metadata (commit count, model, last update)
├── embeddings.db      # SQLite with vectors + commit refs
└── model/             # (optional) bundled model weights
```

**meta.json:**
```json
{
  "version": 1,
  "model": "all-MiniLM-L6-v2",
  "dimensions": 384,
  "commit_count": 8432,
  "last_commit": "abc123f",
  "last_updated": "2024-12-31T10:00:00Z"
}
```

---

## Performance Requirements

| Operation | Target | Notes |
|-----------|--------|-------|
| Index 10K commits | <60s | Batched embedding, all cores |
| Incremental update (100 new commits) | <5s | Only embed new |
| Search query | <100ms | Vector similarity is fast |
| Binary size | <50MB | Including model weights |
| Index size | ~1-2 KB per commit | 10K commits ≈ 10-20 MB |

---

## Build & Distribution

### Makefile Targets

```makefile
build:              # build for current platform
build-all:          # cross-compile linux/darwin/windows (amd64, arm64)
test:               # run tests
lint:               # run linters
release:            # create release binaries with embedded model
```

### Static Linking

ONNX Runtime statically linked to produce single binary:
- `git-seek-linux-amd64`
- `git-seek-linux-arm64`
- `git-seek-darwin-amd64`
- `git-seek-darwin-arm64`
- `git-seek-windows-amd64.exe`

### Installation Methods

```bash
# Direct download
curl -sSL https://github.com/user/git-seek/releases/latest/download/git-seek-$(uname -s)-$(uname -m) -o /usr/local/bin/git-seek

# Homebrew (future)
brew install git-seek

# Go install (if ONNX runtime available)
go install github.com/user/git-seek@latest
```

---

## Non-Goals (Explicit Scope Boundaries)

| Will NOT do | Reason |
|-------------|--------|
| Embed code diffs | Dramatically increases complexity and index size; commit messages are the MVP |
| LLM-powered explanations | Out of scope; this is search, not chat |
| Cloud/API option | Focus is offline-first single binary |
| GitHub/GitLab integration | Pure git, works anywhere |
| Real-time indexing hooks | Manual index update is fine for MVP |

---

## Success Criteria

**For portfolio purposes:**
1. Single binary that works on Linux/macOS without setup
2. Indexes a 10K commit repo in under a minute
3. Returns semantically relevant results (demo on well-known OSS repo)
4. Clean, well-documented code showing Go architecture
5. README with clear examples, GIF demo, benchmarks

**Stretch goals:**
- VS Code extension using same core
- Homebrew formula
- >100 GitHub stars

---

## MVP Scope (Build Order)

### Phase 1: Core (Day 1-2)
- [ ] Git commit walking with go-git
- [ ] Embedding generation with local model
- [ ] Storage to SQLite
- [ ] Basic search (query → results)
- [ ] Human-readable output

### Phase 2: Filters (Day 3)
- [ ] Author filter
- [ ] Date range filters
- [ ] Path filter
- [ ] Result limit flag

### Phase 3: Polish (Day 4)
- [ ] Incremental indexing
- [ ] Progress bar
- [ ] JSON output
- [ ] Short output (hashes only)
- [ ] --status command

### Phase 4: Distribution (Day 5)
- [ ] Static linking / single binary
- [ ] GitHub Actions for releases
- [ ] README with examples
- [ ] Demo GIF

---

## Open Questions

1. **Embed commit message only, or message + file paths?**
   - Message only: simpler, smaller index
   - Message + paths: richer matching ("changes to auth" matches files in `pkg/auth/`)
   - **Recommendation:** Start with message only, add paths as flag later

2. **Auto-index on first search?**
   - Yes with prompt: "No index found. Index 8,432 commits? (~30s) [Y/n]"
   - Or require explicit `--index` first
   - **Recommendation:** Auto with prompt

3. **How to handle very short/useless commit messages?**
   - "fix", "wip", "update" will have poor embeddings
   - Options: skip them, flag them, or just let them rank low
   - **Recommendation:** Include but let them naturally rank low

---

## Competitive Positioning

| Tool | Approach | Your advantage |
|------|----------|----------------|
| `git log --grep` | Exact text only | Semantic matching |
| git-log-search | Python + deps | Single Go binary, no setup |
| Spelungit | MCP server only | Works as standalone CLI |
| Both Python tools | No filters | Semantic + author/date/path filters |

**One-liner:**
> "Semantic git search that installs in one command and combines natural language queries with the filters you already know."