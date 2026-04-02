package engine

import "testing"

func TestRunShutdownHooksOnce(t *testing.T) {
	jr := &JSRuntime{}
	count := 0
	jr.AddShutdownHook(func() {
		count++
	})
	jr.AddShutdownHook(func() {
		count++
	})

	jr.runShutdownHooks()
	jr.runShutdownHooks()

	if count != 2 {
		t.Fatalf("shutdown hooks ran %d times, want 2", count)
	}
}

func TestRunShutdownHooksRecoverAndContinue(t *testing.T) {
	jr := &JSRuntime{}
	var order []string
	jr.AddShutdownHook(func() {
		order = append(order, "third")
	})
	jr.AddShutdownHook(func() {
		order = append(order, "second")
		panic("hook failed")
	})
	jr.AddShutdownHook(func() {
		order = append(order, "first")
	})

	jr.runShutdownHooks()

	if len(order) != 3 {
		t.Fatalf("shutdown hooks ran %d times, want 3", len(order))
	}
	if order[0] != "first" || order[1] != "second" || order[2] != "third" {
		t.Fatalf("unexpected shutdown order: %#v", order)
	}
}
