package server

import (
	"errors"
	"testing"

	"github.com/machbase/neo-server/v8/mods/logging"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

type mqttTestAuthServer struct {
	token    string
	allow    bool
	allowErr error
}

func (s *mqttTestAuthServer) ValidateClientToken(token string) (bool, error) {
	s.token = token
	return s.allow, s.allowErr
}

func (s *mqttTestAuthServer) ValidateClientCertificate(clientId string, certHash string) (bool, error) {
	return false, nil
}

func (s *mqttTestAuthServer) ValidateUserPublicKey(user string, publicKey ssh.PublicKey) (bool, string, error) {
	return false, "", nil
}

func (s *mqttTestAuthServer) ValidateUserPassword(user string, password string) (bool, string, error) {
	return false, "", nil
}

func (s *mqttTestAuthServer) ValidateUserOtp(user string, otp string) (bool, error) {
	return false, nil
}

func (s *mqttTestAuthServer) GenerateOtp(user string) (string, error) {
	return "", nil
}

func (s *mqttTestAuthServer) GenerateSnowflake() string {
	return ""
}

func TestNewMqttOptions(t *testing.T) {
	var started bool
	var stopped bool
	authSvc := &mqttTestAuthServer{allow: true}

	svr, err := NewMqtt(nil,
		WithMqttAuthServer(authSvc, true),
		WithMqttMaxMessageSizeLimit(4096),
		WithMqttOnStarted(func() { started = true }),
		WithMqttOnStopped(func() { stopped = true }),
	)
	require.NoError(t, err)
	require.NotNil(t, svr)
	require.Equal(t, "db/reply", svr.defaultReplyTopic)
	require.True(t, svr.enableTokenAuth)
	require.Same(t, authSvc, svr.authServer)
	require.EqualValues(t, 4096, svr.broker.Options.Capabilities.MaximumPacketSize)
	require.Same(t, svr, svr.authHook.svr)

	svr.authHook.OnStarted()
	svr.authHook.OnStopped()
	require.True(t, started)
	require.True(t, stopped)

	t.Run("option_error", func(t *testing.T) {
		expected := errors.New("option failure")
		_, err := NewMqtt(nil, func(s *mqttd) error { return expected })
		require.ErrorIs(t, err, expected)
	})
}

func TestMqttACLCheck(t *testing.T) {
	svr := &mqttd{restrictTopics: true}

	tests := []struct {
		name  string
		topic string
		write bool
		allow bool
	}{
		{name: "deny_subscribe_query", topic: "db/query", write: false, allow: false},
		{name: "deny_publish_reply", topic: "db/reply/abc", write: true, allow: false},
		{name: "deny_subscribe_tql", topic: "db/tql/script.tql", write: false, allow: false},
		{name: "deny_root_topic", topic: "db", write: true, allow: false},
		{name: "deny_wildcard_subscribe", topic: "db/#", write: false, allow: false},
		{name: "deny_publish_sys", topic: "$SYS/broker/load", write: true, allow: false},
		{name: "allow_write_query", topic: "db/query", write: true, allow: true},
		{name: "allow_normal_subscribe", topic: "db/reply/custom", write: false, allow: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.allow, svr.onACLCheck(nil, tt.topic, tt.write))
		})
	}
}

func TestAuthHookProvidesAndPacketEncode(t *testing.T) {
	hook := &AuthHook{}
	require.True(t, hook.Provides(mqtt.OnStarted))
	require.True(t, hook.Provides(mqtt.OnStopped))
	require.True(t, hook.Provides(mqtt.OnConnectAuthenticate))
	require.True(t, hook.Provides(mqtt.OnACLCheck))
	require.True(t, hook.Provides(mqtt.OnConnect))
	require.True(t, hook.Provides(mqtt.OnPublished))
	require.True(t, hook.Provides(mqtt.OnDisconnect))
	require.True(t, hook.Provides(mqtt.OnPacketEncode))
	require.False(t, hook.Provides(0))

	puback := packets.Packet{FixedHeader: packets.FixedHeader{Type: packets.Puback}, ReasonCode: 1}
	encoded := hook.OnPacketEncode(nil, puback)
	require.Equal(t, byte(0), encoded.ReasonCode)

	other := packets.Packet{FixedHeader: packets.FixedHeader{Type: packets.Puback}, ReasonCode: 2}
	encoded = hook.OnPacketEncode(nil, other)
	require.Equal(t, byte(2), encoded.ReasonCode)
	encoded = hook.OnPacketEncode(nil, packets.Packet{FixedHeader: packets.FixedHeader{Type: packets.Publish}, ReasonCode: 1})
	require.Equal(t, byte(1), encoded.ReasonCode)
}

func TestAuthHookOnConnectAuthenticate(t *testing.T) {
	client := &mqtt.Client{ID: "client-1"}
	pk := packets.Packet{Connect: packets.ConnectParams{Username: []byte("token-value")}}
	log := logging.GetLog("mqttd-test")

	t.Run("disabled", func(t *testing.T) {
		hook := &AuthHook{svr: &mqttd{log: log, enableTokenAuth: false}}
		require.True(t, hook.OnConnectAuthenticate(client, pk))
	})

	t.Run("missing_auth_server", func(t *testing.T) {
		hook := &AuthHook{svr: &mqttd{log: log, enableTokenAuth: true}}
		require.False(t, hook.OnConnectAuthenticate(client, pk))
	})

	t.Run("validate_true", func(t *testing.T) {
		authSvc := &mqttTestAuthServer{allow: true}
		hook := &AuthHook{svr: &mqttd{log: log, enableTokenAuth: true, authServer: authSvc}}
		require.True(t, hook.OnConnectAuthenticate(client, pk))
		require.Equal(t, "token-value", authSvc.token)
	})

	t.Run("validate_false", func(t *testing.T) {
		hook := &AuthHook{svr: &mqttd{log: log, enableTokenAuth: true, authServer: &mqttTestAuthServer{allow: false}}}
		require.False(t, hook.OnConnectAuthenticate(client, pk))
	})

	t.Run("validate_error", func(t *testing.T) {
		hook := &AuthHook{svr: &mqttd{log: log, enableTokenAuth: true, authServer: &mqttTestAuthServer{allowErr: errors.New("boom")}}}
		require.False(t, hook.OnConnectAuthenticate(client, pk))
	})
}

func TestLoadTlsConfigErrorsAndTcpHelper(t *testing.T) {
	require.NotPanics(t, func() {
		configureTcpConn(nil)
	})

	_, err := LoadTlsConfig("/path/does/not/exist.crt", "/path/does/not/exist.key", false, false)
	require.Error(t, err)
}
