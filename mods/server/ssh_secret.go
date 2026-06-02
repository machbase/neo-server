package server

import (
	"fmt"
	"time"

	"github.com/machbase/neo-server/v8/jsh/service"
)

func (svr *sshd) appendNeoShellSecretEnv(shell *SshShell, keyValues ...string) func() {
	if shell == nil {
		return func() {}
	}
	items := make([]service.SecretItem, 0, len(keyValues)/2)
	filterNames := []string{service.SecretRefEnv}
	for i := 0; i+1 < len(keyValues); i += 2 {
		key := keyValues[i]
		if key == "" {
			continue
		}
		items = append(items, service.SecretItem{Key: key, Value: keyValues[i+1]})
		filterNames = append(filterNames, key)
	}
	shell.Envs = filterSecretEnv(shell.Envs, filterNames...)
	if len(items) == 0 {
		return func() {}
	}
	if svr.authServer != nil && svr.authServer.serviceController != nil {
		token, err := svr.authServer.serviceController.PutSecret(items, 30*time.Second)
		if err == nil {
			shell.Envs = append(shell.Envs, fmt.Sprintf("%s=%s", service.SecretRefEnv, token))
			return func() {
				svr.authServer.serviceController.RevokeSecret(token)
			}
		}
		if svr.log != nil {
			svr.log.Warnf("neo shell secret broker unavailable: %s", err.Error())
		}
	}

	for _, item := range items {
		shell.Envs = append(shell.Envs, fmt.Sprintf("%s=%s", item.Key, item.Value))
	}
	return func() {}
}

func filterSecretEnv(envs []string, names ...string) []string {
	filtered := envs[:0]
	for _, env := range envs {
		if hasAnyEnvName(env, names...) {
			continue
		}
		filtered = append(filtered, env)
	}
	return filtered
}

func hasAnyEnvName(env string, names ...string) bool {
	for _, name := range names {
		if hasEnvName(env, name) {
			return true
		}
	}
	return false
}

func hasEnvName(env string, name string) bool {
	return len(env) > len(name) && env[:len(name)] == name && env[len(name)] == '='
}
