package session

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/lib"
)

func TestConfigureExtFlagsGet(t *testing.T) {
	flags := ExtFlags{
		{flag: "server", value: "localhost"},
		{flag: "user", value: "sys"},
	}

	if ef := flags.Get("server"); ef == nil || ef.value != "localhost" {
		t.Fatalf("Get('server') = %v", ef)
	}
	if ef := flags.Get("user"); ef == nil || ef.value != "sys" {
		t.Fatalf("Get('user') = %v", ef)
	}
	if ef := flags.Get("nonexistent"); ef != nil {
		t.Fatalf("Get('nonexistent') = %v, want nil", ef)
	}
}

func TestEntryBuildsRuntimeFromCommandFlags(t *testing.T) {
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	ext := &ExtConfig{
		flags: ExtFlags{{flag: "user", envKey: "APP_USER"}},
		callback: func(conf *engine.Config, extFlags ExtFlags) error {
			if conf.Code != "console.log('ok')" {
				t.Errorf("Config.Code = %q", conf.Code)
			}
			if len(conf.Args) != 1 || conf.Args[0] != "arg1" {
				t.Errorf("Config.Args = %#v, want [arg1]", conf.Args)
			}
			if conf.Env["APP_USER"] != "demo" || extFlags.Get("user").value != "demo" {
				t.Errorf("APP_USER env/flag = %v/%q, want demo/demo", conf.Env["APP_USER"], extFlags.Get("user").value)
			}
			for _, mountPoint := range []string{"/", "/lib", "/work"} {
				if !conf.FSTabs.HasMountPoint(mountPoint) {
					t.Errorf("Config.FSTabs is missing %q", mountPoint)
				}
			}
			return nil
		},
	}

	runtime, exitCode, err := entry(flags, []string{"jsh"}, []string{"-C", "console.log('ok')", "-e", "APP_USER=demo", "arg1"}, ext)
	if err != nil {
		t.Fatalf("entry() error = %v", err)
	}
	if exitCode != 0 || runtime == nil {
		t.Fatalf("entry() = (%v, %d), want non-nil runtime and exit code 0", runtime, exitCode)
	}
}

func TestEntryReturnsParseError(t *testing.T) {
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	_, exitCode, err := entry(flags, nil, []string{"-unknown"}, &ExtConfig{})
	if exitCode != 1 || err == nil || err.Error() != "Error parsing flags: flag provided but not defined: -unknown" {
		t.Fatalf("entry() = (_, %d, %v), want parse error and exit code 1", exitCode, err)
	}
}

func TestEntryReturnsCallbackError(t *testing.T) {
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	wantErr := errors.New("callback failed")
	ext := &ExtConfig{callback: func(*engine.Config, ExtFlags) error { return wantErr }}
	_, exitCode, err := entry(flags, nil, nil, ext)
	if exitCode != 1 || err == nil || err.Error() != "Error in callback: callback failed" {
		t.Fatalf("entry() = (_, %d, %v), want wrapped callback error and exit code 1", exitCode, err)
	}
}

func runEntryWorkProbe(t *testing.T, args []string) string {
	t.Helper()
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "cwd-marker.txt"), []byte("host"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(cwd)

	out := &bytes.Buffer{}
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	ext := &ExtConfig{callback: func(conf *engine.Config, _ ExtFlags) error {
		conf.Writer = out
		conf.ErrorWriter = out
		return nil
	}}
	code := "const fs = require('fs'); console.println(fs.existsSync('/work/cwd-marker.txt'), fs.readdirSync('/work').filter(n => n !== '.' && n !== '..').length)"
	runtime, exitCode, err := entry(flags, []string{"jsh"}, append(args, "-C", code), ext)
	if err != nil || exitCode != 0 {
		t.Fatalf("entry() = (_, %d, %v)", exitCode, err)
	}
	lib.Enable(runtime)
	if rc := runtime.Main(); rc != 0 {
		t.Fatalf("Main() = %d, output %q", rc, out.String())
	}
	return strings.TrimSpace(out.String())
}

func TestEntryWorkFallsBackToCurrentDirectory(t *testing.T) {
	if got := runEntryWorkProbe(t, nil); got != "true 1" {
		t.Fatalf("standalone /work output = %q, want %q", got, "true 1")
	}
}

func TestEntryEmptyWorkSourceMountsVirtualDirectory(t *testing.T) {
	if got := runEntryWorkProbe(t, []string{"-v", "/work="}); got != "false 0" {
		t.Fatalf("virtual /work output = %q, want %q", got, "false 0")
	}
}

func TestFSTabsEmptySourceRoundTrip(t *testing.T) {
	var tabs engine.FSTabs
	if err := tabs.Set("/work="); err != nil {
		t.Fatalf("Set(/work=) error = %v", err)
	}
	if err := tabs.Set("=/tmp"); err == nil {
		t.Fatal("Set(=/tmp) error = nil, want invalid mount option")
	}
	tabs = append(tabs, engine.FSTab{MountPoint: "/fsonly", FS: engine.NewVirtualFS()})
	data, err := tabs.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `[{"mountPoint":"/work","source":""}]` {
		t.Fatalf("MarshalJSON() = %s", data)
	}
	var decoded engine.FSTabs
	if err := decoded.UnmarshalJSON(data); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 1 || decoded[0].MountPoint != "/work" || decoded[0].Source != "" || decoded[0].FS != nil {
		t.Fatalf("UnmarshalJSON() = %#v", decoded)
	}
}
