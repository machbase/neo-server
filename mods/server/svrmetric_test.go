package server

import (
	"reflect"
	"testing"

	"github.com/machbase/neo-server/v8/mods/util/metric"
	"github.com/stretchr/testify/require"
)

func gatherMeasureNames(g *metric.Gather) []string {
	v := reflect.ValueOf(g).Elem().FieldByName("measures")
	names := make([]string, 0, v.Len())
	for i := 0; i < v.Len(); i++ {
		names = append(names, v.Index(i).FieldByName("Name").String())
	}
	return names
}

func TestStopServerMetrics(t *testing.T) {
	require.NotPanics(t, func() {
		stopServerMetrics()
	})
}

func TestCollectMqttStatz(t *testing.T) {
	t.Run("missing_broker", func(t *testing.T) {
		g := &metric.Gather{}
		err := collectMqttStatz(&Server{})(g)
		require.Error(t, err)
		require.ErrorContains(t, err, "MQTT broker is not initialized")
	})

	t.Run("collects_metrics_from_broker_info", func(t *testing.T) {
		mqttd, err := NewMqtt()
		require.NoError(t, err)
		t.Cleanup(func() {
			mqttd.Stop()
		})

		mqttd.broker.Info.BytesReceived = 11
		mqttd.broker.Info.BytesSent = 12
		mqttd.broker.Info.MessagesReceived = 13
		mqttd.broker.Info.MessagesSent = 14
		mqttd.broker.Info.MessagesDropped = 15
		mqttd.broker.Info.PacketsSent = 16
		mqttd.broker.Info.PacketsReceived = 17
		mqttd.broker.Info.Retained = 18
		mqttd.broker.Info.Subscriptions = 19
		mqttd.broker.Info.ClientsTotal = 20
		mqttd.broker.Info.ClientsConnected = 21
		mqttd.broker.Info.ClientsDisconnected = 22
		mqttd.broker.Info.Inflight = 23
		mqttd.broker.Info.InflightDropped = 24

		g := &metric.Gather{}
		err = collectMqttStatz(&Server{mqttd: mqttd})(g)
		require.NoError(t, err)

		names := gatherMeasureNames(g)
		require.Len(t, names, 14)
		require.Contains(t, names, "mqtt:recv_bytes")
		require.Contains(t, names, "mqtt:clients_connected")
		require.Contains(t, names, "mqtt:inflight_dropped")
	})
}
