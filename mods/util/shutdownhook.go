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
	slices.Reverse(shutdownHooks)
	for _, f := range shutdownHooks {
		f()
	}
	shutdownHooks = nil
	shutdownHooksMutex.Unlock()
}
