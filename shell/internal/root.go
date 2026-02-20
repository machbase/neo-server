package internal

import (
	"embed"
)

//go:embed usr/*
var FsUsr embed.FS
