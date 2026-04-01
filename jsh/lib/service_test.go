package lib_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
	jshservice "github.com/machbase/neo-server/v8/jsh/service"
	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestServiceModule(t *testing.T) {
	tmpDir := t.TempDir()
	servicesDir := filepath.Join(tmpDir, "services")
	if err := os.MkdirAll(servicesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	config := jshservice.Config{Name: "alpha", Enable: false, Executable: "echo"}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(servicesDir, "alpha.json"), data, 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	ctl, err := jshservice.NewController(&jshservice.ControllerConfig{
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

			const client = service.createClient({ timeout: 1000 });
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
