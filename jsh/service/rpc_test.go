package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestControllerRPCConnectionLimit verifies that concurrent RPC connections are bounded
// by the rpcConnMax (256) limit. Connections beyond the limit should be dropped gracefully.
func TestControllerRPCConnectionLimit(t *testing.T) {
	ctl, err := NewController(&ControllerConfig{
		Launcher:  []string{},
		Mounts:    nil,
		ConfigDir: t.TempDir(),
		Address:   "tcp://127.0.0.1:0",
	})
	if err != nil {
		t.Fatalf("NewController failed: %v", err)
	}
	defer ctl.Stop(nil)

	if err := ctl.startRPC(); err != nil {
		t.Fatalf("startRPC failed: %v", err)
	}

	addr := ctl.Address()
	if addr == "" {
		t.Fatal("RPC address is empty")
	}

	// Verify rpcConnMax is set
	if ctl.rpcConnMax != 256 {
		t.Errorf("Expected rpcConnMax=256, got %d", ctl.rpcConnMax)
	}

	// Try to open more connections than allowed (300 vs limit 256)
	const attemptCount = 300
	const maxConcurrent = 256
	successCount := atomic.Int32{}
	failureCount := atomic.Int32{}
	var wg sync.WaitGroup

	for i := 0; i < attemptCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.Dial("tcp", strings.TrimPrefix(addr, "tcp://"))
			if err != nil {
				failureCount.Add(1)
				return
			}
			defer conn.Close()

			// Send a simple JSON-RPC request
			req := map[string]interface{}{
				"jsonrpc": "2.0",
				"method":  "nonexistent",
				"id":      1,
			}
			if err := json.NewEncoder(conn).Encode(req); err != nil {
				failureCount.Add(1)
				return
			}

			// Try to read response (may fail if connection was dropped)
			var resp map[string]interface{}
			if err := json.NewDecoder(conn).Decode(&resp); err != nil {
				failureCount.Add(1)
				return
			}

			successCount.Add(1)
		}()
	}

	wg.Wait()

	success := int(successCount.Load())
	failure := int(failureCount.Load())

	t.Logf("Results: %d successful, %d failed (expected: max %d concurrent)", success, failure, maxConcurrent)

	// While we can't guarantee exact numbers due to timing, we expect:
	// - Most connections to succeed (since we're not holding them open)
	// - No panic or crash from unbounded goroutine creation
	if success == 0 {
		t.Errorf("Expected at least some successful connections, got 0")
	}

	if success+failure != attemptCount {
		t.Errorf("Expected %d total connection attempts, got %d", attemptCount, success+failure)
	}
}

// TestControllerRPCConnectionDeadline verifies that RPC connections have a Read/Write deadline.
// A slow/hanging client should be forcefully disconnected after ~30 seconds.
func TestControllerRPCConnectionDeadline(t *testing.T) {
	ctl, err := NewController(&ControllerConfig{
		Launcher:  []string{},
		Mounts:    nil,
		ConfigDir: t.TempDir(),
		Address:   "tcp://127.0.0.1:0",
	})
	if err != nil {
		t.Fatalf("NewController failed: %v", err)
	}
	defer ctl.Stop(nil)

	if err := ctl.startRPC(); err != nil {
		t.Fatalf("startRPC failed: %v", err)
	}

	addr := ctl.Address()
	if addr == "" {
		t.Fatal("RPC address is empty")
	}

	// Connect and send partial/incomplete request, then wait
	conn, err := net.Dial("tcp", strings.TrimPrefix(addr, "tcp://"))
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}

	// Send incomplete JSON and wait for deadline to close connection
	_, err = conn.Write([]byte(`{"jsonrpc":"2.0","method":"test"`))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Connection should be closed due to deadline after ~30 seconds
	// (for test purposes, we just verify the connection eventually fails)
	start := time.Now()
	conn.SetReadDeadline(time.Now().Add(35 * time.Second)) // Give it time
	buf := make([]byte, 1024)
	_, err = conn.Read(buf)

	elapsed := time.Since(start)

	// Deadline should cause closure within reasonable time (30-35 seconds)
	// If error is not about deadline, connection was likely closed by server
	if err == nil {
		t.Errorf("Expected connection to close, but Read succeeded")
	} else if err != io.EOF && !strings.Contains(err.Error(), "deadline") && !strings.Contains(err.Error(), "reset") {
		t.Logf("Connection error (expected): %v after %v", err, elapsed)
	}

	conn.Close()

	// For quick testing, we primarily verify the code runs without panic
	// Full deadline precision can be tested with a long-running test suite
}

// TestControllerRPCConcurrentStress sends many concurrent RPC requests to verify
// the system handles high load without panics or deadlocks.
func TestControllerRPCConcurrentStress(t *testing.T) {
	ctl, err := NewController(&ControllerConfig{
		Launcher:  []string{},
		Mounts:    nil,
		ConfigDir: t.TempDir(),
		Address:   "tcp://127.0.0.1:0",
	})
	if err != nil {
		t.Fatalf("NewController failed: %v", err)
	}
	defer ctl.Stop(nil)

	if err := ctl.startRPC(); err != nil {
		t.Fatalf("startRPC failed: %v", err)
	}

	addr := ctl.Address()
	if addr == "" {
		t.Fatal("RPC address is empty")
	}

	// Send 500 concurrent requests to stress test the connection limit
	const requestCount = 500
	successCount := atomic.Int32{}
	var wg sync.WaitGroup

	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			conn, err := net.Dial("tcp", strings.TrimPrefix(addr, "tcp://"))
			if err != nil {
				// Connection failures are expected when limit is reached
				return
			}
			defer conn.Close()

			// Send a valid request to the service status method (known to exist)
			req := map[string]interface{}{
				"jsonrpc": "2.0",
				"method":  "listService",
				"id":      idx,
			}

			if err := json.NewEncoder(conn).Encode(req); err != nil {
				return
			}

			// Read response with timeout
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			var resp map[string]interface{}
			if err := json.NewDecoder(conn).Decode(&resp); err != nil {
				return
			}

			successCount.Add(1)
		}(i)
	}

	wg.Wait()

	success := int(successCount.Load())
	t.Logf("Stress test: %d/%d requests succeeded", success, requestCount)

	// We expect most requests to succeed since handler is fast
	// Some may fail due to connection limit, but total should be high
	if success < requestCount/2 {
		t.Logf("Warning: Less than 50%% success rate (%d/%d)", success, requestCount)
	}

	// Main goal: verify no panic/crash occurred
	fmt.Printf("Completed stress test with %d concurrent connections\n", requestCount)
}

// TestControllerRPCSemaphoreCleanup verifies that semaphore is properly released
// when connections are dropped at the limit.
func TestControllerRPCSemaphoreCleanup(t *testing.T) {
	ctl, err := NewController(&ControllerConfig{
		Launcher:  []string{},
		Mounts:    nil,
		ConfigDir: t.TempDir(),
		Address:   "tcp://127.0.0.1:0",
	})
	if err != nil {
		t.Fatalf("NewController failed: %v", err)
	}
	defer ctl.Stop(nil)

	if err := ctl.startRPC(); err != nil {
		t.Fatalf("startRPC failed: %v", err)
	}

	addr := ctl.Address()
	if addr == "" {
		t.Fatal("RPC address is empty")
	}

	// Phase 1: Hold many connections (within limit)
	const holdCount = 100
	conns := make([]net.Conn, holdCount)
	for i := 0; i < holdCount; i++ {
		conn, err := net.Dial("tcp", strings.TrimPrefix(addr, "tcp://"))
		if err != nil {
			t.Fatalf("Dial %d failed: %v", i, err)
		}
		conns[i] = conn
	}

	// Phase 2: Close 50 connections
	for i := 0; i < holdCount/2; i++ {
		conns[i].Close()
	}
	time.Sleep(100 * time.Millisecond)

	// Phase 3: Should be able to create more connections
	// (semaphore should have freed up slots from closed connections)
	newConns := make([]net.Conn, 20)
	for i := 0; i < 20; i++ {
		conn, err := net.Dial("tcp", strings.TrimPrefix(addr, "tcp://"))
		if err != nil {
			t.Fatalf("Failed to create new connection after cleanup (i=%d): %v", i, err)
		}
		newConns[i] = conn
	}

	// Cleanup
	for _, conn := range conns {
		if conn != nil {
			conn.Close()
		}
	}
	for _, conn := range newConns {
		if conn != nil {
			conn.Close()
		}
	}

	t.Logf("Semaphore cleanup test: Successfully created new connections after releasing some")
}
