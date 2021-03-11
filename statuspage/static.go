package statuspage

import "embed"

//nolint
//go:embed dist/*
var StaticContent embed.FS
