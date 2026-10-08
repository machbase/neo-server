package session

import (
	"errors"
	"flag"
	"io"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
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
