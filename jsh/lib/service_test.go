package lib_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/service"
	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestServiceModule(t *testing.T) {
	tmpDir := t.TempDir()
	servicesDir := filepath.Join(tmpDir, "services")
	if err := os.MkdirAll(servicesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	config := service.Config{Name: "alpha", Enable: false, Executable: "echo", Args: []string{"hello"}}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(servicesDir, "alpha.json"), data, 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	ctl, err := service.NewController(&service.ControllerConfig{
		ConfigDir: "/work/services",
		Mounts: []engine.FSTab{
			{MountPoint: "/work", FS: os.DirFS(tmpDir)},
		},
	})
	if err != nil {
		t.Fatalf("NewController() error: %v", err)
	}
	if err := ctl.Start(nil); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer ctl.Stop(nil)

	test_engine.RunTest(t, test_engine.TestCase{
		Name: "service_module_details",
		Vars: map[string]any{
			"SERVICE_CONTROLLER": ctl.Address(),
		},
		Script: `
			const service = require('service');

			const client = new service.Client({ timeout: 1000 });
			const options = { timeout: 1000 };

			client.details.get('alpha', (err, runtime) => {
				if (err) {
					console.println('ERR get', err.message);
					return;
				}
				console.println('details.initial', JSON.stringify(runtime.details || {}));

				client.details.set('alpha', 'health', 'ok', (err, updated) => {
					if (err) {
						console.println('ERR set', err.message);
						return;
					}
					console.println('details.afterSet', JSON.stringify(updated.details || {}));

					service.details.get('alpha', 'health', options, (err, selected) => {
						if (err) {
							console.println('ERR getkey', err.message);
							return;
						}
						console.println('details.selected', JSON.stringify(selected.details || {}));

						service.call('service.runtime.detail.delete', { name: 'alpha', key: 'health' }, options, (err, deleted) => {
							if (err) {
								console.println('ERR delete', err.message);
								return;
							}
							console.println('details.afterDelete', JSON.stringify(deleted.details || {}));
						});
					});
				});
			});
		`,
		Output: []string{
			"details.initial {}",
			"details.afterSet {\"health\":\"ok\"}",
			"details.selected {\"health\":\"ok\"}",
			"details.afterDelete {}",
		},
	})
}

func TestServiceModuleCommandHelpers(t *testing.T) {
	getCalls := 0
	addr, shutdown := startMockServiceModuleRPCServer(t, func(req serviceModuleRPCRequest) any {
		switch req.Method {
		case "service.get":
			getCalls++
			if getCalls == 1 {
				return map[string]any{
					"config": map[string]any{"name": "alpha", "enable": false, "executable": "echo", "args": []any{"hello"}},
					"status": "stopped",
				}
			}
			return map[string]any{
				"config": map[string]any{"name": "alpha", "enable": true, "executable": "echo", "args": []any{"world"}},
				"status": "running",
			}
		case "service.list":
			return []map[string]any{
				{"config": map[string]any{"name": "alpha", "enable": false, "executable": "echo"}, "status": "stopped"},
			}
		case "service.install":
			return map[string]any{
				"config": map[string]any{"name": "beta", "enable": false, "executable": "echo", "args": []any{"beta"}},
				"status": "stopped",
			}
		case "service.start":
			return map[string]any{
				"config": map[string]any{"name": "beta", "enable": false, "executable": "echo", "args": []any{"beta"}},
				"status": "running",
			}
		case "service.stop":
			return map[string]any{
				"config": map[string]any{"name": "beta", "enable": false, "executable": "echo", "args": []any{"beta"}},
				"status": "stopped",
			}
		case "service.read":
			return map[string]any{
				"updated": []any{map[string]any{"name": "alpha"}},
			}
		case "service.reload":
			return map[string]any{
				"actions": []any{map[string]any{"name": "alpha", "action": "RELOAD start"}},
			}
		case "service.update":
			return map[string]any{
				"actions": []any{map[string]any{"name": "alpha", "action": "UPDATE stop"}},
			}
		case "service.uninstall":
			return true
		default:
			t.Fatalf("unexpected RPC method %q", req.Method)
			return nil
		}
	})
	defer shutdown()

	test_engine.RunTest(t, test_engine.TestCase{
		Name: "service_module_commands",
		Vars: map[string]any{
			"SERVICE_CONTROLLER": addr,
		},
		Script: `
			const service = require('service');

			const options = { timeout: 1000 };
			const client = new service.Client(options);

			client.commands.status('alpha', (err, alpha) => {
				if (err) {
					console.println('ERR status.initial', err.message);
					return;
				}
				console.println('status.initial', alpha.status, alpha.config.args.join(','));

				service.commands.status(options, (err, services) => {
					if (err) {
						console.println('ERR status.count', err.message);
						return;
					}
					console.println('status.count', services.length);

					service.commands.install({ name: 'beta', enable: false, executable: 'echo', args: ['beta'] }, options, (err, beta) => {
						if (err) {
							console.println('ERR install.beta', err.message);
							return;
						}
						console.println('install.beta', beta.config.name, beta.status);

						service.commands.start('beta', options, (err, started) => {
							if (err) {
								console.println('ERR start.beta', err.message);
								return;
							}
							console.println('start.beta', started.status);

							client.commands.stop('beta', (err, stopped) => {
								if (err) {
									console.println('ERR stop.beta', err.message);
									return;
								}
								console.println('stop.beta', stopped.status);

								service.commands.read(options, (err, reread) => {
									if (err) {
										console.println('ERR read.updated', err.message);
										return;
									}
									console.println('read.updated', reread.updated.length, reread.updated[0].name);

									service.commands.reload(options, (err, reloaded) => {
										if (err) {
											console.println('ERR reload.first', err.message);
											return;
										}
										console.println('reload.first', reloaded.actions[0].action);

										service.commands.status('alpha', options, (err, alphaReloaded) => {
											if (err) {
												console.println('ERR status.reloaded', err.message);
												return;
											}
											console.println('status.named', alphaReloaded.status, alphaReloaded.config.args.join(','));

											client.commands.update((err, updated) => {
												if (err) {
													console.println('ERR update.first', err.message);
													return;
												}
												console.println('update.first', updated.actions[0].action);

												service.commands.uninstall('beta', options, (err, removed) => {
													if (err) {
														console.println('ERR uninstall.beta', err.message);
														return;
													}
													console.println('uninstall.beta', removed);
												});
											});
										});
									});
								});
							});
						});
					});
				});
			});
		`,
		Output: []string{
			"status.initial stopped hello",
			"status.count 1",
			"install.beta beta stopped",
			"start.beta running",
			"stop.beta stopped",
			"read.updated 1 alpha",
			"reload.first RELOAD start",
			"status.named running world",
			"update.first UPDATE stop",
			"uninstall.beta true",
		},
	})
}

type serviceModuleRPCRequest struct {
	Version string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

func startMockServiceModuleRPCServer(t *testing.T, handler func(serviceModuleRPCRequest) any) (string, func()) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-stop:
					return
				default:
					t.Errorf("accept: %v", err)
					return
				}
			}

			wg.Add(1)
			go func(conn net.Conn) {
				defer wg.Done()
				defer conn.Close()

				reader := bufio.NewReader(conn)
				line, err := reader.ReadBytes('\n')
				if err != nil {
					t.Errorf("read request: %v", err)
					return
				}

				var req serviceModuleRPCRequest
				if err := json.Unmarshal(line, &req); err != nil {
					t.Errorf("unmarshal request: %v", err)
					return
				}

				resp := map[string]any{
					"jsonrpc": "2.0",
					"id":      json.RawMessage("1"),
					"result":  handler(req),
				}
				if len(req.ID) > 0 {
					resp["id"] = req.ID
				}
				if err := json.NewEncoder(conn).Encode(resp); err != nil {
					t.Errorf("encode response: %v", err)
				}
			}(conn)
		}
	}()

	return fmt.Sprintf("tcp://%s", ln.Addr().String()), func() {
		close(stop)
		_ = ln.Close()
		wg.Wait()
	}
}
