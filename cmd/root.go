package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"github.com/user/git-seek/internal/embedding"
	"github.com/user/git-seek/internal/git"
	"github.com/user/git-seek/internal/store"
)

var debugFlag bool
var embedTestFlag bool
var storeTestFlag bool
var indexFlag bool
var statusFlag bool
var batchSize int

// Search flags
var limitFlag int
var jsonFlag bool
var shortFlag bool

// appContext holds initialized components for commands
type appContext struct {
	cwd      string
	store    *store.SQLiteStore
	embedder *embedding.MiniLMEmbedder
	closers  []func() error
}

// newAppContext initializes common components needed by commands
func newAppContext() (*appContext, error) {
	ctx := &appContext{}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting current directory: %w", err)
	}
	ctx.cwd = cwd

	s, err := store.NewSQLiteStore(cwd)
	if err != nil {
		return nil, fmt.Errorf("opening store: %w", err)
	}
	ctx.store = s
	ctx.closers = append(ctx.closers, s.Close)

	embedder, err := embedding.NewMiniLMEmbedder()
	if err != nil {
		ctx.Close()
		return nil, fmt.Errorf("creating embedder: %w", err)
	}
	ctx.embedder = embedder
	ctx.closers = append(ctx.closers, embedder.Close)

	return ctx, nil
}

// Close cleans up all initialized resources
func (ctx *appContext) Close() error {
	for _, closer := range ctx.closers {
		closer()
	}
	return nil
}

var rootCmd = &cobra.Command{
	Use:   "git-seek [query]",
	Short: "Semantic search for git commits",
	Long: `git-seek enables semantic search for git commits.
Find commits by meaning, not just keywords.

Examples:
  git-seek "error handling in API"
  git-seek "authentication changes" --author=alice
  git-seek --index`,
	Run: func(cmd *cobra.Command, args []string) {
		if debugFlag {
			git.Debug = true
			runDebug()
			return
		}

		if embedTestFlag {
			runEmbedTest()
			return
		}

		if storeTestFlag {
			runStoreTest()
			return
		}

		if indexFlag {
			if err := runIndex(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		if statusFlag {
			if err := runStatus(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		if len(args) == 0 {
			cmd.Help()
			return
		}

		// Run search
		if err := runSearch(args[0]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolVar(&debugFlag, "debug", false, "Debug mode: show commit extraction info")
	rootCmd.Flags().BoolVar(&embedTestFlag, "embed-test", false, "Test embedding generation")
	rootCmd.Flags().BoolVar(&storeTestFlag, "store-test", false, "Test vector storage and search")
	rootCmd.Flags().BoolVar(&indexFlag, "index", false, "Build/update semantic index")
	rootCmd.Flags().BoolVar(&statusFlag, "status", false, "Show index information")
	rootCmd.Flags().IntVar(&batchSize, "batch-size", 100, "Batch size for embedding generation")

	// Search flags
	rootCmd.Flags().IntVarP(&limitFlag, "limit", "n", 10, "Maximum number of results")
	rootCmd.Flags().BoolVar(&jsonFlag, "json", false, "Output results as JSON")
	rootCmd.Flags().BoolVar(&shortFlag, "short", false, "Output only commit hashes")
}

func runDebug() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Opening repository at: %s\n", cwd)

	repo, err := git.OpenRepository(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening repository: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Fetching commits...")

	commits, err := git.GetAllCommits(repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting commits: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nFound %d commits\n\n", len(commits))

	// Show first 5 commits as sample
	limit := 5
	if len(commits) < limit {
		limit = len(commits)
	}

	for i := 0; i < limit; i++ {
		c := commits[i]
		fmt.Printf("─────────────────────────────────────────\n")
		fmt.Printf("Hash:     %s\n", c.ShortHash())
		fmt.Printf("Author:   %s <%s>\n", c.Author, c.AuthorEmail)
		fmt.Printf("Date:     %s\n", c.Date.Format("2006-01-02 15:04:05"))
		fmt.Printf("Message:  %s\n", c.Message)
		if len(c.Files) > 0 {
			fmt.Printf("Files:    %v\n", c.Files)
		}
		if len(c.Branches) > 0 {
			fmt.Printf("Branches: %v\n", c.Branches)
		}
		fmt.Println()
	}

	if len(commits) > limit {
		fmt.Printf("... and %d more commits\n", len(commits)-limit)
	}
}

func runEmbedTest() {
	fmt.Println("Initializing MiniLM embedder...")

	embedder, err := embedding.NewMiniLMEmbedder()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating embedder: %v\n", err)
		os.Exit(1)
	}
	defer embedder.Close()

	fmt.Printf("Model loaded (dimensions: %d)\n\n", embedder.Dimensions())

	// Test with sample texts
	samples := []string{
		"Add authentication middleware",
		"Fix login bug in user session",
		"Update README documentation",
		"Refactor database connection pool",
	}

	fmt.Println("Generating embeddings for sample texts:")
	for _, text := range samples {
		vec, err := embedder.Embed(text)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error embedding text: %v\n", err)
			continue
		}
		fmt.Printf("\n  \"%s\"\n", text)
		fmt.Printf("  Vector[0:5]: [%.4f, %.4f, %.4f, %.4f, %.4f]\n",
			vec[0], vec[1], vec[2], vec[3], vec[4])
	}

	// Test similarity between related texts
	fmt.Println("\n─────────────────────────────────────────")
	fmt.Println("Testing similarity (related vs unrelated):")

	vec1, _ := embedder.Embed("Fix authentication bug")
	vec2, _ := embedder.Embed("Fix login issue")
	vec3, _ := embedder.Embed("Update documentation")

	sim12 := store.CosineSimilarity(vec1, vec2)
	sim13 := store.CosineSimilarity(vec1, vec3)

	fmt.Printf("\n  \"Fix authentication bug\" vs \"Fix login issue\": %.4f\n", sim12)
	fmt.Printf("  \"Fix authentication bug\" vs \"Update documentation\": %.4f\n", sim13)
}

func runStoreTest() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Initializing store...")
	s, err := store.NewSQLiteStore(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating store: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	fmt.Println("Initializing embedder...")
	embedder, err := embedding.NewMiniLMEmbedder()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating embedder: %v\n", err)
		os.Exit(1)
	}
	defer embedder.Close()

	// Get commits from repo
	fmt.Println("Fetching commits...")
	repo, err := git.OpenRepository(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening repository: %v\n", err)
		os.Exit(1)
	}

	commits, err := git.GetAllCommits(repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting commits: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d commits\n", len(commits))

	// Generate embeddings
	fmt.Println("Generating embeddings...")
	messages := make([]string, len(commits))
	for i, c := range commits {
		messages[i] = c.Message
	}

	embeddings, err := embedder.EmbedBatch(messages)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating embeddings: %v\n", err)
		os.Exit(1)
	}

	// Save to store
	fmt.Println("Saving to store...")
	err = s.SaveBatch(commits, embeddings)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error saving to store: %v\n", err)
		os.Exit(1)
	}

	count, _ := s.Count()
	fmt.Printf("Stored %d commits\n\n", count)

	// Test search
	fmt.Println("─────────────────────────────────────────")
	fmt.Println("Testing search...")

	query := "initial setup"
	fmt.Printf("Query: \"%s\"\n\n", query)

	queryVec, err := embedder.Embed(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error embedding query: %v\n", err)
		os.Exit(1)
	}

	results, err := s.Search(queryVec, 5)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error searching: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d results:\n\n", len(results))
	for _, r := range results {
		fmt.Printf("  %.4f  %s  %s\n", r.Score, r.Commit.ShortHash(), r.Commit.Message)
	}
}

func runIndex() error {
	start := time.Now()

	ctx, err := newAppContext()
	if err != nil {
		return err
	}
	defer ctx.Close()

	// Open repository (only needed for indexing)
	repo, err := git.OpenRepository(ctx.cwd)
	if err != nil {
		return fmt.Errorf("opening repository: %w", err)
	}

	// Check for existing index
	var commits []git.Commit
	lastHash, err := ctx.store.GetLastIndexedCommit()

	if err == nil && lastHash != "" {
		// Incremental: only new commits
		commits, err = git.GetCommitsSince(repo, lastHash)
		if err != nil {
			return fmt.Errorf("getting commits since %s: %w", lastHash, err)
		}
		if len(commits) == 0 {
			fmt.Println("Index is up to date")
			return nil
		}
		fmt.Printf("Found %d new commits to index\n", len(commits))
	} else {
		// Full index
		commits, err = git.GetAllCommits(repo)
		if err != nil {
			return fmt.Errorf("getting commits: %w", err)
		}
		fmt.Printf("Indexing %d commits\n", len(commits))
	}

	// Progress bar
	bar := progressbar.NewOptions(len(commits),
		progressbar.OptionSetDescription("Embedding commits"),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(40),
		progressbar.OptionClearOnFinish(),
	)

	// Batch process
	for i := 0; i < len(commits); i += batchSize {
		end := i + batchSize
		if end > len(commits) {
			end = len(commits)
		}
		batch := commits[i:end]

		// Extract messages
		messages := make([]string, len(batch))
		for j, c := range batch {
			messages[j] = c.Message
		}

		// Embed batch
		embeddings, err := ctx.embedder.EmbedBatch(messages)
		if err != nil {
			return fmt.Errorf("embedding batch: %w", err)
		}

		// Save to store
		err = ctx.store.SaveBatch(batch, embeddings)
		if err != nil {
			return fmt.Errorf("saving batch: %w", err)
		}

		bar.Add(len(batch))
	}

	// Summary
	count, _ := ctx.store.Count()
	elapsed := time.Since(start)
	fmt.Printf("\nIndexed %d commits in %s\n", count, elapsed.Round(time.Millisecond))

	return nil
}

func runSearch(query string) error {
	start := time.Now()

	ctx, err := newAppContext()
	if err != nil {
		return err
	}
	defer ctx.Close()

	// Check if index exists
	count, err := ctx.store.Count()
	if err != nil || count == 0 {
		return fmt.Errorf("no index found. Run 'git-seek --index' first")
	}

	// Embed query
	queryVec, err := ctx.embedder.Embed(query)
	if err != nil {
		return fmt.Errorf("embedding query: %w", err)
	}

	// Search
	results, err := ctx.store.Search(queryVec, limitFlag)
	if err != nil {
		return fmt.Errorf("searching: %w", err)
	}

	elapsed := time.Since(start)

	// Output based on format
	if shortFlag {
		for _, r := range results {
			fmt.Println(r.Commit.ShortHash())
		}
		return nil
	}

	if jsonFlag {
		return outputJSON(results)
	}

	// Human-readable output
	fmt.Printf("Found %d commits (%s)\n\n", len(results), elapsed.Round(time.Millisecond))
	for _, r := range results {
		printResult(r)
	}

	return nil
}

// JSONResult is the structure for JSON output
type JSONResult struct {
	Hash    string   `json:"hash"`
	Score   float32  `json:"score"`
	Message string   `json:"message"`
	Author  string   `json:"author"`
	Date    string   `json:"date"`
	Files   []string `json:"files"`
}

func outputJSON(results []store.SearchResult) error {
	output := make([]JSONResult, len(results))
	for i, r := range results {
		output[i] = JSONResult{
			Hash:    r.Commit.ShortHash(),
			Score:   r.Score,
			Message: r.Commit.Message,
			Author:  r.Commit.AuthorEmail,
			Date:    r.Commit.Date.Format(time.RFC3339),
			Files:   r.Commit.Files,
		}
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func printResult(r store.SearchResult) {
	// Line 1: hash  score  message
	fmt.Printf("%s  %.2f  %s\n", r.Commit.ShortHash(), r.Score, r.Commit.Message)

	// Line 2: relative time · author
	fmt.Printf("         %s · %s\n", relativeTime(r.Commit.Date), r.Commit.AuthorEmail)

	// Line 3: files (truncated if many)
	if len(r.Commit.Files) > 0 {
		files := r.Commit.Files
		if len(files) > 3 {
			files = append(files[:3], fmt.Sprintf("(+%d more)", len(r.Commit.Files)-3))
		}
		fmt.Printf("         %s\n", strings.Join(files, ", "))
	}

	fmt.Println()
}

func relativeTime(t time.Time) string {
	diff := time.Since(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 30*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case diff < 365*24*time.Hour:
		months := int(diff.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(diff.Hours() / 24 / 365)
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}

func runStatus() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	indexPath := filepath.Join(cwd, ".git", "semantic-index")
	dbPath := filepath.Join(indexPath, "embeddings.db")

	// Check if index exists
	info, err := os.Stat(dbPath)
	if os.IsNotExist(err) {
		fmt.Println("No index found. Run 'git-seek --index' to create one.")
		return nil
	}
	if err != nil {
		return fmt.Errorf("checking index: %w", err)
	}

	// Open store for count
	s, err := store.NewSQLiteStore(cwd)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	defer s.Close()

	count, _ := s.Count()

	// Output
	fmt.Printf("Index: %s/\n", indexPath)
	fmt.Printf("Commits indexed: %s\n", formatNumber(count))
	fmt.Printf("Last updated: %s\n", relativeTime(info.ModTime()))
	fmt.Printf("Index size: %s\n", formatSize(info.Size()))
	fmt.Println("Model: all-MiniLM-L6-v2 (384 dimensions)")

	return nil
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}

func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	// Format with commas
	s := fmt.Sprintf("%d", n)
	var result strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result.WriteRune(',')
		}
		result.WriteRune(c)
	}
	return result.String()
}
