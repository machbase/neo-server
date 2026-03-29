//go:build !windows

package test

import (
	"net"
	"os"
	"reflect"
	"testing"
)

func TestCanConnectDockerSocketReturnsFalseWhenMissing(t *testing.T) {
	path := newSocketPath(t)
	if canConnectDockerSocket(path) {
		t.Fatalf("expected missing socket to be reported as unavailable")
	}
}

func TestCanConnectDockerSocketReturnsFalseWhenSocketIsNotListening(t *testing.T) {
	path := newSocketPath(t)

	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen unix socket: %v", err)
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("close unix socket: %v", err)
	}
	defer os.Remove(path)

	if canConnectDockerSocket(path) {
		t.Fatalf("expected closed socket to be reported as unavailable")
	}
}

func TestCanConnectDockerSocketReturnsTrueWhenReachable(t *testing.T) {
	path := newSocketPath(t)

	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen unix socket: %v", err)
	}
	defer listener.Close()
	defer os.Remove(path)

	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		conn, err := listener.Accept()
		if err == nil {
			_ = conn.Close()
		}
	}()

	if !canConnectDockerSocket(path) {
		t.Fatalf("expected listening socket to be reported as available")
	}

	<-acceptDone
}

func TestFirstReachableDockerSocketReturnsFirstReachablePath(t *testing.T) {
	missingPath := newSocketPath(t)
	reachablePath := newSocketPath(t)

	listener, err := net.Listen("unix", reachablePath)
	if err != nil {
		t.Fatalf("listen unix socket: %v", err)
	}
	defer listener.Close()
	defer os.Remove(reachablePath)

	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		conn, err := listener.Accept()
		if err == nil {
			_ = conn.Close()
		}
	}()

	path, ok := firstReachableDockerSocket([]string{missingPath, reachablePath})
	if !ok {
		t.Fatalf("expected reachable socket to be found")
	}
	if path != reachablePath {
		t.Fatalf("expected %q, got %q", reachablePath, path)
	}

	<-acceptDone
}

func TestDockerHostSocketPathParsesUnixSocket(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///tmp/docker.sock")

	path, ok := dockerHostSocketPath()
	if !ok {
		t.Fatalf("expected unix docker host to be parsed")
	}
	if path != "/tmp/docker.sock" {
		t.Fatalf("expected /tmp/docker.sock, got %q", path)
	}
}

func TestLinuxDockerSocketPathsPrefersDockerHostAndDeduplicates(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")

	paths := linuxDockerSocketPaths()
	expected := []string{"/var/run/docker.sock"}
	if !reflect.DeepEqual(paths, expected) {
		t.Fatalf("expected %v, got %v", expected, paths)
	}
}

func TestDockerTestsEnabledInCIDefaultsToFalse(t *testing.T) {
	t.Setenv("CI", "true")
	t.Setenv(EnableDockerTestsEnv, "")

	if dockerTestsEnabledInCI() {
		t.Fatalf("expected docker tests to be disabled in CI by default")
	}
}

func TestDockerTestsEnabledInCIHonorsOptIn(t *testing.T) {
	t.Setenv("CI", "true")
	t.Setenv(EnableDockerTestsEnv, "true")

	if !dockerTestsEnabledInCI() {
		t.Fatalf("expected docker tests to be enabled in CI when opted in")
	}
}

func TestDockerImageUsesDefaultsWhenEnvNotSet(t *testing.T) {
	repository, tag := PostgresDockerImage.Resolve()
	if repository != "postgres" || tag != "16" {
		t.Fatalf("expected default postgres:16, got %s:%s", repository, tag)
	}
}

func TestDockerImageUsesEnvironmentOverrides(t *testing.T) {
	t.Setenv("TEST_POSTGRES_IMAGE_REPOSITORY", "ghcr.io/machbase/postgres")
	t.Setenv("TEST_POSTGRES_IMAGE_TAG", "16-ci")

	repository, tag := PostgresDockerImage.Resolve()
	if repository != "ghcr.io/machbase/postgres" || tag != "16-ci" {
		t.Fatalf("expected overridden image ghcr.io/machbase/postgres:16-ci, got %s:%s", repository, tag)
	}
}

func newSocketPath(t *testing.T) string {
	t.Helper()

	file, err := os.CreateTemp("/tmp", "dockertest-*.sock")
	if err != nil {
		t.Fatalf("create temp socket path: %v", err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		t.Fatalf("close temp socket path: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove temp socket placeholder: %v", err)
	}

	return path
}
