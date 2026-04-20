package opcua

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/gopcua/opcua/ua"
)

func TestParseAttributeValue(t *testing.T) {
	tests := []struct {
		name   string
		attrID ua.AttributeID
		data   *ua.DataValue
		want   any
	}{
		{
			name:   "DataType NodeID",
			attrID: ua.AttributeIDDataType,
			data:   &ua.DataValue{Value: ua.MustVariant(ua.NewNumericNodeID(0, 6))},
			want:   "Int32",
		},
		{
			name:   "DataType ExpandedNodeID",
			attrID: ua.AttributeIDDataType,
			data:   &ua.DataValue{Value: ua.MustVariant(ua.NewNumericExpandedNodeID(0, 1))},
			want:   "Boolean",
		},
		{
			name:   "DisplayName",
			attrID: ua.AttributeIDDisplayName,
			data:   &ua.DataValue{Value: ua.MustVariant(&ua.LocalizedText{Text: "Temperature"})},
			want:   "Temperature",
		},
		{
			name:   "Description",
			attrID: ua.AttributeIDDescription,
			data:   &ua.DataValue{Value: ua.MustVariant(&ua.LocalizedText{Text: "A sensor value"})},
			want:   "A sensor value",
		},
		{
			name:   "BrowseName",
			attrID: ua.AttributeIDBrowseName,
			data:   &ua.DataValue{Value: ua.MustVariant(&ua.QualifiedName{Name: "MyVar"})},
			want:   "MyVar",
		},
		{
			name:   "NodeClass",
			attrID: ua.AttributeIDNodeClass,
			data:   &ua.DataValue{Value: ua.MustVariant(int32(ua.NodeClassVariable))},
			want:   ua.NodeClassVariable.String(),
		},
		{
			name:   "AccessLevel",
			attrID: ua.AttributeIDAccessLevel,
			data:   &ua.DataValue{Value: ua.MustVariant(byte(ua.AccessLevelTypeCurrentRead))},
			want:   ua.AccessLevelType(ua.AccessLevelTypeCurrentRead).String(),
		},
		{
			name:   "UserAccessLevel",
			attrID: ua.AttributeIDUserAccessLevel,
			data:   &ua.DataValue{Value: ua.MustVariant(byte(ua.AccessLevelTypeCurrentRead | ua.AccessLevelTypeCurrentWrite))},
			want:   ua.AccessLevelType(ua.AccessLevelTypeCurrentRead | ua.AccessLevelTypeCurrentWrite).String(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAttributeValue(tt.attrID, tt.data)
			if got != tt.want {
				t.Fatalf("parseAttributeValue(%v) = %v, want %v", tt.attrID, got, tt.want)
			}
		})
	}
}

func TestToBrowseResultsIncludesContinuationPoint(t *testing.T) {
	point := []byte{1, 2, 3, 4}
	results := toBrowseResults([]*ua.BrowseResult{
		{
			StatusCode:        ua.StatusBadNoContinuationPoints,
			ContinuationPoint: point,
			References: []*ua.ReferenceDescription{
				{
					ReferenceTypeID: ua.NewNumericNodeID(0, 35),
					IsForward:       true,
					NodeID:          ua.NewExpandedNodeID(ua.NewStringNodeID(1, "child"), "", 0),
					BrowseName:      &ua.QualifiedName{Name: "child"},
					DisplayName:     &ua.LocalizedText{Text: "Child"},
					NodeClass:       ua.NodeClassVariable,
					TypeDefinition:  ua.NewExpandedNodeID(ua.NewNumericNodeID(0, 63), "", 0),
				},
			},
		},
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if got, want := results[0].ContinuationPoint, base64.StdEncoding.EncodeToString(point); got != want {
		t.Fatalf("unexpected continuation point: got %q want %q", got, want)
	}
	if got := len(results[0].References); got != 1 {
		t.Fatalf("expected 1 reference, got %d", got)
	}
	if got := results[0].References[0].NodeId; got != "ns=1;s=child" {
		t.Fatalf("unexpected node id: %q", got)
	}
}

func TestDecodeContinuationPoint(t *testing.T) {
	if got := encodeContinuationPoint(nil); got != "" {
		t.Fatalf("expected empty continuation point encoding, got %q", got)
	}

	decoded, err := decodeContinuationPoint(base64.StdEncoding.EncodeToString([]byte("next")))
	if err != nil {
		t.Fatalf("decodeContinuationPoint returned error: %v", err)
	}
	if got, want := string(decoded), "next"; got != want {
		t.Fatalf("unexpected decoded continuation point: got %q want %q", got, want)
	}

	if _, err := decodeContinuationPoint(""); err == nil {
		t.Fatal("expected empty continuation point to return an error")
	}

	if _, err := decodeContinuationPoint("not-base64"); err == nil {
		t.Fatal("expected invalid base64 continuation point to return an error")
	}
}

func TestCloseNilClient(t *testing.T) {
	client := &Client{}
	if err := client.Close(); err != nil {
		t.Fatalf("Close() returned error for nil client: %v", err)
	}
}

func TestNewClientInvalidEndpoint(t *testing.T) {
	client, err := NewClient(ClientOptions{
		Endpoint:          "opc.tcp://127.0.0.1:1",
		ReadRetryInterval: 10 * time.Millisecond,
	})
	if err == nil {
		if client != nil {
			_ = client.Close()
		}
		t.Fatal("expected NewClient to fail for an unreachable endpoint")
	}
}
