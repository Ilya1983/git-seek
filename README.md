# git-seek

**Semantic search for git commits.** Find commits by meaning, not just keywords.

## The Problem

Git's native search is limited to exact text matching:

```bash
git log --grep="auth"
```

This misses commits that say "login", "session", "JWT", or "credentials" - even though they're all related to authentication.

**git-seek** uses semantic search to find commits by *meaning*. Search for "authentication changes" and find all related commits, regardless of the exact words used.

## Features

- **Semantic Search** - Find commits by meaning, not keywords. "auth" matches "login", "session", "JWT"
- **Powerful Filters** - Narrow results by author, date range, file path, or branch
- **Multiple Output Formats** - Human-readable, JSON for scripting, or hashes for piping
- **Fast & Local** - All processing happens locally using embedded ML model. No API calls, no cloud
- **Single Binary** - One executable with embedded model. No Python, no dependencies to install

## Quick Start

```bash
# Index your repository (one-time, ~30s for 10K commits)
git-seek --index

# Search by meaning
git-seek "error handling in API"

# Filter results
git-seek "database optimization" --author=alice --since=2024-01-01
```

## Installation

### From Releases (Recommended)

Download the latest release for your platform from the [Releases page](https://github.com/Ilya1983/git-seek/releases).

**Linux (x64):**
```bash
# Replace v1.1.0 with the latest version
curl -LO https://github.com/Ilya1983/git-seek/releases/download/v1.1.0/git-seek-linux-x64-v1.1.0.tar.gz
tar -xzf git-seek-linux-x64-v1.1.0.tar.gz
cd git-seek-linux-x64-v1.1.0

# Set environment (add to ~/.bashrc for persistence)
export LD_LIBRARY_PATH=$(pwd)/lib:$LD_LIBRARY_PATH
export ONNXRUNTIME_LIB_PATH=$(pwd)/lib/libonnxruntime.so

./git-seek --help
```

**macOS (Apple Silicon):**
```bash
# Replace v1.1.0 with the latest version
curl -LO https://github.com/Ilya1983/git-seek/releases/download/v1.1.0/git-seek-osx-arm64-v1.1.0.tar.gz
tar -xzf git-seek-osx-arm64-v1.1.0.tar.gz
cd git-seek-osx-arm64-v1.1.0

# Set environment (add to ~/.zshrc for persistence)
export DYLD_LIBRARY_PATH=$(pwd)/lib:$DYLD_LIBRARY_PATH
export ONNXRUNTIME_LIB_PATH=$(pwd)/lib/libonnxruntime.dylib

./git-seek --help
```

**Windows (x64):**
```powershell
# Replace v1.1.0 with the latest version
Invoke-WebRequest -Uri "https://github.com/Ilya1983/git-seek/releases/download/v1.1.0/git-seek-win-x64-v1.1.0.zip" -OutFile "git-seek-win-x64-v1.1.0.zip"
Expand-Archive -Path "git-seek-win-x64-v1.1.0.zip" -DestinationPath "."
cd git-seek-win-x64-v1.1.0

# Set environment (for current session)
$env:PATH = "$(pwd)\lib;$env:PATH"
$env:ONNXRUNTIME_LIB_PATH = "$(pwd)\lib\onnxruntime.dll"

.\git-seek.exe --help
```

### Run from Any Directory (Optional)

If you want to use `git-seek` from any folder, you need to set up permanent environment variables with **absolute paths**.

**Linux:**
```bash
# Add to ~/.bashrc (adjust path to where you extracted git-seek)
export PATH="/path/to/git-seek-linux-x64-v1.1.0:$PATH"
export LD_LIBRARY_PATH="/path/to/git-seek-linux-x64-v1.1.0/lib:$LD_LIBRARY_PATH"
export ONNXRUNTIME_LIB_PATH="/path/to/git-seek-linux-x64-v1.1.0/lib/libonnxruntime.so"

# Reload shell config
source ~/.bashrc

# Now works from any directory
git-seek --help
```

**macOS:**
```bash
# Add to ~/.zshrc (adjust path to where you extracted git-seek)
export PATH="/path/to/git-seek-osx-arm64-v1.1.0:$PATH"
export DYLD_LIBRARY_PATH="/path/to/git-seek-osx-arm64-v1.1.0/lib:$DYLD_LIBRARY_PATH"
export ONNXRUNTIME_LIB_PATH="/path/to/git-seek-osx-arm64-v1.1.0/lib/libonnxruntime.dylib"

# Reload shell config
source ~/.zshrc

# Now works from any directory
git-seek --help
```

**Windows (PowerShell as Administrator):**
```powershell
# Set permanent environment variables (adjust path to where you extracted git-seek)
$installPath = "C:\path\to\git-seek-win-x64-v1.1.0"
[Environment]::SetEnvironmentVariable("Path", "$installPath;$installPath\lib;$env:Path", "Machine")
[Environment]::SetEnvironmentVariable("ONNXRUNTIME_LIB_PATH", "$installPath\lib\onnxruntime.dll", "Machine")

# Restart PowerShell, then run from any directory (without .\ prefix)
git-seek --help
```

> **Windows Note:** When running from PATH, use `git-seek` not `.\git-seek.exe`. The `.\` prefix only looks in the current directory.

### Build from Source

**Prerequisites:**
- Go 1.25 or later
- Make (Linux/macOS) or PowerShell (Windows)

**Linux/macOS:**
```bash
git clone https://github.com/Ilya1983/git-seek.git
cd git-seek
make build    # Downloads ONNX Runtime automatically

# Set environment
export LD_LIBRARY_PATH=$(pwd)/deps/onnxruntime/linux-x64/lib
export ONNXRUNTIME_LIB_PATH=$(pwd)/deps/onnxruntime/linux-x64/lib/libonnxruntime.so

./git-seek --help
```

**Windows:**
```powershell
git clone https://github.com/Ilya1983/git-seek.git
cd git-seek
.\build.ps1 -Build    # Downloads ONNX Runtime automatically

# Set environment
$env:PATH = "$(pwd)\deps\onnxruntime\win-x64\lib;$env:PATH"
$env:ONNXRUNTIME_LIB_PATH = "$(pwd)\deps\onnxruntime\win-x64\lib\onnxruntime.dll"

.\git-seek.exe --help
```

## Usage

### Indexing

Before searching, you need to index your repository:

```bash
git-seek --index
```

This walks through all commits, generates semantic embeddings for each commit message, and stores them locally. Progress is shown for large repositories.

**Fast indexing for large repositories:**

For repositories with 50K+ commits, use `--skip-files` to skip the expensive file extraction:

```bash
git-seek --index --skip-files
```

This is significantly faster but the `--path` filter won't work since file data isn't collected.

**Index location:** `.git/semantic-index/embeddings.db`

Since the index is stored inside the `.git/` directory, it's automatically excluded from version control. Each developer's clone has its own local index.

### Incremental Updates

When new commits are added to the repository, simply run `--index` again:

```bash
git-seek --index
```

This only processes commits added since the last index - no need to rebuild from scratch.

**To force a full reindex**, delete the index directory and run again:

```bash
rm -rf .git/semantic-index
git-seek --index
```

### Check Index Status

```bash
git-seek --status
```

Shows number of indexed commits, last update time, and index size.

### Searching

Basic semantic search:

```bash
git-seek "error handling"
git-seek "payment processing"
git-seek "user authentication flow"
```

**Example output:**

```
Found 5 commits (search took 42ms)

abc123f  0.82  Add retry logic for failed API calls
         2 months ago by alice@company.com
         pkg/api/client.go, pkg/api/retry.go

def456a  0.76  Improve error messages in REST endpoints
         3 months ago by bob@company.com
         pkg/api/handlers.go
```

### Filters

Narrow your search with filters:

```bash
# By author (name or email, substring match)
git-seek "refactoring" --author=alice
git-seek "bug fix" --author=@company.com

# By date range
git-seek "security" --since=2024-01-01
git-seek "feature" --until=2024-06-30
git-seek "changes" --since=2024-01-01 --until=2024-03-31

# By file path (glob pattern)
git-seek "API changes" --path="pkg/api/*.go"
git-seek "frontend" --path="src/components/*"

# By branch
git-seek "hotfix" --branch=main

# Combined filters
git-seek "database" --author=alice --since=2024-01-01 --path="pkg/db/*"
```

### Output Formats

**Human-readable (default):**
```bash
git-seek "authentication"
```

**JSON (for scripting):**
```bash
git-seek "authentication" --json
```

```json
[
  {
    "hash": "abc123f",
    "message": "Add JWT token refresh logic",
    "author": "alice@company.com",
    "date": "2024-10-15T14:32:00Z",
    "files": ["pkg/auth/jwt.go"],
    "score": 0.82
  }
]
```

**Short (hashes only, for piping):**
```bash
# Show commits
git-seek "bug fix" --short | xargs git show

# Cherry-pick commits
git-seek "feature" --short | xargs git cherry-pick
```

### Limit Results

```bash
git-seek "refactoring" -n 5    # Top 5 results
git-seek "changes" --limit=20  # Top 20 results
```

## Command Reference

| Flag | Description |
|------|-------------|
| `--version` | Show version information |
| `--index` | Build or update the semantic index |
| `--skip-files` | Skip file extraction during indexing (faster, but `--path` filter won't work) |
| `--status` | Show index information |
| `--author=<name>` | Filter by author name or email (substring) |
| `--since=<date>` | Show commits after date (YYYY-MM-DD) |
| `--until=<date>` | Show commits before date (YYYY-MM-DD) |
| `--path=<glob>` | Filter by file path (glob pattern) |
| `--branch=<name>` | Filter by branch name |
| `-n, --limit=<N>` | Maximum results (default: 10) |
| `--json` | Output as JSON |
| `--short` | Output hashes only |
| `--help` | Show help |

**Developer/Debug Flags:**

| Flag | Description |
|------|-------------|
| `--debug` | Show commit extraction info |
| `--embed-test` | Test embedding model initialization and similarity |
| `--store-test` | Test vector storage and search functionality |

## How It Works

1. **Indexing**: git-seek walks through all commits and generates a 384-dimensional embedding vector for each commit message using the [all-MiniLM-L6-v2](https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2) model.

2. **Storage**: Embeddings are stored in a local SQLite database at `.git/semantic-index/embeddings.db`.

3. **Search**: Your query is embedded using the same model, then compared against all stored embeddings using cosine similarity. The most similar commits are returned.

4. **Filtering**: SQL-based filters (author, date) are applied before similarity calculation. Path and branch filters are applied after ranking.

**Key points:**
- All processing is local - no API calls, no cloud services
- The model runs via ONNX Runtime for fast inference
- The model weights (~90MB) are downloaded at build time and embedded in the binary

## Project Structure

```
git-seek/
├── cmd/
│   ├── root.go           # CLI entry point and main commands
│   └── devtools.go       # Developer testing utilities
├── internal/
│   ├── git/              # Git operations (commit walking, metadata)
│   ├── embedding/        # Embedding generation interface
│   └── store/            # Vector storage and similarity search
├── patches/              # Patched dependencies (see below)
├── Makefile              # Build automation (Linux/macOS)
└── build.ps1             # Build automation (Windows)
```

### Why the `patches/` Directory Exists

The `patches/all-minilm-l6-v2-go/` directory contains a patched version of the [clems4ever/all-minilm-l6-v2-go](https://github.com/clems4ever/all-minilm-l6-v2-go) package. This was necessary because:

1. **API Incompatibility**: The upstream package calls `tokenizer.NewRawInputSequence()` which doesn't exist in the tokenizer library. The fix changes it to `tokenizer.NewInputSequence()`.

2. **Git LFS Issue**: The upstream package stores `model.onnx` via Git LFS. When Go downloads the module, it gets a 133-byte LFS pointer file instead of the actual 90MB model, causing "protobuf parsing failed" errors.

The patched package is referenced via a `replace` directive in `go.mod`:
```go
replace github.com/clems4ever/all-minilm-l6-v2-go => ./patches/all-minilm-l6-v2-go
```

The model file itself is **not** stored in git - it's downloaded during build with SHA256 checksum verification.

## Performance

| Operation | Target | Typical |
|-----------|--------|---------|
| Index 10K commits | < 60 seconds | ~30-45s |
| Search query | < 100ms | ~40-60ms |
| Index size | ~1-2 KB/commit | ~15MB for 10K |

## Troubleshooting

### "ONNX Runtime not found" or library errors

The ONNX Runtime shared library must be in your library path:

**Linux:**
```bash
export LD_LIBRARY_PATH=/path/to/lib:$LD_LIBRARY_PATH
export ONNXRUNTIME_LIB_PATH=/path/to/lib/libonnxruntime.so
```

**macOS:**
```bash
export DYLD_LIBRARY_PATH=/path/to/lib:$DYLD_LIBRARY_PATH
export ONNXRUNTIME_LIB_PATH=/path/to/lib/libonnxruntime.dylib
```

**Windows:**
```powershell
$env:PATH = "C:\path\to\lib;$env:PATH"
$env:ONNXRUNTIME_LIB_PATH = "C:\path\to\lib\onnxruntime.dll"
```

### "No index found"

Run `git-seek --index` first to build the index.

### Search returns no results

- Check that commits exist matching your filters with `git-seek --status`
- Try a broader query or remove some filters
- Ensure the index is up to date with `git-seek --index`

### Path filter not matching

The `--path` flag uses Go's `filepath.Match()` which does NOT support `**` recursive matching. Use explicit paths:

```bash
# Works
git-seek "changes" --path="pkg/api/*.go"
git-seek "changes" --path="internal/*/*.go"

# Does NOT work
git-seek "changes" --path="**/*.go"
```

### Path filter returns no results (indexed with --skip-files)

If you indexed with `--skip-files`, the `--path` filter won't work because file data wasn't collected. Re-index without the flag:

```bash
rm -rf .git/semantic-index
git-seek --index
```

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `make test`
5. Submit a pull request

Report bugs and feature requests at [GitHub Issues](https://github.com/Ilya1983/git-seek/issues).

## License

MIT License - see [LICENSE](LICENSE) for details.
