//go:build darwin
// +build darwin

package native

/*
#include <stdlib.h>
#include <stdio.h>
#include <signal.h>

sigset_t machengine_mask;

static void inline cliDarwinIgonreSignalHandler() {
}

static void inline cliDarwinDisableSignalHandler() {
	signal(SIGURG, cliDarwinIgonreSignalHandler);

	// TODO: 'sigprocmask' is not working to prevent SIGURG signal
	//
	// sigemptyset(&machengine_mask);;
	// sigaddset(&machengine_mask, SIGURG);
	// sigprocmask(SIG_BLOCK, &machengine_mask, NULL);
}

static void inline cliDarwinEnableSignalHandler() {
	sigprocmask(SIG_UNBLOCK, &machengine_mask, NULL);
}
*/
import "C"

func InitSignalHandler() {
	C.cliDarwinDisableSignalHandler()
}

func DeinitSignalHandler() {
	C.cliDarwinEnableSignalHandler()
}
