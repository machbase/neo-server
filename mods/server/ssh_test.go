package server

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/pkg/sftp"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestSSH(t *testing.T) {
	tests := []SSHTestCase{
		{
			name: "shell_show_tables",
			user: "sys",
			cmd:  "show tables --format csv",
			expect: []string{
				"ROWNUM,DATABASE_NAME,USER_NAME,TABLE_NAME,TABLE_ID,TABLE_TYPE,TABLE_FLAG",
				"/r/^1,MACHBASEDB,SYS,EXAMPLE,[0-9]+,Tag,$",
				"/r/^2,MACHBASEDB,SYS,LOG_DATA,[0-9]+,Log,$",
				"/r/^3,MACHBASEDB,SYS,TAG_DATA,[0-9]+,Tag,$",
			},
		},
		{
			name: "jsh_echo",
			user: "sys:jsh",
			cmd:  "echo ssh-ok",
			expect: []string{
				"ssh-ok",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runSSHTest(t, tt)
		})
	}
}

func TestSSH_SshKey(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	require.NoError(t, err)
	sshPubKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	sshPubKeyBytes := ssh.MarshalAuthorizedKey(sshPubKey)
	sha256Fingerprint := ssh.FingerprintSHA256(sshPubKey)
	sshSigner, err := ssh.NewSignerFromKey(privateKey)
	require.NoError(t, err)

	tests := []SSHTestCase{
		{
			name: "shell_ssh-key_add",
			user: "sys",
			cmd:  fmt.Sprintf("ssh-key add %s your_email@example.com", strings.TrimSpace(string(sshPubKeyBytes))),
			expect: []string{
				"SSH key added successfully.",
			},
		},
		{
			name:       "shell_ssh-key_list",
			user:       "sys",
			cmd:        "ssh-key list",
			privateKey: sshSigner,
			expect: []string{
				fmt.Sprintf("/r/^│\\s+\\d+\\s*│\\s+your_email@example\\.com\\s+│\\s+ecdsa-sha2-nistp256\\s+│\\s+%s\\s*│$", regexp.QuoteMeta(sha256Fingerprint)),
			},
		},
		{
			name: "shell_ssh-key_delete",
			user: "sys",
			cmd:  fmt.Sprintf("ssh-key del %s", sha256Fingerprint),
			expect: []string{
				"SSH key deleted successfully.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runSSHTest(t, tt)
		})
	}
}

func TestSSH_Bridge_SQLite(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping SSH tests on Windows")
	}
	tests := []SSHTestCase{
		{
			name: "bridge_sqlite_add",
			cmd:  `bridge add -t sqlite mem file::memory:?cache=shared`,
			expect: []string{
				`Adding bridge... mem type: sqlite path: file::memory:?cache=shared`,
			},
		},
		{
			name: "bridge_sqlite_list",
			cmd:  "bridge list",
			expect: []string{
				`┌────────┬──────┬────────┬────────────────────────────┐`,
				`│ ROWNUM │ NAME │ TYPE   │ CONNECTION                 │`,
				`├────────┼──────┼────────┼────────────────────────────┤`,
				`│      1 │ mem  │ sqlite │ file::memory:?cache=shared │`,
				`└────────┴──────┴────────┴────────────────────────────┘`,
			},
		},
		{
			name: "bridge_sqlite_create_table",
			cmd:  `bridge exec mem "CREATE TABLE IF NOT EXISTS mem_example (id INTEGER NOT NULL PRIMARY KEY, company TEXT, employee INTEGER, discount REAL, code TEXT, valid BOOLEAN, memo BLOB,  created_on DATETIME NOT NULL);"`,
			expect: []string{
				`executed.`,
			},
		},
		{
			name: "bridge_sqlite_query_table",
			cmd:  `bridge query mem "SELECT * FROM sqlite_schema WHERE name = 'mem_example';"`,
			expect: []string{
				`┌────────┬───────┬─────────────┬─────────────┬──────────┬───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐`,
				`│ ROWNUM │ TYPE  │ NAME        │ TBL_NAME    │ ROOTPAGE │ SQL                                                                                                                                                                           │`,
				`├────────┼───────┼─────────────┼─────────────┼──────────┼───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤`,
				`│      1 │ table │ mem_example │ mem_example │        2 │ CREATE TABLE mem_example (id INTEGER NOT NULL PRIMARY KEY, company TEXT, employee INTEGER, discount REAL, code TEXT, valid BOOLEAN, memo BLOB,  created_on DATETIME NOT NULL) │`,
				`└────────┴───────┴─────────────┴─────────────┴──────────┴───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘`,
			},
		},
		{
			name: "bridge_sqlite_insert",
			cmd:  `bridge exec mem "insert into mem_example(company, employee, discount, created_on) values ('acme', 10, 1.234, '2023-09-09 00:00:00Z');"`,
			expect: []string{
				`executed.`,
			},
		},
		{
			name: "bridge_sqlite_select",
			cmd:  `bridge query mem "select company, employee, discount, created_on from mem_example;"`,
			expect: []string{
				`┌────────┬─────────┬──────────┬──────────┬──────────────────────┐`,
				`│ ROWNUM │ COMPANY │ EMPLOYEE │ DISCOUNT │ CREATED_ON           │`,
				`├────────┼─────────┼──────────┼──────────┼──────────────────────┤`,
				`│      1 │ acme    │       10 │    1.234 │ 2023-09-09T00:00:00Z │`,
				`└────────┴─────────┴──────────┴──────────┴──────────────────────┘`,
			},
		},
		{
			name: "bridge_sqlite_drop_table",
			cmd:  `bridge exec mem "drop table mem_example;"`,
			expect: []string{
				`executed.`,
			},
		},
		{
			name: "bridge_sqlite_delete",
			cmd:  `bridge del mem`,
			expect: []string{
				`Deleted.`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runSSHTest(t, tt)
		})
	}
}

func sshBridgePostgresTest(t *testing.T, dsn string) {
	tests := []SSHTestCase{
		{
			name: "bridge_list",
			cmd:  `bridge list`,
			expect: []string{
				"┌────────┬──────┬──────┬────────────┐",
				"│ ROWNUM │ NAME │ TYPE │ CONNECTION │",
				"├────────┼──────┼──────┼────────────┤",
				"└────────┴──────┴──────┴────────────┘",
			},
		},
		{
			name: "bridge_add_postgres",
			cmd:  fmt.Sprintf("bridge add br-postgres --type postgres %s", dsn),
			expect: []string{
				"Adding bridge... br-postgres type: postgres path: " + dsn,
			},
		},
		{
			name: "bridge_list_after_add",
			cmd:  `bridge list`,
			expect: []string{
				"┌────────┬─────────────┬──────────┬─────────────────────────────────────────────────────────────────────────────────┐",
				"│ ROWNUM │ NAME        │ TYPE     │ CONNECTION                                                                      │",
				"├────────┼─────────────┼──────────┼─────────────────────────────────────────────────────────────────────────────────┤",
				"│      1 │ br-postgres │ postgres │ " + dsn + " │",
				"└────────┴─────────────┴──────────┴─────────────────────────────────────────────────────────────────────────────────┘",
			},
		},
		{
			name: "bridge_test_postgres",
			cmd:  `bridge test br-postgres`,
			expect: []string{
				"Testing bridge... br-postgres",
				"OK.",
			},
		},
		{
			name: "bridge_exec_postgres_create_table",
			cmd:  `bridge exec br-postgres "CREATE TABLE IF NOT EXISTS ids(id SERIAL PRIMARY KEY, memo TEXT)"`,
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_postgres_insert_1",
			cmd:  `bridge exec br-postgres "INSERT INTO ids(memo) VALUES('pg-1')"`,
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_postgres_insert_2",
			cmd:  `bridge exec br-postgres INSERT INTO ids(memo) VALUES('pg-2')`,
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_postgres_query",
			cmd:  `bridge query br-postgres SELECT * FROM ids ORDER BY id`,
			expect: []string{
				"┌────────┬────┬──────┐",
				"│ ROWNUM │ ID │ MEMO │",
				"├────────┼────┼──────┤",
				"│      1 │  1 │ pg-1 │",
				"│      2 │  2 │ pg-2 │",
				"└────────┴────┴──────┘",
			},
		},
		{
			name: "bridge_exec_postgres_create_supported_table",
			cmd:  `bridge exec br-postgres CREATE TABLE IF NOT EXISTS typed_ids(id SERIAL PRIMARY KEY, event_bool BOOLEAN, event_int INTEGER, event_bigint BIGINT, event_real REAL, event_text TEXT, event_uuid UUID, event_date DATE, event_timestamp TIMESTAMP, event_timestamptz TIMESTAMPTZ)`,
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_postgres_insert_supported_row",
			cmd:  `bridge exec br-postgres INSERT INTO typed_ids(event_bool, event_int, event_bigint, event_real, event_text, event_uuid, event_date, event_timestamp, event_timestamptz) VALUES(TRUE, 42, 4200000000, 3.25, 'pg-text', '550e8400-e29b-41d4-a716-446655440000', DATE '2026-03-14', TIMESTAMP '2026-03-14 05:29:01', TIMESTAMPTZ '2026-03-14 05:29:01+00')`,
			expect: []string{
				"executed.",
			},
		},
		{
			name: "bridge_exec_postgres_query_supported_types",
			cmd:  `bridge query br-postgres SELECT id, event_bool, event_int, event_bigint, event_real, event_text, event_uuid::text AS event_uuid, TO_CHAR(event_date, 'YYYY-MM-DD') AS event_date, TO_CHAR(event_timestamp, 'YYYY-MM-DD HH24:MI:SS') AS event_timestamp, TO_CHAR(event_timestamptz AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS') AS event_timestamptz FROM typed_ids ORDER BY id`,
			expect: []string{
				"/r/^┌.*┐$",
				"/r/^│ ROWNUM │ ID │ EVENT_BOOL │ EVENT_INT │ EVENT_BIGINT │ EVENT_REAL │ EVENT_TEXT │ EVENT_UUID\\s+│ EVENT_DATE │ EVENT_TIMESTAMP\\s+│ EVENT_TIMESTAMPTZ\\s+│$",
				"/r/^├.*┤$",
				"/r/^│\\s+1 │\\s+1 │ true\\s+│\\s+42\\s+│\\s+(4200000000|4\\.2e\\+09)\\s+│\\s+3\\.25\\s+│ pg-text\\s+│ 550e8400-e29b-41d4-a716-446655440000 │ 2026-03-14 │ 2026-03-14 05:29:01 │ 2026-03-14 05:29:01 │$",
				"/r/^└.*┘$",
			},
		},
		{
			name: "bridge_exec_postgres_query_timestamp_string",
			cmd:  `bridge query br-postgres SELECT id, memo, TO_CHAR(TIMESTAMP '2026-03-14 05:29:01', 'YYYY-MM-DD HH24:MI:SS') AS ts FROM ids WHERE id = 1 ORDER BY id`,
			expect: []string{
				"/r/^┌.*┐$",
				"/r/^│ ROWNUM │ ID │ MEMO │ TS\\s*│$",
				"/r/^├.*┤$",
				"/r/^│\\s+1 │\\s+1 │ pg-1 │ 2026-03-14 05:29:01 │$",
				"/r/^└.*┘$",
			},
		},
		{
			name: "bridge_exec_postgres_query_null_timestamp",
			cmd:  `bridge query br-postgres SELECT id, memo, CAST(NULL AS TIMESTAMP) AS ts FROM ids WHERE id = 1 ORDER BY id`,
			expect: []string{
				"/r/^┌.*┐$",
				"/r/^│ ROWNUM │ ID │ MEMO │ TS\\s*│$",
				"/r/^├.*┤$",
				"/r/^│\\s+1 │\\s+1 │ pg-1 │ NULL\\s*│$",
				"/r/^└.*┘$",
			},
		},
		{
			name: "bridge_exec_postgres_query_no_rows",
			cmd:  `bridge query br-postgres SELECT * FROM ids WHERE id < 0 ORDER BY id`,
			expect: []string{
				"┌────────┬────┬──────┐",
				"│ ROWNUM │ ID │ MEMO │",
				"├────────┼────┼──────┤",
				"└────────┴────┴──────┘",
			},
		},
		{
			name: "bridge_del_postgres",
			cmd:  `bridge del br-postgres`,
			expect: []string{
				"Deleted.",
			},
		},
		{
			name: "bridge_list_after_del",
			cmd:  `bridge list`,
			expect: []string{
				"┌────────┬──────┬──────┬────────────┐",
				"│ ROWNUM │ NAME │ TYPE │ CONNECTION │",
				"├────────┼──────┼──────┼────────────┤",
				"└────────┴──────┴──────┴────────────┘",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runSSHTest(t, tt)
		})
	}
}

func sshBridgeMySqlTest(t *testing.T, dsn string) {
	tests := []SSHTestCase{
		{
			name: "bridge_list",
			cmd:  `bridge list`,
			expect: []string{
				"┌────────┬──────┬──────┬────────────┐",
				"│ ROWNUM │ NAME │ TYPE │ CONNECTION │",
				"├────────┼──────┼──────┼────────────┤",
				"└────────┴──────┴──────┴────────────┘",
			},
		},
		{
			name: "bridge_add_mysql",
			cmd:  fmt.Sprintf(`bridge add -t mysql my "%s"`, dsn),
			expect: []string{
				"Adding bridge... my type: mysql path: " + dsn,
			},
		},
		{
			name: "bridge_list_after_add",
			cmd:  `bridge list`,
			expect: []string{
				"┌────────┬──────┬───────┬──────────────────────────────────────────────────────┐",
				"│ ROWNUM │ NAME │ TYPE  │ CONNECTION                                           │",
				"├────────┼──────┼───────┼──────────────────────────────────────────────────────┤",
				"│      1 │ my   │ mysql │ " + dsn + " │",
				"└────────┴──────┴───────┴──────────────────────────────────────────────────────┘",
			},
		},
		{
			name: "bridge_create_mysql_table",
			cmd: "bridge exec my \"CREATE TABLE IF NOT EXISTS my_example(" +
				"id INT NOT NULL AUTO_INCREMENT, " +
				"company VARCHAR(50) UNIQUE NOT NULL, " +
				"employee INT, " +
				"discount REAL, " +
				"plan FLOAT, " +
				"code CHAR(64), " +
				"valid SMALLINT, " +
				"memo TEXT, " +
				"created_on TIMESTAMP NOT NULL, " +
				"PRIMARY KEY(id));\"",
			expect: []string{
				`executed.`,
			},
		},
		{
			name: "bridge_desc_table",
			cmd:  `bridge query my desc my_example`,
			expect: []string{
				"┌────────┬────────────┬─────────────┬──────┬─────┬─────────┬────────────────┐",
				"│ ROWNUM │ FIELD      │ TYPE        │ NULL │ KEY │ DEFAULT │ EXTRA          │",
				"├────────┼────────────┼─────────────┼──────┼─────┼─────────┼────────────────┤",
				"│      1 │ id         │ int         │ NO   │ PRI │ NULL    │ auto_increment │",
				"│      2 │ company    │ varchar(50) │ NO   │ UNI │ NULL    │                │",
				"│      3 │ employee   │ int         │ YES  │     │ NULL    │                │",
				"│      4 │ discount   │ double      │ YES  │     │ NULL    │                │",
				"│      5 │ plan       │ float       │ YES  │     │ NULL    │                │",
				"│      6 │ code       │ char(64)    │ YES  │     │ NULL    │                │",
				"│      7 │ valid      │ smallint    │ YES  │     │ NULL    │                │",
				"│      8 │ memo       │ text        │ YES  │     │ NULL    │                │",
				"│      9 │ created_on │ timestamp   │ NO   │     │ NULL    │                │",
				"└────────┴────────────┴─────────────┴──────┴─────┴─────────┴────────────────┘",
			},
		},
		{
			name: "bridge_insert_mysql",
			cmd:  `bridge exec my "insert into my_example(company, employee, discount, plan, created_on) value ('acme', 10, 1.234, 2.3456, '2023-09-09 00:00:00')"`,
			expect: []string{
				`executed.`,
			},
		},
		{
			name: "bridge_select_mysql",
			cmd:  `bridge query my "select * from my_example;"`,
			expect: []string{
				"┌────────┬────┬─────────┬──────────┬──────────┬────────┬──────┬───────┬──────┬──────────────────────┐",
				"│ ROWNUM │ ID │ COMPANY │ EMPLOYEE │ DISCOUNT │   PLAN │ CODE │ VALID │ MEMO │ CREATED_ON           │",
				"├────────┼────┼─────────┼──────────┼──────────┼────────┼──────┼───────┼──────┼──────────────────────┤",
				"│      1 │  1 │ acme    │       10 │    1.234 │ 2.3456 │ NULL │ NULL  │ NULL │ 2023-09-09T00:00:00Z │",
				"└────────┴────┴─────────┴──────────┴──────────┴────────┴──────┴───────┴──────┴──────────────────────┘",
			},
		},
		{
			name: "bridge_drop_mysql_table",
			cmd:  `bridge exec my "drop table my_example;"`,
			expect: []string{
				`executed.`,
			},
		},
		{
			name: "bridge_del_mysql",
			cmd:  `bridge del my`,
			expect: []string{
				`Deleted.`,
			},
			wait: 5 * time.Second,
		},
		{
			name: "bridge_list_after_del",
			cmd:  `bridge list`,
			expect: []string{
				"┌────────┬──────┬──────┬────────────┐",
				"│ ROWNUM │ NAME │ TYPE │ CONNECTION │",
				"├────────┼──────┼──────┼────────────┤",
				"└────────┴──────┴──────┴────────────┘",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runSSHTest(t, tt)
		})
	}
}

func TestSSHSession(t *testing.T) {
	s := &SSHTestSession{
		name:     "session_test",
		user:     "sys",
		password: "manager",
	}
	err := s.Connect()
	require.NoError(t, err)

	err = s.Run(t, "show info", []string{"runtime.os"}, 5*time.Second)
	require.NoError(t, err)

	err = s.Run(t, "desc example", []string{
		"EXAMPLE (ID:",
		"┌────────┬───────┬──────────┬────────┬────────────┬───────┐",
		"│ ROWNUM │ NAME  │ TYPE     │ LENGTH │ FLAG       │ INDEX │",
		"├────────┼───────┼──────────┼────────┼────────────┼───────┤",
		"│      1 │ NAME  │ varchar  │     40 │ tag name   │       │",
		"│      2 │ TIME  │ datetime │     31 │ basetime   │       │",
		"│      3 │ VALUE │ double   │     17 │ summarized │       │",
		"└────────┴───────┴──────────┴────────┴────────────┴───────┘",
	}, 5*time.Second)
	require.NoError(t, err)

	err = s.Run(t, "exit", []string{}, 5*time.Second)
	require.NoError(t, err)

	err = s.Close()
	require.NoError(t, err)
}

func TestSSHSftp(t *testing.T) {
	authMethods := []ssh.AuthMethod{ssh.Password("manager")}
	client, err := ssh.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", shellPort), &ssh.ClientConfig{
		User:            "sys",
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	})
	require.NoError(t, err)
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	require.NoError(t, err)
	defer sftpClient.Close()

	remoteDir := path.Join("tmp", "test", "sftp", t.Name())
	remotePath := path.Join(remoteDir, "payload.txt")
	payload := []byte("neo sftp integration\nline-2\n")

	require.NoError(t, sftpClient.MkdirAll(remoteDir))
	defer func() {
		_ = sftpClient.Remove(remotePath)
		_ = sftpClient.RemoveDirectory(remoteDir)
	}()

	writer, err := sftpClient.Create(remotePath)
	require.NoError(t, err)
	_, err = writer.Write(payload)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	st, err := sftpClient.Stat(remotePath)
	require.NoError(t, err)
	require.Equal(t, int64(len(payload)), st.Size())

	entries, err := sftpClient.ReadDir(remoteDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "payload.txt", entries[0].Name())

	reader, err := sftpClient.Open(remotePath)
	require.NoError(t, err)
	defer reader.Close()
	actual, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.True(t, bytes.Equal(payload, actual))
}

type SSHTestSession struct {
	name       string
	user       string
	password   string
	privateKey ssh.Signer

	client  *ssh.Client
	session *ssh.Session
	stdout  lockedBuffer
	stdin   io.WriteCloser
}

func (s *SSHTestSession) Connect() error {
	authMethods := []ssh.AuthMethod{}
	if s.privateKey != nil {
		authMethods = append(authMethods, ssh.PublicKeys(s.privateKey))
	} else {
		authMethods = append(authMethods, ssh.Password(s.password))
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", shellPort), &ssh.ClientConfig{
		User:            s.user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("Failed to dial SSH server: %v", err)
	}
	s.client = client
	if session, err := client.NewSession(); err != nil {
		return fmt.Errorf("Failed to create SSH session: %v", err)
	} else {
		s.session = session
	}

	s.session.Stdout = &s.stdout
	s.session.Stderr = &s.stdout

	if stdin, err := s.session.StdinPipe(); err != nil {
		return fmt.Errorf("Failed to get SSH stdin pipe: %v", err)
	} else {
		s.stdin = stdin
	}
	if err := s.session.RequestPty("xterm", 40, 240, ssh.TerminalModes{}); err != nil {
		return fmt.Errorf("Failed to request PTY: %v", err)
	}
	if err := s.session.Shell(); err != nil {
		return fmt.Errorf("Failed to start SSH shell: %v", err)
	}

	return nil
}

func (s *SSHTestSession) Close() error {
	if s.session != nil {
		if err := s.session.Close(); err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("Failed to close SSH session: %v", err)
		}
	}
	if s.client != nil {
		if err := s.client.Close(); err != nil {
			return fmt.Errorf("Failed to close SSH client: %v", err)
		}
	}
	return nil
}

func (s *SSHTestSession) Run(t *testing.T, cmd string, expect []string, waitTimeout time.Duration) error {
	_, err := s.stdin.Write([]byte(cmd + "\n"))
	if err != nil {
		return fmt.Errorf("Failed to write SSH command: %v", err)
	}
	if !waitForSSHOutput(&s.stdout, s.user, cmd, expect, waitTimeout) {
		return fmt.Errorf("Timed out waiting for SSH output, got %s", removeTerminalControlCharacters(s.stdout.String()))
	}
	rawOutput := s.stdout.String()
	if strings.Contains(rawOutput, "panic:") {
		t.Fatalf("Unexpected panic in SSH shell output: %s", rawOutput)
	}

	outputStr := removeTerminalControlCharacters(rawOutput)
	if strings.TrimSpace(outputStr) == "" && len(expect) > 0 {
		t.Fatalf("Expected SSH command %q to produce output", cmd)
	}

	for _, line := range expect {
		require.Contains(t, outputStr, line, "Expected SSH output to contain %q, got %s", line, outputStr)
	}

	s.stdout.Clear()
	return nil
}

func TestSSHCommands(t *testing.T) {
	tests := []struct {
		name   string
		cmd    string
		expect []string
	}{
		{
			cmd: "select 1+2",
			expect: []string{
				"┌────────┬─────┐",
				"│ ROWNUM │ 1+2 │",
				"├────────┼─────┤",
				"│      1 │   3 │",
				"└────────┴─────┘",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &SSHTestCommand{
				cmd: tt.cmd,
			}
			output, err := cmd.Exec(t)
			_ = err
			//require.NoError(t, err)
			outputStr := removeTerminalControlCharacters(string(output))
			for _, line := range tt.expect {
				require.Contains(t, outputStr, line, "Expected SSH output to contain %q, got %s", line, outputStr)
			}
		})
	}
}

type SSHTestCommand struct {
	user       string
	password   string
	privateKey ssh.Signer
	cmd        string
}

func (c *SSHTestCommand) Exec(t *testing.T) ([]byte, error) {
	user := c.user
	if user == "" {
		user = "sys"
	}
	password := c.password
	if password == "" {
		password = "manager"
	}
	authMethods := []ssh.AuthMethod{}
	if c.privateKey != nil {
		authMethods = append(authMethods, ssh.PublicKeys(c.privateKey))
	} else {
		authMethods = append(authMethods, ssh.Password(password))
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", shellPort), &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to dial SSH server: %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("Failed to create SSH session: %v", err)
	}
	defer session.Close()

	return session.CombinedOutput(c.cmd)
}

type SSHTestCase struct {
	name       string
	user       string
	cmd        string
	privateKey ssh.Signer
	expect     []string
	wait       time.Duration
}

func runSSHTest(t *testing.T, tt SSHTestCase) {
	t.Helper()
	waitTimeout := tt.wait
	if waitTimeout <= 0 {
		waitTimeout = 10 * time.Second
	}
	user := tt.user
	if user == "" {
		user = "sys"
	}
	authMethods := []ssh.AuthMethod{}
	if tt.privateKey != nil {
		authMethods = append(authMethods, ssh.PublicKeys(tt.privateKey))
	} else {
		authMethods = append(authMethods, ssh.Password("manager"))
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", shellPort), &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to dial SSH server: %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("Failed to create SSH session: %v", err)
	}
	defer session.Close()

	var stdout lockedBuffer
	var stderr lockedBuffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	stdin, err := session.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to get SSH stdin pipe: %v", err)
	}

	if err := session.RequestPty("xterm", 40, 240, ssh.TerminalModes{}); err != nil {
		t.Fatalf("Failed to request PTY: %v", err)
	}

	if err := session.Shell(); err != nil {
		t.Fatalf("Failed to start SSH shell: %v", err)
	}

	if _, err := stdin.Write([]byte(tt.cmd + "\n")); err != nil {
		t.Fatalf("Failed to write SSH command: %v", err)
	}
	if !waitForSSHOutput(&stdout, user, tt.cmd, tt.expect, waitTimeout) {
		t.Fatalf("Timed out waiting for SSH output, got %s", removeTerminalControlCharacters(stdout.String()))
	}
	if _, err := stdin.Write([]byte("exit\n")); err != nil {
		if !errors.Is(err, io.EOF) {
			t.Fatalf("Failed to write SSH exit command: %v", err)
		}
	}
	stdin.Close()

	if err := session.Wait(); err != nil {
		if !strings.Contains(err.Error(), "remote command exited without exit status or exit signal") {
			t.Fatalf("SSH shell failed: %v, stderr: %s", err, removeTerminalControlCharacters(stderr.String()))
		}
	}

	rawOutput := stdout.String()
	outputStr := removeTerminalControlCharacters(rawOutput)
	if strings.TrimSpace(outputStr) == "" {
		t.Fatalf("Expected SSH command to produce output")
	}
	if strings.TrimSpace(stderr.String()) != "" {
		t.Fatalf("Expected empty stderr, got %s", removeTerminalControlCharacters(stderr.String()))
	}
	if strings.Contains(rawOutput, "panic:") {
		t.Fatalf("Unexpected panic in SSH shell output: %s", rawOutput)
	}
	if !matchExpectedOutput(normalizeSSHOutputLines(outputStr, user), tt.expect) {
		t.Fatalf("Expected SSH output sequence %v, got %s", tt.expect, outputStr)
	}
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *lockedBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func waitForSSHOutput(buf *lockedBuffer, user string, cmd string, expects []string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		output := removeTerminalControlCharacters(buf.String())
		lines := normalizeSSHOutputLines(output, user)
		if matchExpectedOutput(lines, expects) || containsExpectedOutput(lines, cmd, expects) {
			return true
		}
		if len(expects) == 0 && isSSHOutputAtPrompt(lines, user) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func containsExpectedOutput(lines []string, cmd string, expects []string) bool {
	if len(expects) == 0 {
		return true
	}
	for _, expected := range expects {
		matched := false
		for _, line := range lines {
			if isEchoedCommandLine(line, cmd) {
				continue
			}
			if strings.HasPrefix(expected, "/r/") {
				ok, err := regexp.MatchString(expected[3:], line)
				if err == nil && ok {
					matched = true
					break
				}
				continue
			}
			if strings.Contains(line, expected) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func isEchoedCommandLine(line string, cmd string) bool {
	trimmedLine := strings.TrimSpace(line)
	trimmedCmd := strings.TrimSpace(cmd)
	if trimmedLine == "" || trimmedCmd == "" {
		return false
	}
	if trimmedLine == trimmedCmd {
		return true
	}
	if strings.HasSuffix(trimmedLine, "> "+trimmedCmd) {
		return true
	}
	return false
}

func isSSHOutputAtPrompt(lines []string, user string) bool {
	if len(lines) == 0 {
		return false
	}
	lastLine := strings.TrimSpace(lines[len(lines)-1])
	if lastLine == ">" {
		return true
	}
	if sshShellID(user) == "jsh" {
		return strings.EqualFold(lastLine, "jsh>") || strings.EqualFold(lastLine, "jsh >")
	}
	return false
}

func sshShellID(user string) string {
	if idx := strings.Index(user, ":"); idx >= 0 {
		return strings.ToLower(strings.TrimSpace(user[idx+1:]))
	}
	return ""
}

func normalizeSSHOutputLines(output string, user string) []string {
	lines := strings.Split(output, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		if strings.TrimSpace(trimmed) == "" {
			continue
		}
		if isTerminalPromptLine(trimmed, user) {
			continue
		}
		if len(result) > 0 && shouldJoinWrappedSSHLine(result[len(result)-1], trimmed) {
			result[len(result)-1] += trimmed
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

func shouldJoinWrappedSSHLine(previous string, current string) bool {
	if previous == "" || current == "" {
		return false
	}
	first, _ := utf8.DecodeRuneInString(previous)
	if !strings.ContainsRune("┌├└│", first) {
		return false
	}
	last, _ := utf8.DecodeLastRuneInString(previous)
	if strings.ContainsRune("┐┤┘│", last) {
		return false
	}
	return true
}

func matchExpectedOutput(lines []string, expects []string) bool {
	if len(expects) == 0 {
		return true
	}
	idx := 0
	for _, line := range lines {
		if lineMatchesExpected(line, expects[idx]) {
			idx++
			if idx == len(expects) {
				return true
			}
		}
	}
	return false
}

func lineMatchesExpected(line string, expected string) bool {
	if strings.HasPrefix(expected, "/r/") {
		matched, err := regexp.MatchString(expected[3:], line)
		return err == nil && matched
	}
	return line == expected
}

func TestRemoveTerminalControlCharactersPreservesBoxDrawing(t *testing.T) {
	expected := strings.Join([]string{
		`┌────────┬──────┬────────┬────────────────────────────┐`,
		`│ ROWNUM │ NAME │ TYPE   │ CONNECTION                 │`,
		`├────────┼──────┼────────┼────────────────────────────┤`,
		`│      1 │ mem  │ sqlite │ file::memory:?cache=shared │`,
		`└────────┴──────┴────────┴────────────────────────────┘`,
	}, "\n")

	actual := removeTerminalControlCharacters("\x1b[32m" + expected + "\x1b[0m")

	require.Equal(t, expected, actual)
}

func TestNormalizeSSHOutputLinesRemovesPromptAndBlankLines(t *testing.T) {
	raw := strings.Join([]string{
		"Greetings, SYS",
		"machbase-neo  ( )",
		"",
		"sys machbase-neo 2026-05-06 10:20:37",
		"> bridge list",
		"┌────────┬──────┬──────┬────────────┐",
		"│ ROWNUM │ NAME │ TYPE │ CONNECTION │",
		"└────────┴──────┴──────┴────────────┘",
		"sys machbase-neo 2026-05-06 10:20:37",
		">",
	}, "\n")

	actual := normalizeSSHOutputLines(raw, "sys")

	require.Equal(t, []string{
		"Greetings, SYS",
		"machbase-neo  ( )",
		"> bridge list",
		"┌────────┬──────┬──────┬────────────┐",
		"│ ROWNUM │ NAME │ TYPE │ CONNECTION │",
		"└────────┴──────┴──────┴────────────┘",
		">",
	}, actual)
}

func TestNormalizeSSHOutputLinesJoinsWrappedTableLines(t *testing.T) {
	raw := strings.Join([]string{
		"Greetings, SYS",
		"machbase-neo  ( )",
		"sys machbase-neo 2026-05-06 10:20:37",
		"> bridge list",
		"┌────────┬──────┬───────┬───────────────────────────────────────────────",
		"───────┐",
		"│ ROWNUM │ NAME │ TYPE  │ CONNECTION                                    ",
		"       │",
		"├────────┼──────┼───────┼───────────────────────────────────────────────",
		"───────┤",
		"│      1 │ my   │ mysql │ dbuser:secret@tcp(127.0.0.1:55050)/db?parseTime",
		"=true │",
		"└────────┴──────┴───────┴───────────────────────────────────────────────",
		"───────┘",
		"sys machbase-neo 2026-05-06 10:20:37",
		">",
	}, "\n")

	actual := normalizeSSHOutputLines(raw, "sys")

	require.Equal(t, []string{
		"Greetings, SYS",
		"machbase-neo  ( )",
		"> bridge list",
		"┌────────┬──────┬───────┬──────────────────────────────────────────────────────┐",
		"│ ROWNUM │ NAME │ TYPE  │ CONNECTION                                           │",
		"├────────┼──────┼───────┼──────────────────────────────────────────────────────┤",
		"│      1 │ my   │ mysql │ dbuser:secret@tcp(127.0.0.1:55050)/db?parseTime=true │",
		"└────────┴──────┴───────┴──────────────────────────────────────────────────────┘",
		">",
	}, actual)
}

func TestMatchExpectedOutputSupportsRegexSequence(t *testing.T) {
	lines := []string{
		"bridge query br-postgres SELECT * FROM ids ORDER BY id",
		"┌────────┬────┬──────┐",
		"│ ROWNUM │ ID │ MEMO │",
		"├────────┼────┼──────┤",
		"│      1 │  1 │ pg-1 │",
		"│      2 │  2 │ pg-2 │",
		"└────────┴────┴──────┘",
	}

	require.True(t, matchExpectedOutput(lines, []string{
		`/r/^┌.*┐$`,
		`/r/^│ ROWNUM │ ID │ MEMO │$`,
		`/r/^├.*┤$`,
		`/r/^│\s+1 │\s+1 │ pg-1 │$`,
		`/r/^│\s+2 │\s+2 │ pg-2 │$`,
		`/r/^└.*┘$`,
	}))
}

func TestMatchExpectedOutputRejectsOutOfOrderSequence(t *testing.T) {
	lines := []string{
		"first",
		"second",
		"third",
	}

	require.False(t, matchExpectedOutput(lines, []string{"second", "first"}))
}

func TestLineMatchesExpectedInvalidRegexReturnsFalse(t *testing.T) {
	require.False(t, lineMatchesExpected("value", "/r/[invalid"))
}

func removeTerminalControlCharacters(s string) string {
	runes := []rune(s)
	var lines []string
	line := make([]rune, 0, 128)
	cursor := 0

	ensureCursor := func() {
		for len(line) < cursor {
			line = append(line, ' ')
		}
	}
	writeRune := func(r rune) {
		ensureCursor()
		if cursor == len(line) {
			line = append(line, r)
		} else {
			line[cursor] = r
		}
		cursor++
	}
	finishLine := func() {
		lines = append(lines, strings.TrimRight(string(line), " "))
		line = line[:0]
		cursor = 0
	}
	clearLineFromCursor := func() {
		ensureCursor()
		line = line[:cursor]
	}
	clearEntireLine := func() {
		line = line[:0]
		cursor = 0
	}

	for i := 0; i < len(runes); i++ {
		switch runes[i] {
		case '\x1b':
			if i+1 >= len(runes) {
				continue
			}
			switch runes[i+1] {
			case '[':
				j := i + 2
				for j < len(runes) && (runes[j] < '@' || runes[j] > '~') {
					j++
				}
				if j >= len(runes) {
					i = len(runes)
					break
				}
				params := string(runes[i+2 : j])
				final := runes[j]
				paramValues := parseTerminalParams(params)
				switch final {
				case 'm', 'h', 'l':
					// Ignore SGR and mode switches.
				case 'K':
					mode := firstTerminalParam(paramValues, 0)
					switch mode {
					case 0:
						clearLineFromCursor()
					case 1:
						ensureCursor()
						for pos := 0; pos < cursor && pos < len(line); pos++ {
							line[pos] = ' '
						}
					case 2:
						clearEntireLine()
					}
				case 'G':
					cursor = max(0, firstTerminalParam(paramValues, 1)-1)
				case 'C':
					cursor += firstTerminalParam(paramValues, 1)
				case 'D':
					cursor -= firstTerminalParam(paramValues, 1)
					if cursor < 0 {
						cursor = 0
					}
				case 'P':
					count := firstTerminalParam(paramValues, 1)
					ensureCursor()
					if cursor < len(line) {
						end := cursor + count
						if end > len(line) {
							end = len(line)
						}
						line = append(line[:cursor], line[end:]...)
					}
				case '@':
					count := firstTerminalParam(paramValues, 1)
					ensureCursor()
					spaces := make([]rune, count)
					for idx := range spaces {
						spaces[idx] = ' '
					}
					line = append(line[:cursor], append(spaces, line[cursor:]...)...)
				}
				i = j
			case ']':
				j := i + 2
				for j < len(runes) && runes[j] != '\a' {
					if runes[j] == '\x1b' && j+1 < len(runes) && runes[j+1] == '\\' {
						j++
						break
					}
					j++
				}
				i = j
			default:
				i++
			}
		case '\r':
			cursor = 0
		case '\n':
			finishLine()
		case '\b', 0x7f:
			if cursor > 0 {
				cursor--
			}
		case '\t':
			nextTabStop := ((cursor / 8) + 1) * 8
			for cursor < nextTabStop {
				writeRune(' ')
			}
		default:
			if runes[i] >= 0x20 {
				writeRune(runes[i])
			}
		}
	}
	if len(line) > 0 || cursor > 0 {
		finishLine()
	}
	return strings.Join(compactRepeatedLines(lines), "\n")
}

func parseTerminalParams(params string) []int {
	if params == "" {
		return nil
	}
	parts := strings.Split(params, ";")
	values := make([]int, 0, len(parts))
	for _, part := range parts {
		value := 0
		for _, ch := range part {
			if ch < '0' || ch > '9' {
				continue
			}
			value = value*10 + int(ch-'0')
		}
		values = append(values, value)
	}
	return values
}

func firstTerminalParam(params []int, fallback int) int {
	if len(params) == 0 || params[0] == 0 {
		return fallback
	}
	return params[0]
}

func compactRepeatedLines(lines []string) []string {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		if len(result) > 0 && result[len(result)-1] == trimmed {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

func isTerminalPromptLine(line string, user string) bool {
	baseUser := strings.ToLower(user)
	if idx := strings.Index(baseUser, ":"); idx >= 0 {
		baseUser = baseUser[:idx]
	}
	trimmed := strings.ToLower(strings.TrimSpace(line))
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, baseUser+" ") && strings.Contains(trimmed, " machbase-neo ") {
		return true
	}
	return false
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func TestSshCoverage_ParsePemBlock(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	rsaBytes, err := x509.MarshalPKCS8PrivateKey(rsaKey)
	require.NoError(t, err)

	key, err := parsePemBlock(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: rsaBytes})
	require.NoError(t, err)
	require.NotNil(t, key)

	ec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	ecBytes, err := x509.MarshalECPrivateKey(ec)
	require.NoError(t, err)

	key, err = parsePemBlock(&pem.Block{Type: "EC PRIVATE KEY", Bytes: ecBytes})
	require.NoError(t, err)
	require.NotNil(t, key)

	_, err = parsePemBlock(&pem.Block{Type: "UNKNOWN PRIVATE KEY", Bytes: []byte("abc")})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported key type")
}

func TestSshCoverage_SignerFromPem(t *testing.T) {
	ec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	ecBytes, err := x509.MarshalECPrivateKey(ec)
	require.NoError(t, err)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: ecBytes})

	signer, err := signerFromPem(pemBytes, nil)
	require.NoError(t, err)
	require.NotNil(t, signer)

	_, err = signerFromPem([]byte("not a pem"), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "pem decode failed")

	_, err = signerFromPem(pemBytes, []byte("wrong-password"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "decrypting PEM block failed")
}

func TestSshCoverage_SignerFromPath(t *testing.T) {
	signer, err := signerFromPath("", "")
	require.NoError(t, err)
	require.Nil(t, signer)

	_, err = signerFromPath(filepath.Join(t.TempDir(), "missing.pem"), "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "server key")

	ec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	ecBytes, err := x509.MarshalECPrivateKey(ec)
	require.NoError(t, err)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: ecBytes})

	pemPath := filepath.Join(t.TempDir(), "id_ecdsa.pem")
	require.NoError(t, os.WriteFile(pemPath, pemBytes, 0o600))

	signer, err = signerFromPath(pemPath, "")
	require.NoError(t, err)
	require.NotNil(t, signer)
}

func TestSshCoverage_NewIODebuggerWrite(t *testing.T) {
	writer := NewIODebugger(logging.GetLog("ssh-coverage-test"), "in")
	n, err := writer.Write([]byte("hello"))
	require.NoError(t, err)
	require.Equal(t, 5, n)
}

func TestServerCoverage_DoServiceUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only coverage test")
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
	})

	doService(nil)
	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Contains(t, strings.ToLower(string(out)), "windows")
}

func TestSshCoverage_StopNoPanic(t *testing.T) {
	s := &sshd{}
	require.NotPanics(t, func() {
		s.Stop()
	})
}
