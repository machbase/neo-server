package dbus_test

import (
	"encoding/json"
	"runtime"
	"testing"
	"time"

	godbus "github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

// Mock PLC for testing purposes
type MockPLCService struct{}

func (m MockPLCService) GetTemperature() (float64, *godbus.Error) {
	return 25.4, nil
}

func (m MockPLCService) Add(left, right int32) (int32, *godbus.Error) {
	return left + right, nil
}

func emitTemperatureChanged(t *testing.T, conn *godbus.Conn, value float64) {
	t.Helper()
	go func() {
		time.Sleep(200 * time.Millisecond)
		_ = conn.Emit(godbus.ObjectPath("/com/plc/device0"), "com.plc.manufacture.Interval.TemperatureChanged", value)
	}()
}

func emitNameOwnerChanged(t *testing.T, name string) {
	t.Helper()
	go func() {
		time.Sleep(200 * time.Millisecond)
		conn, err := godbus.ConnectSessionBus()
		if err != nil {
			return
		}
		defer conn.Close()
		reply, err := conn.RequestName(name, godbus.NameFlagReplaceExisting)
		if err != nil || reply != godbus.RequestNameReplyPrimaryOwner {
			return
		}
		_, _ = conn.ReleaseName(name)
	}()
}

func startMockPLCService(t *testing.T) *godbus.Conn {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("DBus tests run only on Linux.")
	}

	serverConn, err := godbus.SessionBus()
	if err != nil {
		t.Fatalf("Failed to connect to test bus: %v", err)
	}

	mockPLC := MockPLCService{}
	err = serverConn.Export(mockPLC, "/com/plc/device0", "com.plc.manufacture.Interval")
	if err != nil {
		serverConn.Close()
		t.Fatalf("Failed to register mock PLC object: %v", err)
	}

	_, err = prop.Export(serverConn, "/com/plc/device0", prop.Map{
		"com.plc.manufacture.Status": {
			"Mode": {
				Value:    "AUTO",
				Writable: true,
			},
		},
	})
	if err != nil {
		serverConn.Close()
		t.Fatalf("Failed to export mock PLC properties: %v", err)
	}

	node := &introspect.Node{
		Name: "/com/plc/device0",
		Interfaces: []introspect.Interface{
			{
				Name:    "com.plc.manufacture.Interval",
				Methods: introspect.Methods(mockPLC),
				Signals: []introspect.Signal{{
					Name: "TemperatureChanged",
					Args: []introspect.Arg{{Name: "value", Type: "d", Direction: "out"}},
				}},
			},
			prop.IntrospectData,
			{
				Name: "com.plc.manufacture.Status",
				Properties: []introspect.Property{{
					Name:   "Mode",
					Type:   "s",
					Access: "readwrite",
				}},
			},
		},
	}
	err = serverConn.Export(introspect.NewIntrospectable(node), "/com/plc/device0", "org.freedesktop.DBus.Introspectable")
	if err != nil {
		serverConn.Close()
		t.Fatalf("Failed to export mock PLC introspection: %v", err)
	}

	reply, err := serverConn.RequestName("com.plc.manufacture.Service", godbus.NameFlagReplaceExisting)
	if err != nil || reply != godbus.RequestNameReplyPrimaryOwner {
		serverConn.Close()
		t.Fatalf("Failed to acquire mock PLC service name: %v", err)
	}

	return serverConn
}

func TestPLCMonitoring(t *testing.T) {
	serverConn := startMockPLCService(t)
	defer serverConn.Close()

	// ----------------------------------------------------
	// 4. Start validating the connection flow (original monitoring logic)
	// ----------------------------------------------------
	clientConn, err := godbus.SessionBus()
	if err != nil {
		t.Fatalf("Failed to connect consumer bus: %v", err)
	}
	defer clientConn.Close()

	obj := clientConn.Object("com.plc.manufacture.Service", "/com/plc/device0")

	var temp float64
	err = obj.Call("com.plc.manufacture.Interval.GetTemperature", 0).Store(&temp)
	if err != nil {
		t.Fatalf("Method call failed: %v", err)
	}

	// Check expected value
	expected := 25.4
	if temp != expected {
		t.Errorf("Expected value %.1f, but received %.1f", expected, temp)
	}
}

func TestScriptDBus(t *testing.T) {
	serverConn := startMockPLCService(t)
	defer serverConn.Close()

	tests := []test_engine.TestCase{
		{
			Name: "dbus-object-call",
			Script: `
				const dbus = require("dbus");
				let conn;
				try {
					conn = new dbus.Connection();
					const plc = conn.object("com.plc.manufacture.Service", "/com/plc/device0");
					const temp = plc.call("com.plc.manufacture.Interval.GetTemperature");
					const sum = plc.call("com.plc.manufacture.Interval.Add", 7, 5);
					console.println("temperature:", temp.body[0]);
					console.println("sum:", sum.body[0]);
					console.println("bus:", dbus.BusType.Session);
				} catch (e) {
					console.println("Error:", e.message);
				} finally {
					if (conn !== undefined) conn.close();
				}
			`,
			Output: []string{
				"temperature: 25.4",
				"sum: 12",
				"bus: session",
			},
		},
		{
			Name: "dbus-introspect",
			Script: `
				const dbus = require("dbus");
				const conn = new dbus.Connection();
				const plc = conn.object("com.plc.manufacture.Service", "/com/plc/device0");
				const node = plc.introspect();
				console.println(JSON.stringify({
					name: node.name,
					interfaces: node.interfaces.map(iface => iface.name).sort(),
					methodNames: node.interfaces.find(iface => iface.name === "com.plc.manufacture.Interval").methods.map(method => method.name).sort(),
					propertyNames: node.interfaces.find(iface => iface.name === "com.plc.manufacture.Status").properties.map(prop => prop.name).sort(),
					signalNames: node.interfaces.find(iface => iface.name === "com.plc.manufacture.Interval").signals.map(sig => sig.name).sort(),
				}));
				conn.close();
			`,
			ExpectFunc: func(t *testing.T, got string) {
				var payload struct {
					Name          string   `json:"name"`
					Interfaces    []string `json:"interfaces"`
					MethodNames   []string `json:"methodNames"`
					PropertyNames []string `json:"propertyNames"`
					SignalNames   []string `json:"signalNames"`
				}
				err := json.Unmarshal([]byte(got), &payload)
				if err != nil {
					t.Fatalf("failed to decode introspection output: %v\noutput: %s", err, got)
				}
				if payload.Name != "/com/plc/device0" {
					t.Fatalf("unexpected node name: %q", payload.Name)
				}
				if len(payload.Interfaces) == 0 || len(payload.MethodNames) == 0 || len(payload.PropertyNames) == 0 || len(payload.SignalNames) == 0 {
					t.Fatalf("introspection payload missing expected members: %+v", payload)
				}
			},
		},
		{
			Name: "dbus-property-helpers",
			Script: `
				const dbus = require("dbus");
				const conn = new dbus.Connection();
				const plc = conn.object("com.plc.manufacture.Service", "/com/plc/device0");
				console.println("mode:", plc.get("Mode", "com.plc.manufacture.Status"));
				plc.set("Mode", "MANUAL", "com.plc.manufacture.Status");
				console.println("mode:", plc.get("Mode", "com.plc.manufacture.Status"));
				conn.close();
			`,
			Output: []string{
				"mode: AUTO",
				"mode: MANUAL",
			},
		},
		{
			Name: "dbus-name-owner-helper",
			Script: `
				const dbus = require("dbus");
				const conn = new dbus.Connection();
				const known = conn.getNameOwner("com.plc.manufacture.Service");
				console.println("known owner:", known.hasOwner, known.owner !== "");
				const unknown = conn.getNameOwner("com.plc.manufacture.NoSuchService");
				console.println("unknown owner:", unknown.hasOwner === false, unknown.owner === "");
				conn.close();
			`,
			Output: []string{
				"known owner: true true",
				"unknown owner: true true",
			},
		},
		{
			Name: "dbus-name-watch",
			Script: `
				const dbus = require("dbus");
				const conn = new dbus.Connection();
				let count = 0;
				const initial = conn.getNameOwner("com.plc.manufacture.Watcher");
				console.println("initial owner:", initial.hasOwner === false);
				conn.watchName("com.plc.manufacture.Watcher");
				conn.on("name-owner-changed", (event) => {
					if (event.name !== "com.plc.manufacture.Watcher") {
						return;
					}
					count += 1;
					console.println("name-owner:", event.name, event.oldOwner === "", event.newOwner === "");
					if (count === 2) {
						conn.unwatchName("com.plc.manufacture.Watcher");
						conn.close();
					}
				});
				setTimeout(() => {
					if (count < 2) {
						console.println("name watch timeout", count);
						conn.close();
					}
				}, 1000);
			`,
			Output: []string{
				"initial owner: true",
				"name-owner: com.plc.manufacture.Watcher true false",
				"name-owner: com.plc.manufacture.Watcher false true",
			},
		},
		{
			Name: "dbus-signal-subscribe",
			Script: `
				const dbus = require("dbus");
				const conn = new dbus.Connection();
				const plc = conn.object("com.plc.manufacture.Service", "/com/plc/device0");
				let received = false;
				plc.subscribeSignal("TemperatureChanged", "com.plc.manufacture.Interval");
				conn.on("signal", (sig) => {
					if (sig.member !== "TemperatureChanged") {
						return;
					}
					received = true;
					console.println("signal:", sig.interface, sig.member, sig.body[0]);
					plc.unsubscribeSignal("TemperatureChanged", "com.plc.manufacture.Interval");
					conn.close();
				});
				setTimeout(() => {
					if (!received) {
						console.println("signal timeout");
						conn.close();
					}
				}, 500);
			`,
			Output: []string{
				"signal: com.plc.manufacture.Interval TemperatureChanged 26.5",
			},
		},
		{
			Name: "dbus-close-idempotent",
			Script: `
				const dbus = require("dbus");
				const conn = new dbus.Connection();
				console.println("first close:", typeof conn.close() === "undefined");
				console.println("second close:", typeof conn.close() === "undefined");
			`,
			Output: []string{
				"first close: true",
				"second close: true",
			},
		},
		{
			Name: "dbus-errors",
			Script: `
				const dbus = require("dbus");
				let conn;
				try {
					conn = new dbus.Connection();
					let failed = false;
					try {
						conn.call({});
					} catch (e) {
						failed = true;
					}
					console.println("missing destination:", failed);
					failed = false;
					try {
						new dbus.Connection({ busType: "invalid" });
					} catch (e) {
						failed = true;
					}
					console.println("invalid bus type:", failed);
				} finally {
					if (conn !== undefined) conn.close();
				}
			`,
			Output: []string{
				"missing destination: true",
				"invalid bus type: true",
			},
		},
		{
			Name: "dbus-signal-errors",
			Script: `
				const dbus = require("dbus");
				const conn = new dbus.Connection();
				let failed = false;
				try {
					conn.subscribeSignal({});
				} catch (e) {
					failed = true;
				}
				console.println("missing signal match:", failed);
				failed = false;
				try {
					conn.unsubscribeSignal({ path: "/com/plc/device0", interface: "com.plc.manufacture.Interval", member: "TemperatureChanged" });
				} catch (e) {
					failed = true;
				}
				console.println("missing subscription:", failed);
				conn.close();
			`,
			Output: []string{
				"missing signal match: true",
				"missing subscription: true",
			},
		},
		{
			Name: "dbus-name-watch-errors",
			Script: `
				const dbus = require("dbus");
				const conn = new dbus.Connection();
				let failed = false;
				try {
					conn.watchName("");
				} catch (e) {
					failed = true;
				}
				console.println("missing name:", failed);
				failed = false;
				try {
					conn.unwatchName("com.plc.manufacture.unknown");
				} catch (e) {
					failed = true;
				}
				console.println("missing name watch:", failed);
				conn.close();
			`,
			Output: []string{
				"missing name: true",
				"missing name watch: true",
			},
		},
	}

	for _, tc := range tests {
		if tc.Name == "dbus-signal-subscribe" {
			emitTemperatureChanged(t, serverConn, 26.5)
		}
		if tc.Name == "dbus-name-watch" {
			emitNameOwnerChanged(t, "com.plc.manufacture.Watcher")
		}
		test_engine.RunTest(t, tc)
	}
}
