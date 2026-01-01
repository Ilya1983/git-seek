package cmd

import (
	"fmt"
	"math"
	"os"

	"github.com/spf13/cobra"
	"github.com/user/git-seek/internal/embedding"
	"github.com/user/git-seek/internal/git"
)

var debugFlag bool
var embedTestFlag bool

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
			runDebug()
			return
		}

		if embedTestFlag {
			runEmbedTest()
			return
		}

		if len(args) == 0 {
			cmd.Help()
			return
		}

		// TODO: Implement search
		fmt.Printf("Search query: %s\n", args[0])
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
		fmt.Printf("Hash:     %s\n", c.Hash)
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

	sim12 := cosineSimilarity(vec1, vec2)
	sim13 := cosineSimilarity(vec1, vec3)

	fmt.Printf("\n  \"Fix authentication bug\" vs \"Fix login issue\": %.4f\n", sim12)
	fmt.Printf("  \"Fix authentication bug\" vs \"Update documentation\": %.4f\n", sim13)
}

func cosineSimilarity(a, b []float32) float32 {
	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (sqrt(normA) * sqrt(normB))
}

func sqrt(x float32) float32 {
	return float32(math.Sqrt(float64(x)))
}
