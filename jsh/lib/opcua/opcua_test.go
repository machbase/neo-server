package opcua_test

import (
	"context"
	"log"
	"testing"

	"github.com/gopcua/opcua/id"
	opc_server "github.com/gopcua/opcua/server"
	"github.com/gopcua/opcua/server/attrs"
	"github.com/gopcua/opcua/ua"
	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestScriptOPCUA(t *testing.T) {
	svr := startOPCUAServer()
	defer svr.Close()

	tests := []test_engine.TestCase{
		{
			Name: "opcua-panic",
			Script: `
				ua = require("opcua");
				try {
					client = new ua.Client();
				} catch(e) {
					console.println("Error:", e.message); 
				}
			`,
			Output: []string{
				"Error: missing client options",
			},
		},
		{
			Name: "opcua-read",
			Script: `
				ua = require("opcua");
				nodeList = [
					[
						"ns=1;s=ro_bool",   // true
						"ns=1;s=rw_bool",   // true
						"ns=1;s=ro_int32",  // int32(5)
						"ns=1;s=rw_int32",  // int32(5)
					],
					[
						"ns=1;s=NoPermVariable",    // ua.StatusOK, int32(742)
						"ns=1;s=ReadWriteVariable", // ua.StatusOK, 12.34
						"ns=1;s=ReadOnlyVariable",  // ua.StatusOK, 9.87
						"ns=1;s=NoAccessVariable",  // ua.StatusBadUserAccessDenied
					]
				];
				
				try {
					// create the client
					client = new ua.Client({ endpoint: "opc.tcp://localhost:4840" });
					for ( i = 0; i < 2; i++ ) {
						nodes = nodeList[i];
						vs = client.read({ nodes: nodes, timestampsToReturn: ua.TimestampsToReturn.Both});
						vs.forEach((v, idx) => {
							console.println(nodes[idx], v.statusCode, v.value, v.type);
						})
					}
				} catch (e) {
					console.println("Error:", e);
				} finally {
				 	// do not forget to close the client
					if (client !== undefined) client.close();
				}
			`,
			Output: []string{
				"ns=1;s=ro_bool StatusGood true Boolean",
				"ns=1;s=rw_bool StatusGood true Boolean",
				"ns=1;s=ro_int32 StatusGood 5 Int32",
				"ns=1;s=rw_int32 StatusGood 5 Int32",
				"ns=1;s=NoPermVariable StatusGood 742 Int32",
				"ns=1;s=ReadWriteVariable StatusGood 12.34 Double",
				"ns=1;s=ReadOnlyVariable StatusGood 9.87 Double",
				"ns=1;s=NoAccessVariable StatusBadUserAccessDenied null Null",
			},
		},
		{
			Name: "opcua-write",
			Script: `
				ua = require("opcua");
				try {
					// create the client
					client = new ua.Client({ endpoint: "opc.tcp://localhost:4840" });
					rsp = client.read({ nodes: ["ns=1;s=rw_bool", "ns=1;s=rw_int32"] });
					console.println("read response:", rsp[0].value, rsp[1].value);
					rsp = client.write({node: "ns=1;s=rw_bool", value: false}, {node: "ns=1;s=rw_int32", value: 1234})
					console.println("write response error:", rsp.error, ", results:", rsp.results);
					rsp = client.read({ nodes: ["ns=1;s=rw_bool", "ns=1;s=rw_int32"] });
					console.println("read response:", rsp[0].value, rsp[1].value);
				} catch (e) {
					console.println("Error:", e);
				} finally {
				 	// do not forget to close the client
					if (client !== undefined) client.close();
				}
			`,
			Output: []string{
				"read response: true 5",
				"write response error: null , results: [0, 0]",
				"read response: false 1234",
			},
		},
		{
			Name: "opcua-close-idempotent",
			Script: `
				ua = require("opcua");
				try {
					client = new ua.Client({ endpoint: "opc.tcp://localhost:4840" });
					console.println("first close:", typeof client.close() === "undefined");
					console.println("second close:", typeof client.close() === "undefined");
				} catch (e) {
					console.println("Error:", e);
				}
			`,
			Output: []string{
				"first close: true",
				"second close: true",
			},
		},
		{
			Name: "opcua-read-errors",
			Script: `
				ua = require("opcua");
				try {
					client = new ua.Client({ endpoint: "opc.tcp://localhost:4840", readRetryInterval: 1 });
					failed = false;
					try {
						client.read({});
					} catch (e) {
						failed = true;
					}
					console.println("read missing nodes:", failed);
					failed = false;
					try {
						client.read({ nodes: ["ns=x;i=1"] });
					} catch (e) {
						failed = true;
					}
					console.println("read invalid node:", failed);
				} finally {
					if (client !== undefined) client.close();
				}
			`,
			Output: []string{
				"read missing nodes: true",
				"read invalid node: true",
			},
		},
		{
			Name: "opcua-write-errors",
			Script: `
				ua = require("opcua");
				try {
					client = new ua.Client({ endpoint: "opc.tcp://localhost:4840" });
					failed = false;
					try {
						client.write();
					} catch (e) {
						failed = true;
					}
					console.println("write missing argument:", failed);
					failed = false;
					try {
						client.write({ node: "ns=x;i=1", value: 1 });
					} catch (e) {
						failed = true;
					}
					console.println("write invalid node:", failed);
				} finally {
					if (client !== undefined) client.close();
				}
			`,
			Output: []string{
				"write missing argument: true",
				"write invalid node: true",
			},
		},
		{
			Name: "opcua-children-variables",
			Script: `
				ua = require("opcua");
				try {
					client = new ua.Client({ endpoint: "opc.tcp://localhost:4840" });
					refs = client.children({
						node: "ns=1;i=85",
						nodeClassMask: ua.NodeClass.Variable,
					});
					refs.sort((a,b) => a.browseName < b.browseName ? -1 : 1)
						.forEach(r => console.println(r.browseName, r.nodeId, r.nodeClass));
				} catch(e) {
					console.println("Error:", e);
				} finally {
					if (client !== undefined) client.close();
				}
			`,
			Output: []string{
				"NoAccessVariable ns=1;s=NoAccessVariable 2",
				"NoPermVariable ns=1;s=NoPermVariable 2",
				"ReadOnlyVariable ns=1;s=ReadOnlyVariable 2",
				"ReadWriteVariable ns=1;s=ReadWriteVariable 2",
				"ro_bool ns=1;s=ro_bool 2",
				"ro_int32 ns=1;s=ro_int32 2",
				"rw_bool ns=1;s=rw_bool 2",
				"rw_int32 ns=1;s=rw_int32 2",
			},
		},
		{
			Name: "opcua-browse-variables",
			Script: `
				ua = require("opcua");
				try {
					client = new ua.Client({ endpoint: "opc.tcp://localhost:4840" });
					results = client.browse({
						nodes: ["ns=1;i=85"],
						nodeClassMask: ua.NodeClass.Variable,
					});
					results[0].references
						.sort((a,b) => a.browseName < b.browseName ? -1 : 1)
						.forEach(r => console.println(r.browseName, r.nodeId, r.nodeClass));
				} catch(e) {
					console.println("Error:", e);
				} finally {
					if (client !== undefined) client.close();
				}
			`,
			Output: []string{
				"NoAccessVariable ns=1;s=NoAccessVariable 2",
				"NoPermVariable ns=1;s=NoPermVariable 2",
				"ReadOnlyVariable ns=1;s=ReadOnlyVariable 2",
				"ReadWriteVariable ns=1;s=ReadWriteVariable 2",
				"ro_bool ns=1;s=ro_bool 2",
				"ro_int32 ns=1;s=ro_int32 2",
				"rw_bool ns=1;s=rw_bool 2",
				"rw_int32 ns=1;s=rw_int32 2",
			},
		},
		{
			Name: "opcua-browse-errors",
			Script: `
				ua = require("opcua");
				try {
					client = new ua.Client({ endpoint: "opc.tcp://localhost:4840" });
					failed = false;
					try {
						client.browse({});
					} catch (e) {
						failed = true;
					}
					console.println("browse missing nodes:", failed);
					failed = false;
					try {
						client.browse({ nodes: ["ns=x;i=1"] });
					} catch (e) {
						failed = true;
					}
					console.println("browse invalid node:", failed);
					results = client.browse({
						nodes: ["ns=1;i=85"],
						referenceTypeId: "ns=0;i=31",
						includeSubtypes: true,
						resultMask: ua.BrowseResultMask.All,
					});
					console.println("browse custom status:", results[0].status);
				} finally {
					if (client !== undefined) client.close();
				}
			`,
			Output: []string{
				"browse missing nodes: true",
				"browse invalid node: true",
				"browse custom status: 0",
			},
		},
		{
			Name: "opcua-browse-next-variables",
			Script: `
				ua = require("opcua");
				try {
					client = new ua.Client({ endpoint: "opc.tcp://localhost:4840" });
					results = client.browse({
						nodes: ["ns=1;i=85"],
						nodeClassMask: ua.NodeClass.Variable,
						requestedMaxReferencesPerNode: 2,
					});
					console.println("continuation type:", typeof results[0].continuationPoint);
					console.println("browseNext type:", typeof client.browseNext);
					results[0].references
						.sort((a,b) => a.browseName < b.browseName ? -1 : 1)
						.forEach(r => console.println(r.browseName, r.nodeId, r.nodeClass));
				} catch(e) {
					console.println("Error:", e);
				} finally {
					if (client !== undefined) client.close();
				}
			`,
			Output: []string{
				"continuation type: string",
				"browseNext type: function",
				"NoAccessVariable ns=1;s=NoAccessVariable 2",
				"NoPermVariable ns=1;s=NoPermVariable 2",
				"ReadOnlyVariable ns=1;s=ReadOnlyVariable 2",
				"ReadWriteVariable ns=1;s=ReadWriteVariable 2",
				"ro_bool ns=1;s=ro_bool 2",
				"ro_int32 ns=1;s=ro_int32 2",
				"rw_bool ns=1;s=rw_bool 2",
				"rw_int32 ns=1;s=rw_int32 2",
			},
		},
		{
			Name: "opcua-browse-next-errors",
			Script: `
				ua = require("opcua");
				try {
					client = new ua.Client({ endpoint: "opc.tcp://localhost:4840" });
					failed = false;
					try {
						client.browseNext({ continuationPoints: [] });
					} catch (e) {
						failed = true;
					}
					console.println("browseNext missing continuationPoints:", failed);
					failed = false;
					try {
						client.browseNext({ continuationPoints: ["not-base64"] });
					} catch (e) {
						failed = true;
					}
					console.println("browseNext invalid continuationPoint:", failed);
				} finally {
					if (client !== undefined) client.close();
				}
			`,
			Output: []string{
				"browseNext missing continuationPoints: true",
				"browseNext invalid continuationPoint: true",
			},
		},
		{
			Name: "opcua-attributes",
			Script: `
				ua = require("opcua");
				try {
					client = new ua.Client({ endpoint: "opc.tcp://localhost:4840" });
					results = client.attributes({ requests: [
						{ node: "ns=1;s=ro_bool",           attributeId: ua.AttributeID.DataType },
						{ node: "ns=1;s=ro_int32",          attributeId: ua.AttributeID.DataType },
						{ node: "ns=1;s=ReadWriteVariable", attributeId: ua.AttributeID.DataType },
						{ node: "ns=1;s=ReadWriteVariable", attributeId: ua.AttributeID.BrowseName },
						{ node: "ns=1;s=ReadWriteVariable", attributeId: ua.AttributeID.NodeClass },
					]});
					results.forEach(r => console.println(r.status, r.value));
				} catch(e) {
					console.println("Error:", e);
				} finally {
					if (client !== undefined) client.close();
				}
			`,
			Output: []string{
				"0 Boolean",
				"0 Int32",
				"0 Double",
				"0 ReadWriteVariable",
				"0 NodeClassVariable",
			},
		},
		{
			Name: "opcua-attributes-errors",
			Script: `
				ua = require("opcua");
				try {
					client = new ua.Client({ endpoint: "opc.tcp://localhost:4840" });
					failed = false;
					try {
						client.attributes({ requests: [] });
					} catch (e) {
						failed = true;
					}
					console.println("attributes empty:", failed);
					failed = false;
					try {
						client.attributes({ requests: [{ node: "ns=x;i=1", attributeId: ua.AttributeID.DataType }] });
					} catch (e) {
						failed = true;
					}
					console.println("attributes invalid node:", failed);
				} finally {
					if (client !== undefined) client.close();
				}
			`,
			Output: []string{
				"attributes empty: true",
				"attributes invalid node: true",
			},
		},
		{
			Name: "opcua-children-errors",
			Script: `
				ua = require("opcua");
				try {
					client = new ua.Client({ endpoint: "opc.tcp://localhost:4840" });
					failed = false;
					try {
						client.children({});
					} catch (e) {
						failed = true;
					}
					console.println("children missing node:", failed);
					failed = false;
					try {
						client.children({ node: "ns=x;i=1" });
					} catch (e) {
						failed = true;
					}
					console.println("children invalid node:", failed);
				} finally {
					if (client !== undefined) client.close();
				}
			`,
			Output: []string{
				"children missing node: true",
				"children invalid node: true",
			},
		},
	}
	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func startOPCUAServer() *opc_server.Server {
	var opts []opc_server.Option
	port := 4840

	opts = append(opts,
		opc_server.EnableSecurity("None", ua.MessageSecurityModeNone),
		opc_server.EnableSecurity("Basic128Rsa15", ua.MessageSecurityModeSign),
		opc_server.EnableSecurity("Basic128Rsa15", ua.MessageSecurityModeSignAndEncrypt),
		opc_server.EnableSecurity("Basic256", ua.MessageSecurityModeSign),
		opc_server.EnableSecurity("Basic256", ua.MessageSecurityModeSignAndEncrypt),
		opc_server.EnableSecurity("Basic256Sha256", ua.MessageSecurityModeSignAndEncrypt),
		opc_server.EnableSecurity("Basic256Sha256", ua.MessageSecurityModeSign),
		opc_server.EnableSecurity("Aes128_Sha256_RsaOaep", ua.MessageSecurityModeSign),
		opc_server.EnableSecurity("Aes128_Sha256_RsaOaep", ua.MessageSecurityModeSignAndEncrypt),
		opc_server.EnableSecurity("Aes256_Sha256_RsaPss", ua.MessageSecurityModeSign),
		opc_server.EnableSecurity("Aes256_Sha256_RsaPss", ua.MessageSecurityModeSignAndEncrypt),
	)

	opts = append(opts,
		opc_server.EnableAuthMode(ua.UserTokenTypeAnonymous),
		opc_server.EnableAuthMode(ua.UserTokenTypeUserName),
		opc_server.EnableAuthMode(ua.UserTokenTypeCertificate),
		//		server.EnableAuthWithoutEncryption(), // Dangerous and not recommended, shown for illustration only
	)

	opts = append(opts,
		opc_server.EndPoint("localhost", port),
	)

	s := opc_server.New(opts...)

	root_ns, _ := s.Namespace(0)
	obj_node := root_ns.Objects()

	// Create a new node namespace.  You can add namespaces before or after starting the server.
	nodeNS := opc_server.NewNodeNameSpace(s, "NodeNamespace")
	// add it to the server.
	s.AddNamespace(nodeNS)
	nns_obj := nodeNS.Objects()
	// add the reference for this namespace's root object folder to the server's root object folder
	obj_node.AddRef(nns_obj, id.HasComponent, true)

	// Create some nodes for it.
	n := nodeNS.AddNewVariableStringNode("ro_bool", true)
	n.SetAttribute(ua.AttributeIDUserAccessLevel, &ua.DataValue{EncodingMask: ua.DataValueValue, Value: ua.MustVariant(byte(1))})
	n.SetAttribute(ua.AttributeIDDataType, opc_server.DataValueFromValue(ua.NewNumericExpandedNodeID(0, 1)))
	nns_obj.AddRef(n, id.HasComponent, true)
	n = nodeNS.AddNewVariableStringNode("rw_bool", true)
	n.SetAttribute(ua.AttributeIDDataType, opc_server.DataValueFromValue(ua.NewNumericExpandedNodeID(0, 1)))
	nns_obj.AddRef(n, id.HasComponent, true)

	n = nodeNS.AddNewVariableStringNode("ro_int32", int32(5))
	n.SetAttribute(ua.AttributeIDUserAccessLevel, &ua.DataValue{EncodingMask: ua.DataValueValue, Value: ua.MustVariant(byte(1))})
	n.SetAttribute(ua.AttributeIDDataType, opc_server.DataValueFromValue(ua.NewNumericExpandedNodeID(0, 6)))
	nns_obj.AddRef(n, id.HasComponent, true)
	n = nodeNS.AddNewVariableStringNode("rw_int32", int32(5))
	n.SetAttribute(ua.AttributeIDDataType, opc_server.DataValueFromValue(ua.NewNumericExpandedNodeID(0, 6)))
	nns_obj.AddRef(n, id.HasComponent, true)

	var3 := opc_server.NewNode(
		ua.NewStringNodeID(nodeNS.ID(), "NoPermVariable"), // you can use whatever node id you want here, whether it's numeric, string, guid, etc...
		map[ua.AttributeID]*ua.DataValue{
			ua.AttributeIDBrowseName: opc_server.DataValueFromValue(attrs.BrowseName("NoPermVariable")),
			ua.AttributeIDNodeClass:  opc_server.DataValueFromValue(uint32(ua.NodeClassVariable)),
			ua.AttributeIDDataType:   opc_server.DataValueFromValue(ua.NewNumericExpandedNodeID(0, 6)),
		},
		nil,
		func() *ua.DataValue { return opc_server.DataValueFromValue(int32(742)) },
	)
	nodeNS.AddNode(var3)
	nns_obj.AddRef(var3, id.HasComponent, true)

	var4 := opc_server.NewNode(
		ua.NewStringNodeID(nodeNS.ID(), "ReadWriteVariable"), // you can use whatever node id you want here, whether it's numeric, string, guid, etc...
		map[ua.AttributeID]*ua.DataValue{
			ua.AttributeIDAccessLevel:     opc_server.DataValueFromValue(byte(ua.AccessLevelTypeCurrentRead | ua.AccessLevelTypeCurrentWrite)),
			ua.AttributeIDUserAccessLevel: opc_server.DataValueFromValue(byte(ua.AccessLevelTypeCurrentRead | ua.AccessLevelTypeCurrentWrite)),
			ua.AttributeIDBrowseName:      opc_server.DataValueFromValue(attrs.BrowseName("ReadWriteVariable")),
			ua.AttributeIDNodeClass:       opc_server.DataValueFromValue(uint32(ua.NodeClassVariable)),
			ua.AttributeIDDataType:        opc_server.DataValueFromValue(ua.NewNumericExpandedNodeID(0, 11)),
		},
		nil,
		func() *ua.DataValue { return opc_server.DataValueFromValue(12.34) },
	)
	nodeNS.AddNode(var4)
	nns_obj.AddRef(var4, id.HasComponent, true)

	var5 := opc_server.NewNode(
		ua.NewStringNodeID(nodeNS.ID(), "ReadOnlyVariable"), // you can use whatever node id you want here, whether it's numeric, string, guid, etc...
		map[ua.AttributeID]*ua.DataValue{
			ua.AttributeIDAccessLevel:     opc_server.DataValueFromValue(byte(ua.AccessLevelTypeCurrentRead)),
			ua.AttributeIDUserAccessLevel: opc_server.DataValueFromValue(byte(ua.AccessLevelTypeCurrentRead)),
			ua.AttributeIDBrowseName:      opc_server.DataValueFromValue(attrs.BrowseName("ReadOnlyVariable")),
			ua.AttributeIDNodeClass:       opc_server.DataValueFromValue(uint32(ua.NodeClassVariable)),
			ua.AttributeIDDataType:        opc_server.DataValueFromValue(ua.NewNumericExpandedNodeID(0, 11)),
		},
		nil,
		func() *ua.DataValue { return opc_server.DataValueFromValue(9.87) },
	)
	nodeNS.AddNode(var5)
	nns_obj.AddRef(var5, id.HasComponent, true)

	var6 := opc_server.NewNode(
		ua.NewStringNodeID(nodeNS.ID(), "NoAccessVariable"), // you can use whatever node id you want here, whether it's numeric, string, guid, etc...
		map[ua.AttributeID]*ua.DataValue{
			ua.AttributeIDAccessLevel:     opc_server.DataValueFromValue(byte(ua.AccessLevelTypeNone)),
			ua.AttributeIDUserAccessLevel: opc_server.DataValueFromValue(byte(ua.AccessLevelTypeNone)),
			ua.AttributeIDBrowseName:      opc_server.DataValueFromValue(attrs.BrowseName("NoAccessVariable")),
			ua.AttributeIDNodeClass:       opc_server.DataValueFromValue(uint32(ua.NodeClassVariable)),
			ua.AttributeIDDataType:        opc_server.DataValueFromValue(ua.NewNumericExpandedNodeID(0, 11)),
		},
		nil,
		func() *ua.DataValue { return opc_server.DataValueFromValue(55.43) },
	)
	nodeNS.AddNode(var6)
	nns_obj.AddRef(var6, id.HasComponent, true)

	// Create a new node namespace.  You can add namespaces before or after starting the server.
	gopcuaNS := opc_server.NewNodeNameSpace(s, "http://gopcua.com/")
	// add it to the server.
	s.AddNamespace(gopcuaNS)
	nns_obj = gopcuaNS.Objects()
	// add the reference for this namespace's root object folder to the server's root object folder
	obj_node.AddRef(nns_obj, id.HasComponent, true)

	// Create a new node namespace.  You can add namespaces before or after starting the server.
	// Start the server
	if err := s.Start(context.Background()); err != nil {
		log.Fatalf("Error starting server, exiting: %s", err)
	}
	return s
}
