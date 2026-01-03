package cmd

import (
	"fmt"
	"os"

	"github.com/user/git-seek/internal/embedding"
	"github.com/user/git-seek/internal/git"
	"github.com/user/git-seek/internal/store"
)

// Developer/testing flags
var embedTestFlag bool
var storeTestFlag bool

func init() {
	rootCmd.Flags().BoolVar(&embedTestFlag, "embed-test", false, "Test embedding generation")
	rootCmd.Flags().BoolVar(&storeTestFlag, "store-test", false, "Test vector storage and search")
}

func runEmbedTest() error {
	fmt.Println("Initializing MiniLM embedder...")

	embedder, err := embedding.NewMiniLMEmbedder()
	if err != nil {
		return fmt.Errorf("creating embedder: %w", err)
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

	return nil
}

func runStoreTest() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	fmt.Println("Initializing store...")
	s, err := store.NewSQLiteStore(cwd)
	if err != nil {
		return fmt.Errorf("creating store: %w", err)
	}
	defer s.Close()

	fmt.Println("Initializing embedder...")
	embedder, err := embedding.NewMiniLMEmbedder()
	if err != nil {
		return fmt.Errorf("creating embedder: %w", err)
	}
	defer embedder.Close()

	// Get commits from repo
	fmt.Println("Fetching commits...")
	repo, err := git.OpenRepository(cwd)
	if err != nil {
		return fmt.Errorf("opening repository: %w", err)
	}

	commits, err := git.GetCommits(repo, "", false) // Store test always gets files
	if err != nil {
		return fmt.Errorf("getting commits: %w", err)
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
		return fmt.Errorf("generating embeddings: %w", err)
	}

	// Save to store
	fmt.Println("Saving to store...")
	err = s.SaveBatch(commits, embeddings)
	if err != nil {
		return fmt.Errorf("saving to store: %w", err)
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
		return fmt.Errorf("embedding query: %w", err)
	}

	results, err := s.Search(queryVec, DebugSampleLimit, store.SearchFilters{})
	if err != nil {
		return fmt.Errorf("searching: %w", err)
	}

	fmt.Printf("Found %d results:\n\n", len(results))
	for _, r := range results {
		fmt.Printf("  %.4f  %s  %s\n", r.Score, r.Commit.ShortHash(), r.Commit.Message)
	}

	return nil
}
