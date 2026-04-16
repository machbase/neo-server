package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dop251/goja"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"
)

func TestListRemoteRefsAndCloneRepository(t *testing.T) {
	repoDir := createLocalRepository(t, "main", []string{"v1.2.3"})
	remoteURL := fileURLFromPathForGitTest(repoDir)

	refs, err := listRemoteRefs(remoteURL)
	require.NoError(t, err)
	require.NotEmpty(t, refs)

	var sawBranch bool
	var sawTag bool
	var sawHead bool
	for _, ref := range refs {
		if ref["isBranch"] == true && ref["shortName"] == "main" {
			sawBranch = true
		}
		if ref["isTag"] == true && ref["shortName"] == "v1.2.3" {
			sawTag = true
		}
		if ref["isHEAD"] == true {
			sawHead = true
		}
	}
	require.True(t, sawBranch)
	require.True(t, sawTag)
	require.True(t, sawHead)

	cloneDir := filepath.Join(t.TempDir(), "clone")
	result, err := cloneRepository(remoteURL, cloneDir, cloneOptions{
		Ref:          "v1.2.3",
		RefType:      "tag",
		Depth:        1,
		SingleBranch: false,
		RemoveGitDir: true,
	})
	require.NoError(t, err)
	require.NotEmpty(t, result["headRef"])
	require.NotEmpty(t, result["headHash"])

	_, err = os.Stat(filepath.Join(cloneDir, ".git"))
	require.True(t, os.IsNotExist(err))

	content, err := os.ReadFile(filepath.Join(cloneDir, "index.js"))
	require.NoError(t, err)
	require.Contains(t, string(content), "git-module")
}

func TestModuleExportsWorkWithJSOptions(t *testing.T) {
	repoDir := createLocalRepository(t, "main", []string{"v1.2.3"})
	cloneDir := filepath.Join(t.TempDir(), "clone")
	remoteURL := fileURLFromPathForGitTest(repoDir)

	rt := goja.New()
	module := rt.NewObject()
	exports := rt.NewObject()
	require.NoError(t, module.Set("exports", exports))

	Module(context.Background(), rt, module)

	cloneFn, ok := goja.AssertFunction(exports.Get("cloneRepository"))
	require.True(t, ok)
	value, err := cloneFn(goja.Undefined(),
		rt.ToValue(remoteURL),
		rt.ToValue(cloneDir),
		rt.ToValue(map[string]any{
			"ref":          "main",
			"refType":      "branch",
			"depth":        1,
			"singleBranch": true,
			"removeGitDir": true,
		}),
	)
	require.NoError(t, err)
	obj := value.ToObject(rt)
	require.NotNil(t, obj)
	require.NotEmpty(t, obj.Get("headHash").String())

	refsFn, ok := goja.AssertFunction(exports.Get("listRemoteRefs"))
	require.True(t, ok)
	refsValue, err := refsFn(goja.Undefined(), rt.ToValue(remoteURL))
	require.NoError(t, err)
	require.NotNil(t, refsValue.Export())
}

func createLocalRepository(t *testing.T, defaultBranch string, tags []string) string {
	t.Helper()
	repoDir := t.TempDir()
	repo, err := gogit.PlainInit(repoDir, false)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "package.json"), []byte("{\n  \"name\": \"local\",\n  \"version\": \"1.2.3\"\n}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "index.js"), []byte("module.exports = { value: 'git-module' };\n"), 0o644))
	require.NoError(t, wt.AddGlob("."))

	commitHash, err := wt.Commit("initial commit", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "git-test",
			Email: "git-test@example.com",
			When:  time.Unix(1700000000, 0),
		},
	})
	require.NoError(t, err)

	if defaultBranch != "" && defaultBranch != "master" {
		err = wt.Checkout(&gogit.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName(defaultBranch),
			Create: true,
			Hash:   commitHash,
		})
		require.NoError(t, err)
	}

	for _, tagName := range tags {
		_, err := repo.CreateTag(tagName, commitHash, nil)
		require.NoError(t, err)
	}
	return repoDir
}

func fileURLFromPathForGitTest(path string) string {
	clean := filepath.ToSlash(path)
	if len(clean) > 0 && clean[0] == '/' {
		return "file://" + clean
	}
	return "file:///" + clean
}
