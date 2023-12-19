package snowflake

import (
	"strings"
	"time"
)

var _idGen, _ = NewNode(time.Now().Unix() % 1024)

func Generate() string {
	return strings.ReplaceAll(_idGen.Generate().Base64(), "=", "_")
}
