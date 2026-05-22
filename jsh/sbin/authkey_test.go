package sbin_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuthKeyGenVirtualPathECDSA(t *testing.T) {
	workDir := t.TempDir()

	output, err := runCommand(workDir, nil, "authkey", "gen", "-t", "ecdsa", "-o", "keys/demo_key")
	if err != nil {
		t.Fatalf("authkey gen ecdsa failed: %v\n%s", err, output)
	}

	privatePath := filepath.Join(workDir, "keys", "demo_key")
	publicPath := privatePath + ".pub"

	privateBytes, err := os.ReadFile(privatePath)
	if err != nil {
		t.Fatalf("read private key: %v", err)
	}
	publicBytes, err := os.ReadFile(publicPath)
	if err != nil {
		t.Fatalf("read public key: %v", err)
	}

	if !strings.Contains(string(privateBytes), "BEGIN EC PRIVATE KEY") {
		t.Fatalf("unexpected private key format: %s", string(privateBytes))
	}
	if !strings.Contains(string(publicBytes), "BEGIN PUBLIC KEY") {
		t.Fatalf("unexpected public key format: %s", string(publicBytes))
	}
}

func TestAuthKeyGenHostPathRSA(t *testing.T) {
	workDir := t.TempDir()
	hostDir := t.TempDir()
	hostBase := filepath.Join(hostDir, "sample_key")

	output, err := runCommand(workDir, nil, "authkey", "gen", "-t", "rsa", "-o", "@"+hostBase)
	if err != nil {
		t.Fatalf("authkey gen rsa failed: %v\n%s", err, output)
	}

	privateBytes, err := os.ReadFile(hostBase)
	if err != nil {
		t.Fatalf("read host private key: %v", err)
	}
	publicBytes, err := os.ReadFile(hostBase + ".pub")
	if err != nil {
		t.Fatalf("read host public key: %v", err)
	}

	if !strings.Contains(string(privateBytes), "BEGIN RSA PRIVATE KEY") {
		t.Fatalf("unexpected private key format: %s", string(privateBytes))
	}
	if !strings.Contains(string(publicBytes), "BEGIN PUBLIC KEY") {
		t.Fatalf("unexpected public key format: %s", string(publicBytes))
	}
}

func TestAuthKeyGenRequiresOutput(t *testing.T) {
	workDir := t.TempDir()

	output, err := runCommand(workDir, nil, "authkey", "gen", "-t", "ecdsa")
	if err == nil {
		t.Fatalf("expected authkey gen without output to fail, output=%q", output)
	}
	if !strings.Contains(output, "output path is required") {
		t.Fatalf("unexpected output: %q", output)
	}
}
