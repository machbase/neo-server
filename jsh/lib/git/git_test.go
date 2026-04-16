package git_test

import (
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestRequireGitModuleAlias(t *testing.T) {
	test_engine.RunTest(t, test_engine.TestCase{
		Name: "require git alias",
		Script: `
			const git = require('git');
			console.println(typeof git.listRemoteRefs);
			console.println(typeof git.cloneRepository);
		`,
		Output: []string{
			"function",
			"function",
		},
	})
}
