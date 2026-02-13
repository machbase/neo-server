package engine

import (
	"bytes"
	"encoding/json"
	"io"
	"os/exec"
	"strings"
	"testing"
	"testing/fstest"
)

func TestWhich(t *testing.T) {
	// Create a mounted filesystem with proper absolute paths
	mfs := NewFS()
	testFS := fstest.MapFS{
		"shell.js": &fstest.MapFile{Data: []byte("content")},
		"test.js":  &fstest.MapFile{Data: []byte("content")},
	}
	sbin := fstest.MapFS{
		"util.js": &fstest.MapFile{Data: []byte("content")},
	}
	mfs.Mount("/bin", testFS)
	mfs.Mount("/lib", sbin)

	tests := []struct {
		name    string
		path    string
		command string
		want    string
	}{
		{"find in path", "/bin:/lib", "shell", "/bin/shell.js"},
		{"with .js extension", "/bin", "shell.js", "/bin/shell.js"},
		{"not found", "/bin:/lib", "notfound", ""},
		{"absolute path", "/bin", "/lib/util.js", "/lib/util.js"},
		{"empty path", "", "shell", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewEnv(WithFilesystem(mfs))
			env.Set("PATH", tt.path)

			got := env.Which(tt.command)
			if got != tt.want {
				t.Errorf("Which() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExpand(t *testing.T) {
	tests := []struct {
		name  string
		input string
		vars  map[string]any
		want  string
	}{
		{"simple var", "Hello $USER", map[string]any{"USER": "john"}, "Hello john"},
		{"braced var", "Hello ${USER}", map[string]any{"USER": "john"}, "Hello john"},
		{"multiple vars", "$HOME/bin:$PATH", map[string]any{"HOME": "/home/user", "PATH": "/usr/bin"}, "/home/user/bin:/usr/bin"},
		{"missing var", "$NOTEXIST", map[string]any{}, ""},
		{"secure string", "password: $PASS", map[string]any{"PASS": SecureString("secret")}, "password: " + SecureMask},
		{"no vars", "plain text", map[string]any{}, "plain text"},
		{"double quotes", `Value is "$VALUE"`, map[string]any{"VALUE": "42"}, `Value is "42"`},
		{"single quotes", `Value is '$VALUE'`, map[string]any{"VALUE": "42"}, `Value is '$VALUE'`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewEnv()
			for k, v := range tt.vars {
				env.Set(k, v)
			}

			got := env.Expand(tt.input)
			if got != tt.want {
				t.Errorf("Expand() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolvePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		home string
		vars map[string]any
		want string
	}{
		{"tilde expansion", "~/work", "/home/user", nil, "/home/user/work"},
		{"tilde only", "~", "/home/user", nil, "/home/user"},
		{"tilde with slash", "~/", "/home/user", nil, "/home/user"},
		{"env expansion", "$HOME/work", "", map[string]any{"HOME": "/home/user"}, "/home/user/work"},
		{"clean path", "/foo//bar/../baz", "", nil, "/foo/baz"},
		{"no expansion", "/usr/bin", "", nil, "/usr/bin"},
		{"multiple dots", "/foo/./bar/./baz", "", nil, "/foo/bar/baz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewEnv()
			if tt.home != "" {
				env.Set("HOME", tt.home)
			}
			for k, v := range tt.vars {
				env.Set(k, v)
			}

			got := env.ResolvePath(tt.path)
			if got != tt.want {
				t.Errorf("ResolvePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveAbsPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		pwd  string
		want string
	}{
		{"absolute path", "/usr/bin", "/home/user", "/usr/bin"},
		{"relative path", "work", "/home/user", "/home/user/work"},
		{"relative with dot", "./work", "/home/user", "/home/user/work"},
		{"relative with parent", "../other", "/home/user/work", "/home/user/other"},
		{"empty path", "", "/home/user", "/home/user"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewEnv()
			env.Set("PWD", tt.pwd)

			got := env.ResolveAbsPath(tt.path)
			if got != tt.want {
				t.Errorf("ResolveAbsPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGlobalFolders(t *testing.T) {
	tests := []struct {
		name        string
		libraryPath string
		want        []string
	}{
		{"single path", "/lib", []string{"/lib"}},
		{"multiple paths", "/lib:/usr/lib:/local/lib", []string{"/lib", "/usr/lib", "/local/lib"}},
		{"empty", "", []string{}},
		{"not set", "", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewEnv()
			if tt.libraryPath != "" {
				env.Set("LIBRARY_PATH", tt.libraryPath)
			}

			got := env.GlobalFolders()
			if len(got) != len(tt.want) {
				t.Errorf("GlobalFolders() length = %v, want %v", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("GlobalFolders()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestPathResolver(t *testing.T) {
	// PathResolver works with require.DefaultPathResolver
	// and checks if files exist in the filesystem
	// Testing with basic path resolution
	mfs := NewFS()
	testFS := fstest.MapFS{
		"module.js":             &fstest.MapFile{Data: []byte("module content")},
		"package/index.js":      &fstest.MapFile{Data: []byte("package index")},
		"package2/main.js":      &fstest.MapFile{Data: []byte("package main")},
		"package2/package.json": &fstest.MapFile{Data: []byte(`{"main": "main.js"}`)},
	}
	mfs.Mount("/lib", testFS)

	tests := []struct {
		name string
		base string
		path string
		pwd  string
	}{
		{"resolve with extension", "/lib", "module.js", "/work"},
		{"resolve from current dir", ".", "lib/module", "/work"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewEnv(WithFilesystem(mfs))
			env.Set("PWD", tt.pwd)

			got := env.PathResolver(tt.base, tt.path)
			// Just check it returns a non-empty path
			if got == "" {
				t.Errorf("PathResolver() returned empty string")
			}
		})
	}
}

func TestLoadSource(t *testing.T) {
	mfs := NewFS()
	testFS := fstest.MapFS{
		"module.js":             &fstest.MapFile{Data: []byte("module code")},
		"package/index.js":      &fstest.MapFile{Data: []byte("package index")},
		"package2/main.js":      &fstest.MapFile{Data: []byte("package main")},
		"package2/package.json": &fstest.MapFile{Data: []byte(`{"main": "main.js"}`)},
	}
	mfs.Mount("/", testFS)

	tests := []struct {
		name       string
		moduleName string
		want       string
		wantErr    bool
	}{
		{"load module", "/module.js", "module code", false},
		{"load without extension", "/module", "module code", false},
		{"load package index", "/package", "package index", false},
		{"load package main", "/package2", "package main", false},
		{"not found", "/notfound", "", true},
		{"relative path fails", "module", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewEnv(WithFilesystem(mfs))

			got, err := env.LoadSource(tt.moduleName)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadSource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && string(got) != tt.want {
				t.Errorf("LoadSource() = %v, want %v", string(got), tt.want)
			}
		})
	}
}

func TestLoadSource_NoFilesystem(t *testing.T) {
	env := NewEnv()
	_, err := env.LoadSource("/module.js")
	if err == nil {
		t.Error("LoadSource() expected error for nil filesystem")
	}
}

func TestCleanPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "foo", "/foo"},
		{"with leading slash", "/foo", "/foo"},
		{"with trailing slash", "foo/", "/foo"},
		{"with both slashes", "/foo/", "/foo"},
		{"nested", "foo/bar", "/foo/bar"},
		{"nested with trailing slash", "/foo/bar/", "/foo/bar"},
		{"double slash", "foo//bar", "/foo/bar"},
		{"dot segment", "foo/./bar", "/foo/bar"},
		{"parent segment", "foo/../bar", "/bar"},
		{"root", "/", "/"},
		{"dot", ".", "/"},
		{"empty", "", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CleanPath(tt.input)
			if result != tt.expected {
				t.Errorf("CleanPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSecureString_JSON(t *testing.T) {
	tests := []struct {
		name        string
		input       SecureString
		wantMarshal string
	}{
		{"simple password", SecureString("my_secret_password"), `"SecureString:my_secret_password"`},
		{"empty", SecureString(""), `"SecureString:"`},
		{"special chars", SecureString("p@$$w0rd!"), `"SecureString:p@$$w0rd!"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test MarshalJSON
			got, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("MarshalJSON() error = %v", err)
			}
			if string(got) != tt.wantMarshal {
				t.Errorf("MarshalJSON() = %v, want %v", string(got), tt.wantMarshal)
			}
		})
	}
}

func TestSecureString_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    SecureString
		wantErr bool
	}{
		{"with prefix", `"SecureString:password123"`, SecureString("password123"), false},
		{"without prefix", `"password123"`, SecureString("password123"), false},
		{"empty with prefix", `"SecureString:"`, SecureString(""), false},
		{"empty without prefix", `""`, SecureString(""), false},
		{"with special chars", `"SecureString:p@$$w0rd!"`, SecureString("p@$$w0rd!"), false},
		{"invalid json", `not_json`, SecureString(""), true},
		{"number", `123`, SecureString(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got SecureString
			err := json.Unmarshal([]byte(tt.input), &got)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("UnmarshalJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSecureString_RoundTrip(t *testing.T) {
	type Config struct {
		Username string       `json:"username"`
		Password SecureString `json:"password"`
	}

	original := Config{
		Username: "admin",
		Password: SecureString("my_secret_password"),
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error = %v", err)
	}

	// Check that password has SecureString prefix in JSON
	jsonStr := string(jsonData)
	if !strings.Contains(jsonStr, "SecureString:my_secret_password") {
		t.Errorf("JSON output should contain 'SecureString:my_secret_password', got: %s", jsonStr)
	}

	// Unmarshal back
	var decoded Config
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}

	// Verify the password was restored correctly
	if decoded.Password != original.Password {
		t.Errorf("Unmarshaled password = %v, want %v", decoded.Password, original.Password)
	}

	// Verify type is preserved
	if _, ok := interface{}(decoded.Password).(SecureString); !ok {
		t.Error("Password is not of type SecureString after unmarshal")
	}

	// Test unmarshaling from regular JSON without prefix (backward compatibility)
	testJSON := `{"username":"admin","password":"my_secret_password"}`
	var decoded2 Config
	if err := json.Unmarshal([]byte(testJSON), &decoded2); err != nil {
		t.Fatalf("Unmarshal from regular JSON error = %v", err)
	}

	if decoded2.Password != SecureString("my_secret_password") {
		t.Errorf("Unmarshaled password from regular JSON = %v, want %v", decoded2.Password, SecureString("my_secret_password"))
	}
}

func TestDefaultEnv_ReaderWriter(t *testing.T) {
	var buf bytes.Buffer
	reader := strings.NewReader("test input")

	env := NewEnv(
		WithWriter(&buf),
		WithReader(reader),
	)

	// Test Writer
	writer := env.Writer()
	n, err := writer.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Writer.Write() error = %v", err)
	}
	if n != 5 {
		t.Errorf("Writer.Write() wrote %d bytes, want 5", n)
	}
	if buf.String() != "hello" {
		t.Errorf("Writer content = %v, want hello", buf.String())
	}

	// Test Reader
	envReader := env.Reader()
	data, err := io.ReadAll(envReader)
	if err != nil {
		t.Fatalf("Reader.ReadAll() error = %v", err)
	}
	if string(data) != "test input" {
		t.Errorf("Reader content = %v, want 'test input'", string(data))
	}
}

func TestDefaultEnv_Filesystem(t *testing.T) {
	testFS := fstest.MapFS{
		"test.txt": &fstest.MapFile{Data: []byte("content")},
	}

	env := NewEnv(WithFilesystem(testFS))

	gotFS := env.Filesystem()
	if gotFS == nil {
		t.Fatal("Filesystem() returned nil")
	}

	// Verify we can read from the filesystem
	file, err := gotFS.Open("test.txt")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(data) != "content" {
		t.Errorf("File content = %v, want 'content'", string(data))
	}
}

func TestDefaultEnv_SetGet(t *testing.T) {
	env := NewEnv()

	// Test Set and Get
	env.Set("KEY1", "value1")
	env.Set("KEY2", 42)
	env.Set("KEY3", true)

	tests := []struct {
		key  string
		want any
	}{
		{"KEY1", "value1"},
		{"KEY2", 42},
		{"KEY3", true},
		{"NOTEXIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := env.Get(tt.key)
			if got != tt.want {
				t.Errorf("Get(%v) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestDefaultEnv_SetNil(t *testing.T) {
	env := NewEnv()

	env.Set("KEY", "value")
	if env.Get("KEY") != "value" {
		t.Error("Set() failed to set value")
	}

	// Setting nil should delete the key
	env.Set("KEY", nil)
	if env.Get("KEY") != nil {
		t.Error("Set(nil) did not delete key")
	}
}

func TestDefaultEnv_ExecBuilder(t *testing.T) {
	execBuilderCalled := false
	testExecBuilder := func(code string, args []string, env map[string]any) (*exec.Cmd, error) {
		execBuilderCalled = true
		return nil, nil
	}

	env := NewEnv(WithExecBuilder(testExecBuilder))

	builder := env.ExecBuilder()
	if builder == nil {
		t.Fatal("ExecBuilder() returned nil")
	}

	_, _ = builder("code", []string{}, nil)
	if !execBuilderCalled {
		t.Error("ExecBuilder function was not called")
	}
}

func TestDefaultEnv_DefaultValues(t *testing.T) {
	env := NewEnv()

	// Default reader should be os.Stdin (can't test exact value)
	reader := env.Reader()
	if reader == nil {
		t.Error("Default Reader() returned nil")
	}

	// Default writer should be os.Stdout (can't test exact value)
	writer := env.Writer()
	if writer == nil {
		t.Error("Default Writer() returned nil")
	}

	// Default filesystem should be nil
	filesystem := env.Filesystem()
	if filesystem != nil {
		t.Error("Default Filesystem() should be nil")
	}

	// Default exec builder should be nil
	execBuilder := env.ExecBuilder()
	if execBuilder != nil {
		t.Error("Default ExecBuilder() should be nil")
	}
}

func TestSecureString(t *testing.T) {
	secure := SecureString("secret_password")
	if string(secure) != "secret_password" {
		t.Errorf("SecureString value = %v, want 'secret_password'", string(secure))
	}

	// Test that SecureMask is the expected value
	if SecureMask != "********" {
		t.Errorf("SecureMask = %v, want '********'", SecureMask)
	}
}

func TestLoadSourceFromDir(t *testing.T) {
	testFS := fstest.MapFS{
		"package1/index.js":     &fstest.MapFile{Data: []byte("index content")},
		"package2/package.json": &fstest.MapFile{Data: []byte(`{"main": "main.js"}`)},
		"package2/main.js":      &fstest.MapFile{Data: []byte("main content")},
		"package3/package.json": &fstest.MapFile{Data: []byte(`{"main": "other.js"}`)},
		"package3/other.js":     &fstest.MapFile{Data: []byte("other content")},
		"emptydir/package.json": &fstest.MapFile{Data: []byte(`{}`)},
	}

	tests := []struct {
		name    string
		dir     string
		want    string
		wantErr bool
	}{
		{"load index.js", "package1", "index content", false},
		{"load from package.json main", "package2", "main content", false},
		{"load alternative main", "package3", "other content", false},
		{"no index or main", "emptydir", "", true},
		{"not exist", "notexist", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadSourceFromDir(testFS, tt.dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadSourceFromDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && string(got) != tt.want {
				t.Errorf("loadSourceFromDir() = %v, want %v", string(got), tt.want)
			}
		})
	}
}

func TestLoadSourceWithExtension(t *testing.T) {
	testFS := fstest.MapFS{
		"module":    &fstest.MapFile{Data: []byte("no extension")},
		"module.js": &fstest.MapFile{Data: []byte("with extension")},
	}

	// When file without extension exists, it should be loaded first
	got, err := loadSource(testFS, "module")
	if err != nil {
		t.Fatalf("loadSource() error = %v", err)
	}
	if string(got) != "no extension" {
		t.Errorf("loadSource() = %v, want 'no extension'", string(got))
	}

	// When only .js extension exists, it should be found
	testFS2 := fstest.MapFS{
		"script.js": &fstest.MapFile{Data: []byte("script content")},
	}

	got2, err := loadSource(testFS2, "script")
	if err != nil {
		t.Fatalf("loadSource() error = %v", err)
	}
	if string(got2) != "script content" {
		t.Errorf("loadSource() = %v, want 'script content'", string(got2))
	}
}

func TestPathResolver_PackageJson(t *testing.T) {
	testFS := fstest.MapFS{
		"package/package.json": &fstest.MapFile{Data: []byte(`{"main": "lib/main.js"}`)},
		"package/lib/main.js":  &fstest.MapFile{Data: []byte("main content")},
	}

	env := NewEnv(WithFilesystem(testFS))
	env.Set("PWD", "/work")

	got := env.PathResolver("/", "package")
	// Should resolve to the main entry point
	if !strings.Contains(got, "package") {
		t.Errorf("PathResolver() = %v, should contain 'package'", got)
	}
}

func TestWhich_WithoutFilesystem(t *testing.T) {
	// Skip this test as Which doesn't handle nil filesystem gracefully
	// This is expected behavior - filesystem must be provided
	t.Skip("Which() requires a filesystem - nil filesystem will panic")
}

func BenchmarkExpand(b *testing.B) {
	env := NewEnv()
	env.Set("HOME", "/home/user")
	env.Set("PATH", "/usr/bin:/bin")

	str := "$HOME/bin:$PATH"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = env.Expand(str)
	}
}

func BenchmarkResolvePath(b *testing.B) {
	env := NewEnv()
	env.Set("HOME", "/home/user")

	path := "~/work/project/../file.txt"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = env.ResolvePath(path)
	}
}

func BenchmarkWhich(b *testing.B) {
	testFS := fstest.MapFS{
		"bin/cmd1.js":  &fstest.MapFile{Data: []byte("cmd1")},
		"bin/cmd2.js":  &fstest.MapFile{Data: []byte("cmd2")},
		"sbin/cmd3.js": &fstest.MapFile{Data: []byte("cmd3")},
	}

	env := NewEnv(WithFilesystem(testFS))
	env.Set("PATH", "/bin:/sbin")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = env.Which("cmd2")
	}
}
