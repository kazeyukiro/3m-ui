package main

import (
	"fmt"

	"github.com/kazeyukiro/3m-ui/backend/internal/buildinfo"
)

func versionString() string {
	return fmt.Sprintf("3m-ui %s\ngit commit: %s\nbuild time: %s",
		buildinfo.Version, buildinfo.GitCommit, buildinfo.BuildTime)
}
