package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceTQLFileSystemIntegration(t *testing.T) {
	for _, rootless := range []bool{false, true} {
		for _, value := range []string{"alpha", "beta"} {
			t.Run(fmt.Sprintf("rootless=%t/%s", rootless, value), func(t *testing.T) {
				t.Parallel()
				root := t.TempDir()
				data := t.TempDir()
				const code = "STRING(file('/data/payload.txt'))\nCSV()"
				require.NoError(t, os.WriteFile(filepath.Join(data, "payload.txt"), []byte(value), 0600))
				require.NoError(t, os.WriteFile(filepath.Join(data, "task.tql"), []byte(code), 0600))
				mounts := []string{"/data=" + data}
				if !rootless {
					mounts = append(mounts, "/="+root)
				}
				files, err := NewWorkspaceFileSystem(mounts)
				require.NoError(t, err)
				script, err := tql.NewLoader(files).Load("/data/task.tql")
				require.NoError(t, err)
				for _, loaded := range []bool{false, true} {
					output := &bytes.Buffer{}
					task := tql.NewTaskContext(t.Context())
					task.SetOutputWriter(output)
					if loaded {
						require.NoError(t, task.CompileScript(script))
					} else {
						task.SetFileSystem(files)
						require.NoError(t, task.CompileString(code))
					}
					require.NoError(t, task.Execute().Err)
					require.Equal(t, value+"\n\n", output.String())
				}
			})
		}
	}
}

func TestWorkspaceTQLScriptWorkMount(t *testing.T) {
	root := t.TempDir()
	data := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "marker.txt"), []byte("root-marker"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(data, "marker.txt"), []byte("hidden"), 0600))
	tests := []struct {
		name   string
		mounts []string
		code   string
		expect string
	}{
		{
			name:   "root_mount",
			mounts: []string{"/=" + root, "/data=" + data},
			code:   `$.yield(require("fs").readFileSync("/work/marker.txt", "utf8"));`,
			expect: "root-marker\n\n",
		},
		{
			name:   "rootless",
			mounts: []string{"/data=" + data},
			code:   `$.yield(require("fs").readdirSync("/work").length === 2);`,
			expect: "true\n\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			files, err := NewWorkspaceFileSystem(tc.mounts)
			require.NoError(t, err)
			output := &bytes.Buffer{}
			task := tql.NewTaskContext(t.Context())
			task.SetFileSystem(files)
			task.SetOutputWriter(output)
			require.NoError(t, task.CompileString("SCRIPT({\n"+tc.code+"\n})\nCSV()"))
			require.NoError(t, task.Execute().Err)
			require.Equal(t, tc.expect, output.String())
		})
	}
}

func TestMarkdownRPCUsesServerWorkspace(t *testing.T) {
	echo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.Copy(w, r.Body)
		if err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(echo.Close)
	content := fmt.Sprintf("```http\nPOST %s\nContent-Type: text/plain\n\n< /payload.txt\n```\n", echo.URL)
	for _, value := range []string{"alpha", "beta"} {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "payload.txt"), []byte(value), 0600))
		files, err := NewWorkspaceFileSystem([]string{"/=" + dir})
		require.NoError(t, err)
		server := &Server{fileService: files}
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			output, err := server.rpcMarkdownRender(content, false, "")
			require.NoError(t, err)
			require.Contains(t, output, value)
			require.NotContains(t, output, "Error opening file")
		})
	}
}

type charFixture struct {
	t    *testing.T
	base string
}

func newCharFixture(t *testing.T) *charFixture {
	return &charFixture{t: t, base: t.TempDir()}
}

// dir creates a host directory under the fixture base and returns its absolute path.
func (cf *charFixture) dir(name string) string {
	p := filepath.Join(cf.base, filepath.FromSlash(name))
	require.NoError(cf.t, os.MkdirAll(p, 0755))
	return p
}

func (cf *charFixture) file(name string, content string) string {
	p := filepath.Join(cf.base, filepath.FromSlash(name))
	require.NoError(cf.t, os.MkdirAll(filepath.Dir(p), 0755))
	require.NoError(cf.t, os.WriteFile(p, []byte(content), 0644))
	return p
}

func (cf *charFixture) exists(name string) bool {
	_, err := os.Stat(filepath.Join(cf.base, filepath.FromSlash(name)))
	return err == nil
}

func (cf *charFixture) files(values ...string) *WorkspaceFileSystem {
	dirs, err := NormalizeFileDirs(values)
	require.NoError(cf.t, err)
	ret, err := NewWorkspaceFileSystem(dirs)
	require.NoError(cf.t, err)
	require.NoError(cf.t, ret.ValidateWorkMounts())
	return ret
}

// jsh builds the JSH filesystem that a server derived runtime would see.
func (cf *charFixture) jsh(s *WorkspaceFileSystem) *engine.FS {
	efs := engine.NewFS()
	for _, tab := range s.WorkFSTabs() {
		dirFS, err := engine.DirFS(tab.Source)
		require.NoError(cf.t, err)
		require.NoError(cf.t, efs.Mount(tab.MountPoint, dirFS))
	}
	require.Empty(cf.t, efs.MountWarnings())
	return efs
}

func childNames(ent *WorkspaceEntry) []string {
	ret := []string{}
	for _, c := range ent.Children {
		if c.Virtual {
			continue // e.g. the Tutorials git sample
		}
		ret = append(ret, c.Name)
	}
	return ret
}

func dirEntryNames(entries []fs.DirEntry) []string {
	ret := []string{}
	for _, e := range entries {
		if e.Name() == "." || e.Name() == ".." {
			continue
		}
		ret = append(ret, e.Name())
	}
	slices.Sort(ret)
	return ret
}

// Both views must read the same host file for the same workspace path.
func TestCharSameHostTarget(t *testing.T) {
	cf := newCharFixture(t)
	cf.file("root/r.txt", "root")
	cf.file("root/sub/s.txt", "root-sub")
	cf.file("other/o.txt", "other")
	cf.file("deep/d.txt", "deep")
	s := cf.files("/="+cf.dir("root"), "/a="+cf.dir("other"), "/apps/central/app="+cf.dir("deep"))
	efs := cf.jsh(s)

	for p, expect := range map[string]string{
		"/r.txt":                  "root",
		"/sub/s.txt":              "root-sub",
		"/a/o.txt":                "other",
		"/apps/central/app/d.txt": "deep",
	} {
		ent, err := s.Get(p)
		require.NoError(t, err, p)
		require.Equal(t, expect, string(ent.Content), p)
		b, err := efs.ReadFile(WorkPath(p))
		require.NoError(t, err, p)
		require.Equal(t, expect, string(b), p)
	}
}

// Listing of implicit parent directories of a multi-depth mount.
func TestCharImplicitParentListing(t *testing.T) {
	cf := newCharFixture(t)
	cf.file("root/r.txt", "root")
	cf.file("deep/d.txt", "deep")
	s := cf.files("/="+cf.dir("root"), "/apps/central/app="+cf.dir("deep"))
	efs := cf.jsh(s)

	// WorkspaceFileSystem: every implicit level is listed and marked read-only
	ent, err := s.Get("/")
	require.NoError(t, err)
	require.Equal(t, []string{"apps", "r.txt"}, childNames(ent))
	ent, err = s.Get("/apps")
	require.NoError(t, err)
	require.True(t, ent.IsDir)
	require.True(t, ent.ReadOnly)
	require.Equal(t, []string{"central"}, childNames(ent))
	ent, err = s.Get("/apps/central")
	require.NoError(t, err)
	require.Equal(t, []string{"app"}, childNames(ent))

	entries, err := efs.ReadDir("/work")
	require.NoError(t, err)
	require.Equal(t, []string{"apps", "r.txt"}, dirEntryNames(entries))
	entries, err = efs.ReadDir("/work/apps")
	require.NoError(t, err)
	require.Equal(t, []string{"central"}, dirEntryNames(entries))
	entries, err = efs.ReadDir("/work/apps/central")
	require.NoError(t, err)
	require.Equal(t, []string{"app"}, dirEntryNames(entries))
	for _, path := range []string{"/work/apps", "/work/apps/central"} {
		info, err := efs.Stat(path)
		require.NoError(t, err)
		require.True(t, info.IsDir())
	}
}

// Without a root mount, "/" is an implicit directory.
func TestCharNoRootMount(t *testing.T) {
	cf := newCharFixture(t)
	cf.file("other/o.txt", "other")
	s := cf.files("/a/b=" + cf.dir("other"))
	efs := cf.jsh(s)

	_, ok := s.RootDir()
	require.False(t, ok)

	root, err := s.Get("/")
	require.NoError(t, err)
	require.True(t, root.ReadOnly)
	require.Equal(t, []string{"a"}, childNames(root))
	parent, err := s.Get("/a")
	require.NoError(t, err)
	require.True(t, parent.ReadOnly)
	require.Equal(t, []string{"b"}, childNames(parent))
	for _, path := range []string{"/", "/a"} {
		ent, err := s.Get(path)
		require.NoError(t, err)
		require.Empty(t, ent.abspath, path) // virtual directories have no host path
		_, err = s.FindRealPath(path)
		require.Error(t, err, path)
	}
	ent, err := s.Get("/a/b/o.txt")
	require.NoError(t, err)
	require.Equal(t, "other", string(ent.Content))

	entries, err := efs.ReadDir("/work")
	require.NoError(t, err)
	require.Equal(t, []string{"a"}, dirEntryNames(entries))
	entries, err = efs.ReadDir("/work/a")
	require.NoError(t, err)
	require.Equal(t, []string{"b"}, dirEntryNames(entries))
	for _, path := range []string{"/", "/work", "/work/a"} {
		info, err := efs.Stat(path)
		require.NoError(t, err)
		require.True(t, info.IsDir())
	}
}

// Mount points are matched on the path element boundary, so "/database"
// is resolved in the root mount, not in the "/data" mount (fixed defect).
func TestCharMountPrefixBoundary(t *testing.T) {
	cf := newCharFixture(t)
	cf.file("root/database/x.txt", "root-database")
	cf.file("data/base/x.txt", "data-base")
	s := cf.files("/="+cf.dir("root"), "/data="+cf.dir("data"))
	efs := cf.jsh(s)

	ent, err := s.Get("/database/x.txt")
	require.NoError(t, err)
	require.Equal(t, "root-database", string(ent.Content))
	rp, err := s.FindRealPath("/database/x.txt")
	require.NoError(t, err)
	require.Equal(t, "/", rp.Mount.MountPoint)
	rp, err = s.FindRealPath("/data/base/x.txt")
	require.NoError(t, err)
	require.Equal(t, "/data", rp.Mount.MountPoint)
	require.Equal(t, "base/x.txt", rp.RelativePath)

	b, err := efs.ReadFile("/work/database/x.txt")
	require.NoError(t, err)
	require.Equal(t, "root-database", string(b)) // JSH resolves on path boundary
}

func TestCharServerMountProjectionOrderIndependent(t *testing.T) {
	cf := newCharFixture(t)
	cf.file("root/database/x.txt", "root")
	cf.file("data/base/x.txt", "data")
	rootDir := cf.dir("root")
	dataDir := cf.dir("data")

	for _, mounts := range [][]string{
		{"/=" + rootDir, "/data=" + dataDir},
		{"/data=" + dataDir, "/=" + rootDir},
	} {
		s := cf.files(mounts...)
		efs := cf.jsh(s)

		for virtualPath, want := range map[string]string{
			"/work/database/x.txt":  "root",
			"/work/data/base/x.txt": "data",
		} {
			got, err := efs.ReadFile(virtualPath)
			require.NoError(t, err, virtualPath)
			require.Equal(t, want, string(got), virtualPath)
		}
	}
}

// Both configuration parsing and direct construction reject duplicate mounts.
func TestCharDuplicateMounts(t *testing.T) {
	cf := newCharFixture(t)
	cf.file("one/x.txt", "one")
	cf.file("two/x.txt", "two")
	_, err := NormalizeFileDirs([]string{"/a=" + cf.dir("one"), "/a=" + cf.dir("two")})
	require.Error(t, err)

	s, err := NewWorkspaceFileSystem([]string{"/a=" + cf.dir("one"), "/a=" + cf.dir("two")})
	require.ErrorIs(t, err, os.ErrExist)
	require.Nil(t, s)
	// Equivalent spellings of a mount point are duplicates as well.
	s, err = NewWorkspaceFileSystem([]string{"/a=" + cf.dir("one"), "a/=" + cf.dir("two")})
	require.ErrorIs(t, err, os.ErrExist)
	require.Nil(t, s)
}

func TestCharListingContract(t *testing.T) {
	cf := newCharFixture(t)
	cf.file("root/b.txt", "bb")
	cf.file("root/a.sql", "a")
	cf.file("root/.hidden", "h")
	cf.file("root/node_modules/x.js", "x")
	cf.dir("root/zdir")
	cf.dir("root/adir")
	cf.file("root/empty.txt", "")
	s := cf.files("/=" + cf.dir("root"))

	ent, err := s.Get("/")
	require.NoError(t, err)
	// directories first, then names; dot files and ignored names are hidden
	require.Equal(t, []string{"adir", "zdir", "a.sql", "b.txt", "empty.txt"}, childNames(ent))

	ent, err = s.GetFilter("/", GlobFilter("*.sql"))
	require.NoError(t, err)
	require.Equal(t, []string{"adir", "zdir", "a.sql"}, childNames(ent))

	// JSON keys of a directory child and a file child
	byName := map[string]*WorkspaceSubEntry{}
	for _, c := range ent.Children {
		byName[c.Name] = c
	}
	keys := func(v any) []string {
		b, err := json.Marshal(v)
		require.NoError(t, err)
		m := map[string]any{}
		require.NoError(t, json.Unmarshal(b, &m))
		ret := []string{}
		for k := range m {
			ret = append(ret, k)
		}
		slices.Sort(ret)
		return ret
	}
	require.Equal(t, []string{"gitClone", "isDir", "lastModifiedUnixMillis", "name", "size", "type", "virtual"}, keys(byName["a.sql"]))
	require.Equal(t, "dir", byName["adir"].Type)
	require.Equal(t, ".sql", byName["a.sql"].Type)

	ent, err = s.Get("/zdir")
	require.NoError(t, err)
	require.Equal(t, []string{"gitClone", "isDir", "name"}, keys(ent)) // empty dir has no "children"

	ent, err = s.Get("/empty.txt")
	require.NoError(t, err)
	require.Equal(t, []string{"gitClone", "isDir", "name"}, keys(ent)) // empty file has no "content"
}

func TestCharMutations(t *testing.T) {
	cf := newCharFixture(t)
	cf.file("root/f.txt", "f")
	cf.file("root/full/x.txt", "x")
	cf.dir("root/empty")
	cf.file("other/o.txt", "o")
	s := cf.files("/="+cf.dir("root"), "/a="+cf.dir("other"))

	// MkDir is single level
	_, err := s.MkDir("/no/such")
	require.Error(t, err)
	_, err = s.MkDir("/empty")
	require.Error(t, err)

	// Set creates/overwrites files, but not directories or missing parents
	require.NoError(t, s.Set("/new.txt", []byte("n")))
	require.NoError(t, s.Set("/new.txt", []byte("n2")))
	require.Error(t, s.Set("/empty", []byte("x")))
	require.Error(t, s.Set("/no/such.txt", []byte("x")))

	// Remove: file and empty directory only
	require.NoError(t, s.Remove("/new.txt"))
	require.NoError(t, s.Remove("/empty"))
	require.Error(t, s.Remove("/full"))
	require.NoError(t, s.RemoveRecursive("/full"))
	require.False(t, cf.exists("root/full"))

	// Rename refuses an existing destination and keeps both contents
	cf.file("root/g.txt", "g")
	require.ErrorContains(t, s.Rename("/f.txt", "/g.txt"), "already exists")
	ent, _ := s.Get("/g.txt")
	require.Equal(t, "g", string(ent.Content))

	// Rename across mounts on the same disk succeeds (engine.FS would return EXDEV)
	require.NoError(t, s.Rename("/a/o.txt", "/moved.txt"))
	require.True(t, cf.exists("root/moved.txt"))
	require.False(t, cf.exists("other/o.txt"))

	// os.Rename errors are returned as they are, so EXDEV stays detectable
	err = s.Rename("/moved.txt", "/a/no/such/dir.txt")
	var linkErr *os.LinkError
	require.ErrorAs(t, err, &linkErr)
	require.ErrorIs(t, err, fs.ErrNotExist)
	require.True(t, cf.exists("root/moved.txt"))
}

func TestCharBasicMutationContractUsesIndependentFixtures(t *testing.T) {
	type operations struct {
		name   string
		prefix string
		mkdir  func(string) error
		write  func(string, []byte) error
		read   func(string) ([]byte, error)
		rename func(string, string) error
		remove func(string) error
		rmdir  func(string) error
	}

	serviceFixture := newCharFixture(t)
	serviceRoot := serviceFixture.dir("root")
	service, err := NewWorkspaceFileSystem([]string{"/=" + serviceRoot})
	require.NoError(t, err)
	serviceOps := operations{
		name: "server-service",
		mkdir: func(path string) error {
			_, err := service.MkDir(path)
			return err
		},
		write: service.Set,
		read: func(path string) ([]byte, error) {
			entry, err := service.Get(path)
			if err != nil {
				return nil, err
			}
			return entry.Content, nil
		},
		rename: service.Rename,
		remove: service.Remove,
		rmdir:  service.Remove,
	}

	engineFixture := newCharFixture(t)
	engineRoot := engineFixture.dir("root")
	engineFS := engine.NewFS()
	dirFS, err := engine.DirFS(engineRoot)
	require.NoError(t, err)
	require.NoError(t, engineFS.Mount("/work", dirFS))
	engineOps := operations{
		name:   "jsh-engine",
		prefix: "/work",
		mkdir:  engineFS.Mkdir,
		write:  engineFS.WriteFile,
		read:   engineFS.ReadFile,
		rename: engineFS.Rename,
		remove: engineFS.Unlink,
		rmdir:  engineFS.Rmdir,
	}

	for _, ops := range []operations{serviceOps, engineOps} {
		t.Run(ops.name, func(t *testing.T) {
			dir := ops.prefix + "/contract"
			source := dir + "/source.txt"
			dest := dir + "/renamed.txt"
			require.NoError(t, ops.mkdir(dir))
			require.NoError(t, ops.write(source, []byte("payload")))
			content, err := ops.read(source)
			require.NoError(t, err)
			require.Equal(t, []byte("payload"), content)
			require.NoError(t, ops.rename(source, dest))
			require.NoError(t, ops.remove(dest))
			require.NoError(t, ops.rmdir(dir))
			_, err = ops.read(dest)
			require.ErrorIs(t, err, os.ErrNotExist)
		})
	}
}

// Mount point directories themselves are protected like engine.FS (EBUSY).
func TestCharMountPointProtected(t *testing.T) {
	cf := newCharFixture(t)
	cf.dir("root")
	cf.file("other/o.txt", "o")
	s := cf.files("/="+cf.dir("root"), "/a="+cf.dir("other"))

	for _, p := range []string{"/a", "/a/", "/a/.", "/", "/x/.."} {
		require.ErrorContains(t, s.Rename(p, "/b"), "mount point", p)
		require.ErrorContains(t, s.Remove(p), "mount point", p)
		require.ErrorContains(t, s.RemoveRecursive(p), "mount point", p)
	}
	require.True(t, cf.exists("other/o.txt"))
	require.True(t, cf.exists("root"))

	// the contents of a mount point are still mutable
	require.NoError(t, s.Rename("/a/o.txt", "/a/p.txt"))
	require.NoError(t, s.Remove("/a/p.txt"))
}

func TestCharMountAncestorProtected(t *testing.T) {
	cf := newCharFixture(t)
	rootDir := cf.dir("root")
	cf.file("root/parent/keep.txt", "keep")
	cf.file("root/source.txt", "source")
	nestedDir := cf.dir("nested")
	cf.file("nested/child.txt", "child")
	s := cf.files("/="+rootDir, "/parent/nested="+nestedDir)

	require.ErrorContains(t, s.Remove("/parent"), "contains a mount point")
	require.ErrorContains(t, s.RemoveRecursive("/parent"), "contains a mount point")
	require.ErrorContains(t, s.Rename("/parent", "/moved"), "contains a mount point")
	require.ErrorContains(t, s.Rename("/source.txt", "/parent"), "contains a mount point")
	require.True(t, cf.exists("root/parent/keep.txt"))
	require.True(t, cf.exists("root/source.txt"))
	require.True(t, cf.exists("nested/child.txt"))
}

func TestCharHostPathContracts(t *testing.T) {
	cf := newCharFixture(t)
	cf.file("root/file.txt", "root")
	cf.file("root/work/file.txt", "nested-work")
	cf.file("app/file.txt", "app")
	s := cf.files("/="+cf.dir("root"), "/apps/tool="+cf.dir("app"))

	rp, err := s.FindRealPath("/missing.txt")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(cf.base, "root", "missing.txt"), rp.AbsPath)
	_, err = s.Get("/missing.txt")
	require.ErrorIs(t, err, os.ErrNotExist)

	entry, err := s.Get("/apps")
	require.NoError(t, err)
	require.True(t, entry.ReadOnly)
	require.Empty(t, entry.abspath)

	rp, err = s.FindRealPath("/apps/tool/file.txt")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(cf.base, "app", "file.txt"), rp.AbsPath)
	require.Equal(t, "/apps/tool", rp.Mount.MountPoint)
	require.Equal(t, "file.txt", rp.RelativePath)
	entry, err = s.Get("/apps/tool/file.txt")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(cf.base, "app", "file.txt"), entry.abspath)
	entry, err = s.Get("/work/file.txt")
	require.NoError(t, err)
	require.Equal(t, "nested-work", string(entry.Content))
}

func TestCharUnavailableSource(t *testing.T) {
	cf := newCharFixture(t)
	missing := filepath.Join(cf.base, "missing")
	s, err := NewWorkspaceFileSystem([]string{"/a=" + missing})
	require.NoError(t, err)
	_, err = s.Get("/a/file.txt")
	require.ErrorIs(t, err, os.ErrNotExist)
	require.NotEmpty(t, s.CheckWorkMounts())

	require.NoError(t, os.Mkdir(missing, 0o755))
	require.NoError(t, s.Set("/a/file.txt", []byte("data")))
	entry, err := s.Get("/a/file.txt")
	require.NoError(t, err)
	require.Equal(t, "data", string(entry.Content))
	require.Empty(t, s.CheckWorkMounts())
	require.Equal(t, []MountSpec{{MountPoint: "/a", Dir: missing}}, s.MountSpecs())
}

// Mutation APIs clean the path, so ".." can not escape the mounts (fixed defect).
// The HTTP file API passes the path parameter and the rename destination as they are.
func TestCharMutationPathEscape(t *testing.T) {
	cf := newCharFixture(t)
	cf.dir("ws/root")
	cf.file("ws/other/o.txt", "o")
	s := cf.files("/="+cf.dir("ws/root"), "/a="+cf.dir("ws/other"))

	_, err := s.Get("/a/../../escape.txt")
	require.Error(t, err)

	// "/../escape.txt" is cleaned to "/escape.txt" in the root mount
	require.NoError(t, s.Set("/../escape.txt", []byte("x")))
	require.False(t, cf.exists("ws/escape.txt"))
	require.True(t, cf.exists("ws/root/escape.txt"))
	require.NoError(t, s.Set("../../escape2.txt", []byte("x")))
	require.False(t, cf.exists("escape2.txt"))
	require.True(t, cf.exists("ws/root/escape2.txt"))

	// "/a/../../x" is cleaned to "/x" in the root mount, not in "/a"
	rp, err := s.FindRealPath("/a/../../moved.txt")
	require.NoError(t, err)
	require.Equal(t, "/", rp.Mount.MountPoint)
	require.NoError(t, s.Rename("/a/o.txt", "/a/../../moved.txt"))
	require.False(t, cf.exists("moved.txt"))
	require.True(t, cf.exists("ws/root/moved.txt"))

	_, err = s.MkDir("/a/../../../newdir")
	require.NoError(t, err)
	require.False(t, cf.exists("newdir"))
	require.True(t, cf.exists("ws/root/newdir"))

	cf.file("ws/victim.txt", "v")
	require.Error(t, s.Remove("/../victim.txt"))
	require.True(t, cf.exists("ws/victim.txt"))
}

const urlGitSample = "https://github.com/machbase/neo-tutorials.git"

func TestFsGET(t *testing.T) {
	files, err := NewWorkspaceFileSystem([]string{"/=./test/workspace/root"})
	require.Nil(t, err)
	require.NotNil(t, files)

	ret, err := files.GetFilter("/", GlobFilter("*.sql"))
	require.Nil(t, err)
	require.NotNil(t, ret)
	require.Equal(t, true, ret.IsDir)
	require.Equal(t, "/", ret.Name)
	require.Equal(t, 2, len(ret.Children))
	require.Equal(t, "hello.sql", ret.Children[0].Name)
	require.Equal(t, ".sql", ret.Children[0].Type)
	require.Equal(t, "select.sql", ret.Children[1].Name)
	require.Equal(t, ".sql", ret.Children[1].Type)

	files, err = NewWorkspaceFileSystem([]string{"/=./test/workspace/root", "/data1=./test/workspace/data1"})
	require.Nil(t, err)
	require.NotNil(t, files)

	ret, err = files.Get("/")
	require.Nil(t, err)
	require.NotNil(t, ret)
	require.Equal(t, true, ret.IsDir)
	require.Equal(t, "/", ret.Name)
	require.Equal(t, 4, len(ret.Children))
	require.Equal(t, "data1", ret.Children[0].Name)
	require.Equal(t, "dir", ret.Children[0].Type)
	require.Equal(t, true, ret.Children[0].IsDir)
	require.Equal(t, "example.tql", ret.Children[1].Name)
	require.Equal(t, ".tql", ret.Children[1].Type)
	require.Equal(t, false, ret.Children[1].IsDir)
	require.Equal(t, "hello.sql", ret.Children[2].Name)
	require.Equal(t, ".sql", ret.Children[2].Type)
	require.Equal(t, false, ret.Children[2].IsDir)
	require.Equal(t, "select.sql", ret.Children[3].Name)
	require.Equal(t, ".sql", ret.Children[3].Type)
	require.Equal(t, false, ret.Children[3].IsDir)

	// do not allow accessing out side of the given dirs
	ret, err = files.Get("/../notaccess.tql")
	require.NotNil(t, err)
	require.Nil(t, ret)

	// do not allow accessing out side of the given dirs
	ret, err = files.Get("/data1/../../notaccess.tql")
	require.NotNil(t, err)
	require.Nil(t, ret)

	// access relative path
	ret, err = files.Get("/data1/../example.tql")
	require.Nil(t, err)
	require.NotNil(t, ret)
	require.Equal(t, "example.tql", ret.Name)

	// directory without trailing slash
	ret, err = files.Get("/data1")
	require.Nil(t, err)
	require.NotNil(t, files)
	require.Equal(t, true, ret.IsDir)
	require.Equal(t, "data1", ret.Name)
	require.Equal(t, 1, len(ret.Children))
	require.Equal(t, "simple.tql", ret.Children[0].Name)
	require.Equal(t, false, ret.Children[0].IsDir)

	// directory with trailing slash
	ret, err = files.Get("/data1/")
	require.Nil(t, err)
	require.NotNil(t, files)
	require.Equal(t, true, ret.IsDir)
	require.Equal(t, "data1", ret.Name)
	require.Equal(t, 1, len(ret.Children))
	require.Equal(t, "simple.tql", ret.Children[0].Name)
	require.Equal(t, false, ret.Children[0].IsDir)

	// create subdirectory
	ret, err = files.MkDir("/data1/newdir")
	require.Nil(t, err)
	require.NotNil(t, ret)
	require.Equal(t, true, ret.IsDir)
	require.Equal(t, "newdir", ret.Name)

	// write file
	err = files.Set("/data1/newdir/test.txt", []byte("Hello World"))
	require.Nil(t, err)

	// read file
	ret, err = files.Get("/data1/newdir/test.txt")
	require.Nil(t, err)
	require.Equal(t, "test.txt", ret.Name)
	require.Equal(t, "Hello World", string(ret.Content))

	// subdir sort
	ret, err = files.MkDir("/data1/newdir2")
	require.Nil(t, err)
	require.NotNil(t, ret)
	require.Equal(t, true, ret.IsDir)
	require.Equal(t, "newdir2", ret.Name)

	ret, err = files.Get("/data1/")
	require.Nil(t, err)
	require.Equal(t, "newdir", ret.Children[0].Name)
	require.Equal(t, "newdir2", ret.Children[1].Name)

	// rename file
	err = files.Rename("/data1/newdir/test.txt", "/test.txt")
	require.Nil(t, err)

	// delete file
	err = files.Remove("/test.txt")
	require.Nil(t, err)

	// delete directory
	err = files.Remove("/data1/newdir")
	require.Nil(t, err)
	err = files.Remove("/data1/newdir2")
	require.Nil(t, err)

	// host path
	rp, err := files.FindRealPath("/data1/simple.tql")
	require.Nil(t, err)
	abspath, _ := filepath.Abs("test/workspace/data1/simple.tql")
	require.Equal(t, abspath, rp.AbsPath)
}

func TestFsGit(t *testing.T) {
	files, err := NewWorkspaceFileSystem([]string{"./test/workspace/root", "./test/workspace/data1"})
	require.Nil(t, err)
	require.NotNil(t, files)

	dest := "/data1/neo-tutorials"
	entry, err := files.GitClone(dest, urlGitSample, nil)
	if err != nil {
		t.Log("ERR", err.Error())
	}
	require.Nil(t, err)
	require.NotNil(t, entry)

	require.True(t, entry.IsDir)
	require.True(t, len(entry.Children) > 0)

	entry, err = files.GitPull(dest, urlGitSample, nil)
	if err != nil {
		t.Log("ERR", err.Error())
	}
	require.Nil(t, err)
	require.NotNil(t, entry)

	err = files.RemoveRecursive(dest)
	require.Nil(t, err)
}

func TestMounts(t *testing.T) {
	files, err := NewWorkspaceFileSystem([]string{"/=./test/workspace/root", "data1=./test/workspace/data1", "data1/apps/neo-docs/=./test/workspace/apps/neo-docs"})
	require.Nil(t, err)
	require.NotNil(t, files)

	mnts := []string{}
	for _, spec := range files.MountSpecs() {
		mnts = append(mnts, spec.MountPoint)
	}
	require.Equal(t, []string{"/", "/data1", "/data1/apps/neo-docs"}, mnts)

	rp, err := files.FindRealPath("/data1/simple.tql")
	require.Nil(t, err)
	require.Equal(t, "/data1", rp.Mount.MountPoint)

	neoDocsDir, _ := filepath.Abs("./test/workspace/apps/neo-docs")
	rp, err = files.FindRealPath("/data1/apps/neo-docs")
	require.Nil(t, err)
	require.Equal(t, "/data1/apps/neo-docs", rp.Mount.MountPoint)
	require.Equal(t, neoDocsDir, rp.AbsPath)
	require.Empty(t, rp.RelativePath)

	rp, err = files.FindRealPath("/data1/apps/neo-docs/simple.tql")
	require.Nil(t, err)
	require.Equal(t, "/data1/apps/neo-docs", rp.Mount.MountPoint)
	require.Equal(t, filepath.Join(neoDocsDir, "simple.tql"), rp.AbsPath)
}

func TestMountsMultiDepth(t *testing.T) {
	files, err := NewWorkspaceFileSystem([]string{
		"/=./test/workspace/root",
		"apps/myapp1=./test/workspace/apps/myapp1",
		"apps/myapp2=./test/workspace/apps/myapp2",
		"apps/central/myapp4=./test/workspace/apps/central/myapp4",
		"apps/central/myapp3=./test/workspace/apps/central/myapp3",
	})
	require.Nil(t, err)
	require.NotNil(t, files)

	mnts := []string{}
	for _, spec := range files.MountSpecs() {
		mnts = append(mnts, spec.MountPoint)
	}
	require.Equal(t, []string{"/", "/apps/central/myapp3", "/apps/central/myapp4", "/apps/myapp1", "/apps/myapp2"}, mnts)

	entry, err := files.Get("/")
	require.Nil(t, err)
	require.NotNil(t, entry)
	childrenNames := []string{}
	for _, ent := range entry.Children {
		childrenNames = append(childrenNames, ent.Name)
	}
	require.Equal(t, []string{"apps", "example.tql", "hello.sql", "select.sql"}, childrenNames)
	require.False(t, entry.ReadOnly)

	entry, err = files.Get("/apps")
	require.Nil(t, err)
	require.NotNil(t, entry)
	childrenNames = []string{}
	for _, ent := range entry.Children {
		childrenNames = append(childrenNames, ent.Name)
	}
	require.Equal(t, []string{"central", "myapp1", "myapp2"}, childrenNames)
	require.True(t, entry.ReadOnly)

	entry, err = files.Get("/apps/central")
	require.Nil(t, err)
	require.NotNil(t, entry)
	childrenNames = []string{}
	for _, ent := range entry.Children {
		childrenNames = append(childrenNames, ent.Name)
	}
	require.Equal(t, []string{"myapp3", "myapp4"}, childrenNames)
	require.True(t, entry.ReadOnly)
}
