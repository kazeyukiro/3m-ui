package main

import "fmt"

var (
	version   = "v1.0.0"
	gitCommit = "unknown"
	buildTime = "unknown"
)

func versionString() string {
	return fmt.Sprintf("3m-ui %s\ngit commit: %s\nbuild time: %s", version, gitCommit, buildTime)
}
