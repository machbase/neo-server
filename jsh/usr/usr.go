package usr

import (
	"embed"
)

//go:embed bin/* lib/*
var Files embed.FS
