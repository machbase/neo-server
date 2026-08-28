package usrlib_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/lib"
	"github.com/machbase/neo-server/v8/jsh/root"
)

// fakeSessionCalls records invocations made against the stubbed `@jsh/session`
// native module so tests can assert on them without a real HTTP/mach server.
type fakeSessionCalls struct {
	switchUser    [][2]string
	reconnect     [][3]string
	useDatabase   []string
	switchUserErr string
	reconnectErr  string
}

func registerFakeSession(jr *engine.JSRuntime, calls *fakeSessionCalls) {
	jr.RegisterNativeModule("@jsh/session", func(_ context.Context, rt *goja.Runtime, module *goja.Object) {
		exports := module.Get("exports").(*goja.Object)
		exports.Set("switchUser", func(user, password string) error {
			calls.switchUser = append(calls.switchUser, [2]string{user, password})
			if calls.switchUserErr != "" {
				return errors.New(calls.switchUserErr)
			}
			return nil
		})
		exports.Set("reconnect", func(server, user, password string) error {
			calls.reconnect = append(calls.reconnect, [3]string{server, user, password})
			if calls.reconnectErr != "" {
				return errors.New(calls.reconnectErr)
			}
			return nil
		})
		exports.Set("useDatabase", func(name string) {
			calls.useDatabase = append(calls.useDatabase, name)
		})
	})
}

func runContextCommandsScript(t *testing.T, script string, calls *fakeSessionCalls) string {
	t.Helper()
	var stdout bytes.Buffer
	jr, err := engine.New(engine.Config{
		Name: "context_cmds-test",
		Code: script,
		FSTabs: []engine.FSTab{
			root.RootFSTab(),
		},
		Env:    map[string]any{},
		Writer: &stdout,
		Reader: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("engine.New() error = %v", err)
	}
	lib.Enable(jr)
	registerFakeSession(jr, calls)

	if err := jr.Run(); err != nil {
		t.Fatalf("jr.Run() error = %v, output = %s", err, stdout.String())
	}
	return stdout.String()
}

func TestContextCommandsTryHandleUnrelatedCommandReturnsNull(t *testing.T) {
	calls := &fakeSessionCalls{}
	out := runContextCommandsScript(t, `
		const contextCommands = require('/usr/lib/context_cmds');
		const store = {};
		const env = { get(k) { return store[k]; }, set(k, v) { store[k] = v; } };
		const result = contextCommands.tryHandle(['sql', 'select 1'], env);
		console.println('result:', result);
	`, calls)

	if out != "result: null\n" {
		t.Fatalf("output = %q, want %q", out, "result: null\n")
	}
	if len(calls.switchUser) != 0 || len(calls.reconnect) != 0 || len(calls.useDatabase) != 0 {
		t.Fatalf("unexpected session calls: %+v", calls)
	}
}

func TestContextCommandsUseSetsDatabase(t *testing.T) {
	calls := &fakeSessionCalls{}
	out := runContextCommandsScript(t, `
		const contextCommands = require('/usr/lib/context_cmds');
		const store = {};
		const env = { get(k) { return store[k]; }, set(k, v) { store[k] = v; } };
		const exitCode = contextCommands.tryHandle(['use', 'DATABASE_A'], env);
		console.println('exitCode:', exitCode, 'database:', env.get('NEOSHELL_DATABASE'));
	`, calls)

	if out != "exitCode: 0 database: DATABASE_A\n" {
		t.Fatalf("output = %q", out)
	}
	if len(calls.useDatabase) != 1 || calls.useDatabase[0] != "DATABASE_A" {
		t.Fatalf("useDatabase calls = %+v, want [DATABASE_A]", calls.useDatabase)
	}
}

func TestContextCommandsConnectUserPasswordOnlySwitchesUser(t *testing.T) {
	calls := &fakeSessionCalls{}
	out := runContextCommandsScript(t, `
		const contextCommands = require('/usr/lib/context_cmds');
		const store = { NEOSHELL_HOST: 'oldhost:5654', NEOSHELL_USER: 'sys', NEOSHELL_PASSWORD: 'manager' };
		const env = { get(k) { return store[k]; }, set(k, v) { store[k] = v; } };
		const exitCode = contextCommands.tryHandle(['connect', 'demo/secret'], env);
		console.println('exitCode:', exitCode, 'user:', env.get('NEOSHELL_USER'), 'password:', env.get('NEOSHELL_PASSWORD'), 'host:', env.get('NEOSHELL_HOST'));
	`, calls)

	if out != "exitCode: 0 user: demo password: secret host: oldhost:5654\n" {
		t.Fatalf("output = %q", out)
	}
	if len(calls.switchUser) != 1 || calls.switchUser[0] != [2]string{"demo", "secret"} {
		t.Fatalf("switchUser calls = %+v, want [[demo secret]]", calls.switchUser)
	}
	if len(calls.reconnect) != 0 {
		t.Fatalf("reconnect should not be called for a user/password-only connect, got %+v", calls.reconnect)
	}
}

func TestContextCommandsConnectHostChangeReconnects(t *testing.T) {
	calls := &fakeSessionCalls{}
	out := runContextCommandsScript(t, `
		const contextCommands = require('/usr/lib/context_cmds');
		const store = { NEOSHELL_HOST: 'oldhost:5654', NEOSHELL_USER: 'sys', NEOSHELL_PASSWORD: 'manager' };
		const env = { get(k) { return store[k]; }, set(k, v) { store[k] = v; } };
		const exitCode = contextCommands.tryHandle(['connect', 'demo2:secret2@newhost:5656'], env);
		console.println('exitCode:', exitCode, 'user:', env.get('NEOSHELL_USER'), 'password:', env.get('NEOSHELL_PASSWORD'), 'host:', env.get('NEOSHELL_HOST'));
	`, calls)

	if out != "exitCode: 0 user: demo2 password: secret2 host: newhost:5656\n" {
		t.Fatalf("output = %q", out)
	}
	if len(calls.reconnect) != 1 || calls.reconnect[0] != [3]string{"newhost:5656", "demo2", "secret2"} {
		t.Fatalf("reconnect calls = %+v, want [[newhost:5656 demo2 secret2]]", calls.reconnect)
	}
	if len(calls.switchUser) != 0 {
		t.Fatalf("switchUser should not be called for a host-changing connect, got %+v", calls.switchUser)
	}
}

func TestContextCommandsConnectFailurePropagatesExitCode(t *testing.T) {
	calls := &fakeSessionCalls{switchUserErr: "user or password is incorrect"}
	out := runContextCommandsScript(t, `
		const contextCommands = require('/usr/lib/context_cmds');
		const store = { NEOSHELL_HOST: 'oldhost:5654', NEOSHELL_USER: 'sys', NEOSHELL_PASSWORD: 'manager' };
		const env = { get(k) { return store[k]; }, set(k, v) { store[k] = v; } };
		const exitCode = contextCommands.tryHandle(['connect', 'demo/wrong'], env);
		console.println('exitCode:', exitCode, 'user:', env.get('NEOSHELL_USER'));
	`, calls)

	if out != "Error: failed to connect: user or password is incorrect\nexitCode: 1 user: sys\n" {
		t.Fatalf("output = %q, want unchanged NEOSHELL_USER and exitCode 1", out)
	}
}

func TestContextCommandsPrintHelp(t *testing.T) {
	calls := &fakeSessionCalls{}
	out := runContextCommandsScript(t, `
		const contextCommands = require('/usr/lib/context_cmds');
		console.println('connect:', contextCommands.printHelp('connect'));
		console.println('use:', contextCommands.printHelp('USE'));
		console.println('sql:', contextCommands.printHelp('sql'));
	`, calls)

	want := "Usage: connect [options] [user:password@]host[:port]\n" +
		"connect: true\n" +
		"Usage: use <database>\n" +
		"use: true\n" +
		"sql: false\n"
	if out != want {
		t.Fatalf("output = %q, want %q", out, want)
	}
}

func TestContextCommandsDescribeAll(t *testing.T) {
	calls := &fakeSessionCalls{}
	out := runContextCommandsScript(t, `
		const contextCommands = require('/usr/lib/context_cmds');
		console.println(JSON.stringify(contextCommands.describeAll()));
	`, calls)

	want := `[{"name":"connect","description":"Connect to a database"},{"name":"use","description":"Select the current database"}]` + "\n"
	if out != want {
		t.Fatalf("output = %q, want %q", out, want)
	}
}
