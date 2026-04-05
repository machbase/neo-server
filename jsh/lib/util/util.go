package util

import (
	"context"
	_ "embed"

	"github.com/dop251/goja"
)

//go:embed util.js
var util_js []byte

//go:embed parseArgs.js
var parseArgs_js []byte

//go:embed splitFields.js
var splitFields_js []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"util/index.js":       util_js,
		"util/parseArgs.js":   parseArgs_js,
		"util/splitFields.js": splitFields_js,
	}
}

func Module(_ context.Context, rt *goja.Runtime, module *goja.Object) {
	// Export native functions
	_ = rt
	_ = module
}
