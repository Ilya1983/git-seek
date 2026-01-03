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

// Configuration constants
const (
	DefaultBatchSize      = 100  // Batch size for embedding generation
	DefaultSearchLimit    = 10   // Default max search results
	DebugSampleLimit      = 5    // Commits shown in debug mode
	ProgressBarWidth      = 40   // Width of progress bar display
	MaxFilesDisplay       = 3    // Max files shown before truncation
	HoursPerDay           = 24
	ApproxDaysPerMonth    = 30
	ApproxDaysPerYear     = 365
	NumberFormatThreshold = 1000
	CommaGroupSize        = 3
)

// Version information (set via SetVersionInfo from main)
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// SetVersionInfo sets version information from main package
func SetVersionInfo(v, c, d string) {
	version = v
	commit = c
	date = d
}

var debugFlag bool
var indexFlag bool
var statusFlag bool
var versionFlag bool
var batchSize int

// Search flags
var limitFlag int
var jsonFlag bool
var shortFlag bool

// Filter flags
var authorFilter string
var sinceFilter string
var untilFilter string
var pathFilter string
var branchFilter string

// Indexing flags
var skipFilesFlag bool

// Display customization flags
var maxFilesFlag int
var hashLengthFlag int
var debugLimitFlag int
var progressWidthFlag int

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
		// Validate display customization flags
		if hashLengthFlag < 7 || hashLengthFlag > 40 {
			fmt.Fprintln(os.Stderr, "Error: --hash-length must be between 7 and 40")
			os.Exit(1)
		}
		if maxFilesFlag < 0 {
			fmt.Fprintln(os.Stderr, "Error: --max-files must be non-negative")
			os.Exit(1)
		}
		if debugLimitFlag < 1 {
			fmt.Fprintln(os.Stderr, "Error: --debug-limit must be at least 1")
			os.Exit(1)
		}
		if progressWidthFlag < 10 {
			fmt.Fprintln(os.Stderr, "Error: --progress-width must be at least 10")
			os.Exit(1)
		}

		// Apply hash length setting
		git.SetShortHashLength(hashLengthFlag)

		if versionFlag {
			fmt.Printf("git-seek %s (commit: %s, built: %s)\n", version, commit, date)
			return
		}

		if debugFlag {
			git.Debug = true
			if err := runDebug(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		if embedTestFlag {
			if err := runEmbedTest(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		if storeTestFlag {
			if err := runStoreTest(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
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
	rootCmd.Flags().BoolVar(&versionFlag, "version", false, "Show version information")
	rootCmd.Flags().BoolVar(&debugFlag, "debug", false, "Debug mode: show commit extraction info")
	rootCmd.Flags().BoolVar(&indexFlag, "index", false, "Build/update semantic index")
	rootCmd.Flags().BoolVar(&statusFlag, "status", false, "Show index information")
	rootCmd.Flags().IntVar(&batchSize, "batch-size", DefaultBatchSize, "Batch size for embedding generation")
	rootCmd.Flags().BoolVar(&skipFilesFlag, "skip-files", false, "Skip file extraction during indexing (faster, but --path filter won't work)")

	// Search flags
	rootCmd.Flags().IntVarP(&limitFlag, "limit", "n", DefaultSearchLimit, "Maximum number of results")
	rootCmd.Flags().BoolVar(&jsonFlag, "json", false, "Output results as JSON")
	rootCmd.Flags().BoolVar(&shortFlag, "short", false, "Output only commit hashes")

	// Filter flags
	rootCmd.Flags().StringVar(&authorFilter, "author", "", "Filter by author name or email")
	rootCmd.Flags().StringVar(&sinceFilter, "since", "", "Show commits after date (YYYY-MM-DD)")
	rootCmd.Flags().StringVar(&untilFilter, "until", "", "Show commits before date (YYYY-MM-DD)")
	rootCmd.Flags().StringVar(&pathFilter, "path", "", "Filter by file path (glob pattern)")
	rootCmd.Flags().StringVar(&branchFilter, "branch", "", "Filter by branch name")

	// Display customization flags
	rootCmd.Flags().IntVar(&maxFilesFlag, "max-files", MaxFilesDisplay, "Maximum files shown per result")
	rootCmd.Flags().IntVar(&hashLengthFlag, "hash-length", git.ShortHashLength, "Commit hash display length (7-40)")
	rootCmd.Flags().IntVar(&debugLimitFlag, "debug-limit", DebugSampleLimit, "Number of commits shown in --debug mode")
	rootCmd.Flags().IntVar(&progressWidthFlag, "progress-width", ProgressBarWidth, "Progress bar width")
}

func runDebug() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	fmt.Printf("Opening repository at: %s\n", cwd)

	repo, err := git.OpenRepository(cwd)
	if err != nil {
		return fmt.Errorf("opening repository: %w", err)
	}

	fmt.Println("Fetching commits...")

	commits, err := git.GetCommits(repo, "", false) // Debug always gets files
	if err != nil {
		return fmt.Errorf("getting commits: %w", err)
	}

	fmt.Printf("\nFound %d commits\n\n", len(commits))

	// Show first few commits as sample
	limit := debugLimitFlag
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

	return nil
}

func runIndex() error {
	start := time.Now()

	ctx, err := newAppContext()
	if err != nil {
		return fmt.Errorf("initializing app context: %w", err)
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
		commits, err = git.GetCommits(repo, lastHash, skipFilesFlag)
		if err != nil {
			return fmt.Errorf("getting commits since %s: %w", lastHash, err)
		}
		if len(commits) == 0 {
			fmt.Println("Index is up to date")
			return nil
		}
		fmt.Printf("Found %d new commits to index\n", len(commits))
		if skipFilesFlag {
			fmt.Println("(skipping file extraction for faster indexing)")
		}
	} else {
		// Full index
		commits, err = git.GetCommits(repo, "", skipFilesFlag)
		if err != nil {
			return fmt.Errorf("getting commits: %w", err)
		}
		fmt.Printf("Indexing %d commits\n", len(commits))
		if skipFilesFlag {
			fmt.Println("(skipping file extraction for faster indexing)")
		}
	}

	// Progress bar
	bar := progressbar.NewOptions(len(commits),
		progressbar.OptionSetDescription("Embedding commits"),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(progressWidthFlag),
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

// parseDate parses a date string in common formats.
func parseDate(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date format: %s (use YYYY-MM-DD)", s)
}

func runSearch(query string) error {
	start := time.Now()

	ctx, err := newAppContext()
	if err != nil {
		return fmt.Errorf("initializing app context: %w", err)
	}
	defer ctx.Close()

	// Check if index exists
	count, err := ctx.store.Count()
	if err != nil || count == 0 {
		return fmt.Errorf("no index found. Run 'git-seek --index' first")
	}

	// Warn if --path filter is used (may not work with --skip-files index)
	if pathFilter != "" {
		fmt.Fprintln(os.Stderr, "Warning: --path filter may not work if index was built with --skip-files")
	}

	// Embed query
	queryVec, err := ctx.embedder.Embed(query)
	if err != nil {
		return fmt.Errorf("embedding query: %w", err)
	}

	// Build filters
	filters := store.SearchFilters{
		Author: authorFilter,
		Path:   pathFilter,
		Branch: branchFilter,
	}
	if sinceFilter != "" {
		filters.Since, err = parseDate(sinceFilter)
		if err != nil {
			return fmt.Errorf("parsing --since date: %w", err)
		}
	}
	if untilFilter != "" {
		filters.Until, err = parseDate(untilFilter)
		if err != nil {
			return fmt.Errorf("parsing --until date: %w", err)
		}
	}

	// Search
	results, err := ctx.store.Search(queryVec, limitFlag, filters)
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
		if len(files) > maxFilesFlag {
			files = append(files[:maxFilesFlag], fmt.Sprintf("(+%d more)", len(r.Commit.Files)-maxFilesFlag))
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
	case diff < HoursPerDay*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < ApproxDaysPerMonth*HoursPerDay*time.Hour:
		days := int(diff.Hours() / HoursPerDay)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case diff < ApproxDaysPerYear*HoursPerDay*time.Hour:
		months := int(diff.Hours() / HoursPerDay / ApproxDaysPerMonth)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(diff.Hours() / HoursPerDay / ApproxDaysPerYear)
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
	if n < NumberFormatThreshold {
		return fmt.Sprintf("%d", n)
	}

	// Format with commas
	s := fmt.Sprintf("%d", n)
	var result strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%CommaGroupSize == 0 {
			result.WriteRune(',')
		}
		result.WriteRune(c)
	}
	return result.String()
}
