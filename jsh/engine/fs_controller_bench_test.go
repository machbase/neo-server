package engine

import (
	"io/fs"
	"testing"
)

// BenchmarkControllerFS_SequentialOperations benchmarks sequential RPC operations
// with connection pooling enabled. This demonstrates the performance improvement
// from reusing a single persistent connection instead of creating a new one per operation.
func BenchmarkControllerFS_SequentialOperations(b *testing.B) {
	address, shutdown := startMockControllerFSServer(&testing.T{})
	defer shutdown()

	remoteFS, err := NewControllerFS(address)
	if err != nil {
		b.Fatalf("NewControllerFS() error: %v", err)
	}
	defer remoteFS.Close()

	mfs := NewFS()
	if err := mfs.Mount("/shared", remoteFS); err != nil {
		b.Fatalf("Mount() error: %v", err)
	}

	b.ResetTimer()

	// Each iteration performs a sequence of RPC operations
	for i := 0; i < b.N; i++ {
		// Create a unique path for this iteration
		path := "/shared/bench_" + string(rune('a'+i%26)) + "_file.txt"

		// Perform sequential operations on the same connection
		if err := mfs.WriteFile(path, []byte("benchmark data")); err != nil {
			b.Fatalf("WriteFile() error: %v", err)
		}
		if _, err := fs.ReadFile(mfs, path); err != nil {
			b.Fatalf("ReadFile() error: %v", err)
		}
		if _, err := fs.Stat(mfs, path); err != nil {
			b.Fatalf("Stat() error: %v", err)
		}
		if err := mfs.Chmod(path, 0o644); err != nil {
			b.Fatalf("Chmod() error: %v", err)
		}
		if err := mfs.Remove(path); err != nil {
			b.Fatalf("Remove() error: %v", err)
		}
	}
}

// BenchmarkControllerFS_MultipleOperations_Many benchmarks many sequential operations
// to better show the connection reuse benefit at scale.
func BenchmarkControllerFS_MultipleOperations_Many(b *testing.B) {
	address, shutdown := startMockControllerFSServer(&testing.T{})
	defer shutdown()

	remoteFS, err := NewControllerFS(address)
	if err != nil {
		b.Fatalf("NewControllerFS() error: %v", err)
	}
	defer remoteFS.Close()

	mfs := NewFS()
	if err := mfs.Mount("/shared", remoteFS); err != nil {
		b.Fatalf("Mount() error: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Base operation count to show batching benefits
		for j := 0; j < 10; j++ {
			path := "/shared/many_file.txt"
			if err := mfs.WriteFile(path, []byte("data")); err != nil {
				b.Fatalf("WriteFile() error: %v", err)
			}
			if _, err := fs.ReadFile(mfs, path); err != nil {
				b.Fatalf("ReadFile() error: %v", err)
			}
			if err := mfs.Remove(path); err != nil {
				b.Fatalf("Remove() error: %v", err)
			}
		}
	}
}
