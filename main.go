package main

import (
	"github.com/user/git-seek/cmd"
)

// Version information set via -ldflags at build time
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	cmd.SetVersionInfo(version, commit, date)
	cmd.Execute()
}
