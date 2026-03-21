package root_test

import (
	"strings"
	"testing"
	"time"

	nserver "github.com/nats-io/nats-server/v2/server"
	gnats "github.com/nats-io/nats.go"
)

func setupNatsPubTestServer(t *testing.T) (string, *gnats.Conn) {
	t.Helper()
	opts := &nserver.Options{
		Host:   "127.0.0.1",
		Port:   -1,
		NoLog:  true,
		NoSigs: true,
	}
	svr, err := nserver.NewServer(opts)
	if err != nil {
		t.Fatalf("create nats server: %v", err)
	}
	go svr.Start()
	if !svr.ReadyForConnections(10 * time.Second) {
		t.Fatal("nats server did not start in time")
	}
	t.Cleanup(func() {
		svr.Shutdown()
	})

	addr := "nats://" + svr.Addr().String()
	conn, err := gnats.Connect(addr)
	if err != nil {
		t.Fatalf("connect nats: %v", err)
	}
	t.Cleanup(func() {
		conn.Close()
	})
	return addr, conn
}

func TestNatsPubSupportsReply(t *testing.T) {
	addr, conn := setupNatsPubTestServer(t)

	_, err := conn.Subscribe("test.subject", func(msg *gnats.Msg) {
		if msg.Reply != "" {
			_ = msg.Respond([]byte("reply-ok"))
		}
	})
	if err != nil {
		t.Fatalf("subscribe responder: %v", err)
	}
	if err := conn.FlushTimeout(5 * time.Second); err != nil {
		t.Fatalf("flush responder: %v", err)
	}

	workDir := t.TempDir()
	output, err := runCommand(workDir, nil,
		"nats_pub",
		"--broker", addr,
		"--topic", "test.subject",
		"--message", "hello",
		"--reply", "reply.subject",
		"--timeout", "3000",
	)
	if err != nil {
		t.Fatalf("nats_pub with reply failed: %v\n%s", err, output)
	}
	if got := strings.TrimSpace(output); got != "reply-ok" {
		t.Fatalf("reply output = %q, want %q", got, "reply-ok")
	}
}

func TestNatsPubSupportsRequestMode(t *testing.T) {
	addr, conn := setupNatsPubTestServer(t)

	_, err := conn.Subscribe("test.subject", func(msg *gnats.Msg) {
		if msg.Reply == "" {
			t.Errorf("expected auto-generated reply subject")
			return
		}
		if !strings.HasPrefix(msg.Reply, "_INBOX.") {
			t.Errorf("reply subject = %q, want _INBOX.*", msg.Reply)
			return
		}
		_ = msg.Respond([]byte("request-ok"))
	})
	if err != nil {
		t.Fatalf("subscribe request responder: %v", err)
	}
	if err := conn.FlushTimeout(5 * time.Second); err != nil {
		t.Fatalf("flush request responder: %v", err)
	}

	workDir := t.TempDir()
	output, err := runCommand(workDir, nil,
		"nats_pub",
		"--broker", addr,
		"--topic", "test.subject",
		"--message", "hello",
		"--request",
		"--timeout=3000",
	)
	if err != nil {
		t.Fatalf("nats_pub with request mode failed: %v\n%s", err, output)
	}
	if got := strings.TrimSpace(output); got != "request-ok" {
		t.Fatalf("request output = %q, want %q", got, "request-ok")
	}
}
