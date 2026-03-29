package test

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const dockerSocketDialTimeout = time.Second

const EnableDockerTestsEnv = "ENABLE_DOCKER_TESTS"

type DockerImageConfig struct {
	RepositoryEnvVar  string
	TagEnvVar         string
	DefaultRepository string
	DefaultTag        string
}

var (
	PostgresDockerImage = DockerImageConfig{
		RepositoryEnvVar:  "TEST_POSTGRES_IMAGE_REPOSITORY",
		TagEnvVar:         "TEST_POSTGRES_IMAGE_TAG",
		DefaultRepository: "postgres",
		DefaultTag:        "16",
	}
	MySQLDockerImage = DockerImageConfig{
		RepositoryEnvVar:  "TEST_MYSQL_IMAGE_REPOSITORY",
		TagEnvVar:         "TEST_MYSQL_IMAGE_TAG",
		DefaultRepository: "mysql",
		DefaultTag:        "8.0",
	}
	MSSQLDockerImage = DockerImageConfig{
		RepositoryEnvVar:  "TEST_MSSQL_IMAGE_REPOSITORY",
		TagEnvVar:         "TEST_MSSQL_IMAGE_TAG",
		DefaultRepository: "mcr.microsoft.com/mssql/server",
		DefaultTag:        "2025-latest",
	}
	MosquittoDockerImage = DockerImageConfig{
		RepositoryEnvVar:  "TEST_MOSQUITTO_IMAGE_REPOSITORY",
		TagEnvVar:         "TEST_MOSQUITTO_IMAGE_TAG",
		DefaultRepository: "eclipse-mosquitto",
		DefaultTag:        "2.0",
	}
	NATSDockerImage = DockerImageConfig{
		RepositoryEnvVar:  "TEST_NATS_IMAGE_REPOSITORY",
		TagEnvVar:         "TEST_NATS_IMAGE_TAG",
		DefaultRepository: "nats",
		DefaultTag:        "2.12",
	}
)

func SupportDockerTest() bool {
	if os.Getenv("CI") == "true" && !dockerTestsEnabledInCI() {
		return false
	}
	if runtime.GOOS == "linux" {
		return supportLinuxDockerTest()
	}
	if runtime.GOOS == "windows" {
		return false
	}
	if runtime.GOOS == "darwin" {
		return supportDarwinDockerTest()
	}
	return true
}

func (config DockerImageConfig) Resolve() (string, string) {
	repository := strings.TrimSpace(os.Getenv(config.RepositoryEnvVar))
	if repository == "" {
		repository = config.DefaultRepository
	}

	tag := strings.TrimSpace(os.Getenv(config.TagEnvVar))
	if tag == "" {
		tag = config.DefaultTag
	}

	return repository, tag
}

func dockerTestsEnabledInCI() bool {
	return envVarEnabled(EnableDockerTestsEnv)
}

func envVarEnabled(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func supportLinuxDockerTest() bool {
	if runtime.GOARCH != "amd64" {
		return false
	}

	return supportDockerTestWithPaths(linuxDockerSocketPaths())
}

func supportDarwinDockerTest() bool {
	return supportDockerTestWithPaths(darwinDockerSocketPaths())
}

func supportDockerTestWithPaths(paths []string) bool {
	path, ok := firstReachableDockerSocket(paths)
	if !ok {
		return false
	}
	os.Setenv("DOCKER_HOST", "unix://"+path)
	return true
}

func darwinDockerSocketPaths() []string {
	paths := make([]string, 0, 3)

	if path, ok := dockerHostSocketPath(); ok {
		paths = append(paths, path)
	}

	home, err := os.UserHomeDir()
	if err == nil {
		paths = append(paths, filepath.Join(home, ".docker", "run", "docker.sock"))
	}
	paths = append(paths, "/var/run/docker.sock")
	return uniquePaths(paths)
}

func linuxDockerSocketPaths() []string {
	paths := make([]string, 0, 2)
	if path, ok := dockerHostSocketPath(); ok {
		paths = append(paths, path)
	}
	paths = append(paths, "/var/run/docker.sock")
	return uniquePaths(paths)
}

func dockerHostSocketPath() (string, bool) {
	host := os.Getenv("DOCKER_HOST")
	const prefix = "unix://"
	if !strings.HasPrefix(host, prefix) {
		return "", false
	}

	path := strings.TrimPrefix(host, prefix)
	if path == "" {
		return "", false
	}

	return path, true
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	ret := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		ret = append(ret, path)
	}
	return ret
}

func firstReachableDockerSocket(paths []string) (string, bool) {
	for _, path := range paths {
		if canConnectDockerSocket(path) {
			return path, true
		}
	}

	return "", false
}

func canConnectDockerSocket(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return false
	}

	conn, err := net.DialTimeout("unix", path, dockerSocketDialTimeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
