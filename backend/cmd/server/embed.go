package main

import "embed"

//go:embed web/dist
var frontendFiles embed.FS
