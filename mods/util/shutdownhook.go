package util

import (
	"slices"
	"sync"
)

var shutdownHooks []func()
var shutdownHooksMutex sync.Mutex

func AddShutdownHook(f func()) {
	shutdownHooksMutex.Lock()
	shutdownHooks = append(shutdownHooks, f)
	shutdownHooksMutex.Unlock()
}

func RunShutdownHooks() {
	shutdownHooksMutex.Lock()
	hooks := shutdownHooks
	shutdownHooks = nil
	slices.Reverse(hooks)
	shutdownHooksMutex.Unlock()

	for _, f := range hooks {
		f()
	}
}
