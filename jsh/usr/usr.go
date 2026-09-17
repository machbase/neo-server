package usr

import (
	"embed"
)

//go:embed bin/* lib/* share/help/jsh/* share/help/neo-shell/*
var Files embed.FS
