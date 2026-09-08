package main

import "fmt"

var (
	version   = "dev"
	gitCommit = "unknown"
	buildTime = "unknown"
)

func versionString() string {
	return fmt.Sprintf("3m-ui %s\ngit commit: %s\nbuild time: %s", version, gitCommit, buildTime)
}
