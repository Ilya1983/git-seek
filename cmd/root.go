package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/user/git-seek/internal/git"
)

var debugFlag bool

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
