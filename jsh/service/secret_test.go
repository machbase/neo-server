package service

import (
	"errors"
	"testing"
	"time"
)

func TestControllerSecretConsumeOnce(t *testing.T) {
	ctl, err := NewController(&ControllerConfig{ConfigDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewController() error: %v", err)
	}
	token, err := ctl.PutSecret([]SecretItem{
		{Key: "NEOSHELL_USER", Value: "sys"},
		{Key: "NEOSHELL_PASSWORD", Value: "secret"},
	}, time.Minute)
	if err != nil {
		t.Fatalf("PutSecret() error: %v", err)
	}

	items, err := ctl.ConsumeSecret(token)
	if err != nil {
		t.Fatalf("ConsumeSecret() error: %v", err)
	}
	if len(items) != 2 || items[0].Key != "NEOSHELL_USER" || items[0].Value != "sys" || items[1].Key != "NEOSHELL_PASSWORD" || items[1].Value != "secret" {
		t.Fatalf("ConsumeSecret() items = %#v", items)
	}
	if _, err := ctl.ConsumeSecret(token); !errors.Is(err, errSecretNotFound) {
		t.Fatalf("ConsumeSecret() second error = %v, want %v", err, errSecretNotFound)
	}
}

func TestControllerSecretExpiryAndRevoke(t *testing.T) {
	ctl, err := NewController(&ControllerConfig{ConfigDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewController() error: %v", err)
	}
	expired, err := ctl.PutSecret([]SecretItem{{Key: "k", Value: "v"}}, time.Nanosecond)
	if err != nil {
		t.Fatalf("PutSecret() expired error: %v", err)
	}
	time.Sleep(time.Millisecond)
	if _, err := ctl.ConsumeSecret(expired); !errors.Is(err, errSecretNotFound) {
		t.Fatalf("ConsumeSecret() expired error = %v, want %v", err, errSecretNotFound)
	}

	revoked, err := ctl.PutSecret([]SecretItem{{Key: "k", Value: "v"}}, time.Minute)
	if err != nil {
		t.Fatalf("PutSecret() revoked error: %v", err)
	}
	ctl.RevokeSecret(revoked)
	if _, err := ctl.ConsumeSecret(revoked); !errors.Is(err, errSecretNotFound) {
		t.Fatalf("ConsumeSecret() revoked error = %v, want %v", err, errSecretNotFound)
	}
}

func TestConsumeSecretRPC(t *testing.T) {
	ctl, err := NewController(&ControllerConfig{
		ConfigDir: t.TempDir(),
		Address:   "tcp://127.0.0.1:0",
	})
	if err != nil {
		t.Fatalf("NewController() error: %v", err)
	}
	defer ctl.Stop(nil)
	if err := ctl.startRPC(); err != nil {
		t.Fatalf("startRPC() error: %v", err)
	}
	token, err := ctl.PutSecret([]SecretItem{{Key: "NEOSHELL_USER", Value: "sys"}}, time.Minute)
	if err != nil {
		t.Fatalf("PutSecret() error: %v", err)
	}
	items, err := ConsumeSecret(ctl.Address(), token)
	if err != nil {
		t.Fatalf("ConsumeSecret() RPC error: %v", err)
	}
	if len(items) != 1 || items[0].Key != "NEOSHELL_USER" || items[0].Value != "sys" {
		t.Fatalf("ConsumeSecret() RPC items = %#v", items)
	}
	if _, err := ConsumeSecret(ctl.Address(), token); err == nil {
		t.Fatalf("ConsumeSecret() second RPC error = nil, want error")
	}
}
